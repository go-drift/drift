/// AudioPlayers.swift
///
/// Audio-only AVPlayer playback for the `drift/media/audio` channel:
/// several players at once, each identified by the playerId Go assigns
/// (runtime/audio.go). Main thread only.

import AVFoundation
import DriftPluginAPI

// MARK: - Audio Player Instance

/// Per-instance audio player state.
private final class AudioPlayerInstance {
    let id: Int
    let player: AVQueuePlayer
    /// Sends an event to Go on channel.
    private let send: (_ channel: String, _ data: [String: Any]) -> Void
    private lazy var ticker = ProgressTicker(player) { [weak self] in self?.sendEvent($0) }
    /// The playback state last reported (runtime PlaybackState).
    private var state = 0 // Idle
    private var timeControlObservation: NSKeyValueObservation?
    private var itemStatusObservation: NSKeyValueObservation?
    private var endOfItemObserver: NSObjectProtocol?
    private var playerLooper: AVPlayerLooper?
    private var playbackSpeed: Float = 1.0
    private var isLooping: Bool = false
    private var hasReachedEnd: Bool = false
    private var isStopped: Bool = false

    init(id: Int, send: @escaping (_ channel: String, _ data: [String: Any]) -> Void) {
        self.id = id
        self.send = send

        MediaSession.activate()

        self.player = AVQueuePlayer()

        // Observe time control status for playback state
        timeControlObservation = player.observe(\.timeControlStatus) { [weak self] player, _ in
            guard let self = self else { return }
            let state: Int
            switch player.timeControlStatus {
            case .paused:
                if player.currentItem == nil || self.isStopped {
                    state = 0 // Idle
                } else if self.hasReachedEnd {
                    state = 3 // Completed
                } else {
                    state = 4 // Paused
                }
            case .waitingToPlayAtSpecifiedRate:
                state = 1 // Buffering
            case .playing:
                state = 2 // Playing
            @unknown default:
                state = 0 // Idle
            }
            self.reportState(state)
        }
    }

    /// Reports state to Go with the progress, at position instead of the
    /// current time when given.
    private func reportState(_ state: Int, at position: CMTime? = nil) {
        self.state = state
        ticker.reportNow(at: position)
    }

    /// Sends progress to Go: every event carries the state with it.
    private func sendEvent(_ progress: PlaybackProgress) {
        var event = progress.fields
        event["playerId"] = id
        event["playbackState"] = state
        send(AudioPlayers.eventsChannel, event)
    }

    func load(url: URL) {
        isStopped = false
        hasReachedEnd = false

        // Disable existing looper before replacing the item
        playerLooper?.disableLooping()
        playerLooper = nil

        // Remove previous end-of-item observer
        if let observer = endOfItemObserver {
            NotificationCenter.default.removeObserver(observer)
            endOfItemObserver = nil
        }

        let item = AVPlayerItem(url: url)
        player.replaceCurrentItem(with: item)
        ticker.start()

        // Observe item status for errors
        itemStatusObservation?.invalidate()
        itemStatusObservation = item.observe(\.status) { [weak self] item, _ in
            guard let self = self else { return }
            if item.status == .failed {
                self.send(AudioPlayers.errorsChannel, [
                    "playerId": self.id,
                    "code": mediaErrorCode(for: item.error),
                    "message": item.error?.localizedDescription ?? "Unknown playback error"
                ])
            }
        }

        // Register end-of-item observer for completion detection.
        // When AVPlayerLooper is active it prevents this notification from firing,
        // so the observer and looper are naturally mutually exclusive.
        endOfItemObserver = NotificationCenter.default.addObserver(
            forName: .AVPlayerItemDidPlayToEndTime,
            object: item,
            queue: .main
        ) { [weak self] _ in
            guard let self = self else { return }
            self.hasReachedEnd = true
            self.reportState(3) // Completed
        }

        // Re-create looper if looping is active
        if isLooping {
            playerLooper = AVPlayerLooper(player: player, templateItem: item)
        }
    }

    func play() {
        isStopped = false
        hasReachedEnd = false
        player.play()
        if playbackSpeed != 1.0 {
            player.rate = playbackSpeed
        }
    }

    func pause() {
        player.pause()
    }

    func stop() {
        isStopped = true
        hasReachedEnd = false
        player.pause()
        player.seek(to: .zero)
        // Idle even when already paused, which changes no status.
        reportState(0, at: .zero)
    }

    func seekTo(positionMs: Int64) {
        hasReachedEnd = false
        let time = CMTime(seconds: Double(positionMs) / 1000.0, preferredTimescale: CMTimeScale(NSEC_PER_SEC))
        player.seek(to: time)
        ticker.reportNow(at: time)
    }

    func setVolume(_ volume: Float) {
        player.volume = volume
    }

    func setLooping(_ looping: Bool) {
        isLooping = looping

        // Disable existing looper
        playerLooper?.disableLooping()
        playerLooper = nil

        if looping, let item = player.currentItem {
            playerLooper = AVPlayerLooper(player: player, templateItem: item)
        }
    }

    func setPlaybackSpeed(_ rate: Float) {
        playbackSpeed = rate
        if player.timeControlStatus == .playing {
            player.rate = rate
        }
    }

    func dispose() {
        ticker.stop()
        timeControlObservation?.invalidate()
        timeControlObservation = nil
        itemStatusObservation?.invalidate()
        itemStatusObservation = nil
        if let observer = endOfItemObserver {
            NotificationCenter.default.removeObserver(observer)
            endOfItemObserver = nil
        }
        playerLooper?.disableLooping()
        playerLooper = nil
        player.pause()
        player.replaceCurrentItem(with: nil)
        playbackSpeed = 1.0
        isLooping = false
        hasReachedEnd = false
        isStopped = false
    }
}

// MARK: - Players

/// The `drift/media/audio` channel's players, keyed by playerId.
final class AudioPlayers {
    static let channel = "drift/media/audio"
    static let eventsChannel = "drift/media/audio/events"
    static let errorsChannel = "drift/media/audio/errors"

    private var players: [Int: AudioPlayerInstance] = [:]
    private let send: (_ channel: String, _ data: [String: Any]) -> Void

    init(send: @escaping (_ channel: String, _ data: [String: Any]) -> Void) {
        self.send = send
    }

    /// Declares the channel's methods.
    func declare(_ channel: DriftChannel) {
        channel.method("load") { [self] args in
            let args = args as? [String: Any] ?? [:]
            guard let url = (args["url"] as? String).flatMap(URL.init(string:)) else {
                throw DriftCallError.invalidArguments("load needs a url")
            }
            player(args).load(url: url)
            return nil
        }
        channel.method("play") { [self] args in player(args).play(); return nil }
        channel.method("pause") { [self] args in player(args).pause(); return nil }
        channel.method("stop") { [self] args in player(args).stop(); return nil }
        channel.method("seekTo") { [self] args in
            let positionMs = ((args as? [String: Any])?["positionMs"] as? NSNumber)?.int64Value ?? 0
            player(args).seekTo(positionMs: positionMs)
            return nil
        }
        channel.method("setVolume") { [self] args in
            player(args).setVolume(((args as? [String: Any])?["volume"] as? NSNumber)?.floatValue ?? 1.0)
            return nil
        }
        channel.method("setLooping") { [self] args in
            player(args).setLooping((args as? [String: Any])?["looping"] as? Bool ?? false)
            return nil
        }
        channel.method("setPlaybackSpeed") { [self] args in
            player(args).setPlaybackSpeed(((args as? [String: Any])?["rate"] as? NSNumber)?.floatValue ?? 1.0)
            return nil
        }
        channel.method("dispose") { [self] args in
            let id = playerId(args)
            if let instance = players.removeValue(forKey: id) {
                instance.dispose()
                MediaSession.deactivate()
            }
            return nil
        }
    }

    private func playerId(_ args: Any?) -> Int {
        ((args as? [String: Any])?["playerId"] as? NSNumber)?.intValue ?? 0
    }

    /// The player args' playerId names, created on first use by any method
    /// but dispose, so settings made before load apply to what it loads.
    private func player(_ args: Any?) -> AudioPlayerInstance {
        let id = playerId(args)
        if let existing = players[id] {
            return existing
        }
        let instance = AudioPlayerInstance(id: id, send: send)
        players[id] = instance
        return instance
    }
}
