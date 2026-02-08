/// NativeAudioPlayer.swift
/// Provides audio-only playback using AVPlayer via a standalone platform channel.
/// Supports multiple concurrent player instances, each identified by a playerId.

import AVFoundation

// MARK: - Audio Player Instance

/// Per-instance audio player state.
private class AudioPlayerInstance {
    let id: Int
    let player: AVQueuePlayer
    private var timeObserver: Any?
    private var timeControlObservation: NSKeyValueObservation?
    private var itemStatusObservation: NSKeyValueObservation?
    private var endOfItemObserver: NSObjectProtocol?
    private var playerLooper: AVPlayerLooper?
    private var playbackSpeed: Float = 1.0
    private var isLooping: Bool = false
    private var hasReachedEnd: Bool = false
    private var isStopped: Bool = false

    init(id: Int) {
        self.id = id

        DriftMediaSession.activate()

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
                self.stopPositionUpdates()
            case .waitingToPlayAtSpecifiedRate:
                state = 1 // Buffering
                self.stopPositionUpdates()
            case .playing:
                state = 2 // Playing
                self.startPositionUpdates()
            @unknown default:
                state = 0 // Idle
                self.stopPositionUpdates()
            }
            self.sendStateEvent(state: state)
        }
    }

    private func startPositionUpdates() {
        guard timeObserver == nil else { return }
        let interval = CMTime(seconds: 0.25, preferredTimescale: CMTimeScale(NSEC_PER_SEC))
        timeObserver = player.addPeriodicTimeObserver(forInterval: interval, queue: .main) { [weak self] time in
            guard let self = self else { return }
            let positionMs = time.isNumeric ? Int64(CMTimeGetSeconds(time) * 1000) : 0
            var durationMs: Int64 = 0
            var bufferedMs: Int64 = 0

            if let item = self.player.currentItem {
                if item.duration.isNumeric {
                    durationMs = Int64(CMTimeGetSeconds(item.duration) * 1000)
                }
                if let timeRange = item.loadedTimeRanges.last?.timeRangeValue {
                    let bufferedEnd = CMTimeAdd(timeRange.start, timeRange.duration)
                    bufferedMs = Int64(CMTimeGetSeconds(bufferedEnd) * 1000)
                }
            }

            PlatformChannelManager.shared.sendEvent(
                channel: "drift/audio_player/events",
                data: [
                    "playerId": self.id,
                    "playbackState": 2, // Playing
                    "positionMs": positionMs,
                    "durationMs": max(durationMs, 0),
                    "bufferedMs": bufferedMs
                ]
            )
        }
    }

    private func stopPositionUpdates() {
        if let observer = timeObserver {
            player.removeTimeObserver(observer)
            timeObserver = nil
        }
    }

    func sendStateEvent(state: Int) {
        let currentTime = player.currentTime()
        let positionMs = currentTime.isNumeric ? Int64(CMTimeGetSeconds(currentTime) * 1000) : 0
        var durationMs: Int64 = 0
        var bufferedMs: Int64 = 0

        if let item = player.currentItem {
            if item.duration.isNumeric {
                durationMs = Int64(CMTimeGetSeconds(item.duration) * 1000)
            }
            if let timeRange = item.loadedTimeRanges.last?.timeRangeValue {
                let bufferedEnd = CMTimeAdd(timeRange.start, timeRange.duration)
                bufferedMs = Int64(CMTimeGetSeconds(bufferedEnd) * 1000)
            }
        }

        PlatformChannelManager.shared.sendEvent(
            channel: "drift/audio_player/events",
            data: [
                "playerId": id,
                "playbackState": state,
                "positionMs": positionMs,
                "durationMs": max(durationMs, 0),
                "bufferedMs": bufferedMs
            ]
        )
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

        // Observe item status for errors
        itemStatusObservation?.invalidate()
        itemStatusObservation = item.observe(\.status) { [weak self] item, _ in
            guard let self = self else { return }
            if item.status == .failed {
                PlatformChannelManager.shared.sendEvent(
                    channel: "drift/audio_player/errors",
                    data: [
                        "playerId": self.id,
                        "code": mediaErrorCode(for: item.error),
                        "message": item.error?.localizedDescription ?? "Unknown playback error"
                    ]
                )
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
            self.sendStateEvent(state: 3) // Completed
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
        player.seek(to: .zero) { [weak self] _ in
            guard let self = self else { return }
            self.sendStateEvent(state: 0) // Idle with position reset to zero
        }
    }

    func seekTo(positionMs: Int64) {
        hasReachedEnd = false
        let time = CMTime(seconds: Double(positionMs) / 1000.0, preferredTimescale: CMTimeScale(NSEC_PER_SEC))
        player.seek(to: time)
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
        stopPositionUpdates()
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

// MARK: - Audio Player Handler

/// Handles audio player platform channel methods from Go.
/// Manages multiple player instances keyed by playerId.
enum AudioPlayerHandler {
    private static var players: [Int: AudioPlayerInstance] = [:]

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        let argsMap = args as? [String: Any]
        let playerId = (argsMap?["playerId"] as? NSNumber)?.intValue ?? 0

        switch method {
        case "load":
            return load(playerId: playerId, args: argsMap)
        case "play":
            return play(playerId: playerId)
        case "pause":
            return pause(playerId: playerId)
        case "stop":
            return stop(playerId: playerId)
        case "seekTo":
            return seekTo(playerId: playerId, args: argsMap)
        case "setVolume":
            return setVolume(playerId: playerId, args: argsMap)
        case "setLooping":
            return setLooping(playerId: playerId, args: argsMap)
        case "setPlaybackSpeed":
            return setPlaybackSpeed(playerId: playerId, args: argsMap)
        case "dispose":
            return dispose(playerId: playerId)
        default:
            return (nil, NSError(domain: "AudioPlayer", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    private static func ensurePlayer(playerId: Int) -> AudioPlayerInstance {
        if let existing = players[playerId] {
            return existing
        }
        let instance = AudioPlayerInstance(id: playerId)
        players[playerId] = instance
        return instance
    }

    private static func load(playerId: Int, args: [String: Any]?) -> (Any?, Error?) {
        guard let url = args?["url"] as? String,
              let mediaURL = URL(string: url) else {
            return (nil, NSError(domain: "AudioPlayer", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing url"]))
        }

        ensurePlayer(playerId: playerId).load(url: mediaURL)
        return (nil, nil)
    }

    private static func play(playerId: Int) -> (Any?, Error?) {
        ensurePlayer(playerId: playerId).play()
        return (nil, nil)
    }

    private static func pause(playerId: Int) -> (Any?, Error?) {
        players[playerId]?.pause()
        return (nil, nil)
    }

    private static func stop(playerId: Int) -> (Any?, Error?) {
        players[playerId]?.stop()
        return (nil, nil)
    }

    private static func seekTo(playerId: Int, args: [String: Any]?) -> (Any?, Error?) {
        let positionMs = args?["positionMs"] as? Int64 ?? 0
        players[playerId]?.seekTo(positionMs: positionMs)
        return (nil, nil)
    }

    private static func setVolume(playerId: Int, args: [String: Any]?) -> (Any?, Error?) {
        let volume = (args?["volume"] as? NSNumber)?.floatValue ?? 1.0
        players[playerId]?.setVolume(volume)
        return (nil, nil)
    }

    private static func setLooping(playerId: Int, args: [String: Any]?) -> (Any?, Error?) {
        let looping = args?["looping"] as? Bool ?? false
        players[playerId]?.setLooping(looping)
        return (nil, nil)
    }

    private static func setPlaybackSpeed(playerId: Int, args: [String: Any]?) -> (Any?, Error?) {
        let rate = (args?["rate"] as? NSNumber)?.floatValue ?? 1.0
        players[playerId]?.setPlaybackSpeed(rate)
        return (nil, nil)
    }

    private static func dispose(playerId: Int) -> (Any?, Error?) {
        players.removeValue(forKey: playerId)?.dispose()
        DriftMediaSession.deactivate()
        return (nil, nil)
    }
}
