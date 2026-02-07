/// NativeAudioPlayer.swift
/// Provides audio-only playback using AVPlayer via a standalone platform channel.
/// Supports multiple concurrent player instances, each identified by a playerId.

import AVFoundation

// MARK: - Audio Player Instance

/// Per-instance audio player state.
private class AudioPlayerInstance {
    let id: Int
    let player: AVPlayer
    private var timeObserver: Any?
    private var timeControlObservation: NSKeyValueObservation?
    private var itemStatusObservation: NSKeyValueObservation?
    private var loopObserver: NSObjectProtocol?
    private var playbackSpeed: Float = 1.0
    private var isLooping: Bool = false

    init(id: Int) {
        self.id = id

        do {
            try AVAudioSession.sharedInstance().setCategory(.playback)
            try AVAudioSession.sharedInstance().setActive(true)
        } catch {
            print("[drift] AVAudioSession setup failed: \(error)")
        }

        self.player = AVPlayer()

        // Observe time control status for playback state
        timeControlObservation = player.observe(\.timeControlStatus) { [weak self] player, _ in
            guard let self = self else { return }
            let state: Int
            switch player.timeControlStatus {
            case .paused:
                if let item = player.currentItem,
                   item.duration.isNumeric,
                   CMTimeCompare(player.currentTime(), item.duration) >= 0 {
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
            self.sendStateEvent(state: state)
        }

        // Add periodic time observer for position updates
        let interval = CMTime(seconds: 0.25, preferredTimescale: CMTimeScale(NSEC_PER_SEC))
        timeObserver = player.addPeriodicTimeObserver(forInterval: interval, queue: .main) { [weak self] time in
            guard let self = self else { return }
            let positionMs = Int64(CMTimeGetSeconds(time) * 1000)
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

            let playbackState: Int
            switch self.player.timeControlStatus {
            case .paused:
                playbackState = 4
            case .waitingToPlayAtSpecifiedRate:
                playbackState = 1
            case .playing:
                playbackState = 2
            @unknown default:
                playbackState = 0
            }

            PlatformChannelManager.shared.sendEvent(
                channel: "drift/audio_player/events",
                data: [
                    "playerId": self.id,
                    "playbackState": playbackState,
                    "positionMs": positionMs,
                    "durationMs": max(durationMs, 0),
                    "bufferedMs": bufferedMs
                ]
            )
        }
    }

    func sendStateEvent(state: Int) {
        let positionMs = Int64(CMTimeGetSeconds(player.currentTime()) * 1000)
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
                        "code": "playback_failed",
                        "message": item.error?.localizedDescription ?? "Unknown playback error"
                    ]
                )
            }
        }

        // Re-attach loop observer to the new item if looping is active
        if isLooping {
            if let observer = loopObserver {
                NotificationCenter.default.removeObserver(observer)
            }
            loopObserver = NotificationCenter.default.addObserver(
                forName: .AVPlayerItemDidPlayToEndTime,
                object: item,
                queue: .main
            ) { [weak self] _ in
                self?.player.seek(to: .zero)
                self?.player.play()
            }
        }
    }

    func play() {
        player.play()
        if playbackSpeed != 1.0 {
            player.rate = playbackSpeed
        }
    }

    func pause() {
        player.pause()
    }

    func stop() {
        player.pause()
        player.seek(to: .zero)
    }

    func seekTo(positionMs: Int64) {
        let time = CMTime(seconds: Double(positionMs) / 1000.0, preferredTimescale: CMTimeScale(NSEC_PER_SEC))
        player.seek(to: time)
    }

    func setVolume(_ volume: Float) {
        player.volume = volume
    }

    func setLooping(_ looping: Bool) {
        isLooping = looping

        // Remove existing loop observer
        if let observer = loopObserver {
            NotificationCenter.default.removeObserver(observer)
            loopObserver = nil
        }

        if looping, let item = player.currentItem {
            loopObserver = NotificationCenter.default.addObserver(
                forName: .AVPlayerItemDidPlayToEndTime,
                object: item,
                queue: .main
            ) { [weak self] _ in
                self?.player.seek(to: .zero)
                self?.player.play()
            }
        }
    }

    func setPlaybackSpeed(_ rate: Float) {
        playbackSpeed = rate
        if player.timeControlStatus == .playing {
            player.rate = rate
        }
    }

    func dispose() {
        if let observer = timeObserver {
            player.removeTimeObserver(observer)
            timeObserver = nil
        }
        timeControlObservation?.invalidate()
        timeControlObservation = nil
        itemStatusObservation?.invalidate()
        itemStatusObservation = nil
        if let observer = loopObserver {
            NotificationCenter.default.removeObserver(observer)
            loopObserver = nil
        }
        player.pause()
        player.replaceCurrentItem(with: nil)
        playbackSpeed = 1.0
        isLooping = false
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

        // Deactivate audio session if no more players
        if players.isEmpty {
            try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
        }

        return (nil, nil)
    }
}
