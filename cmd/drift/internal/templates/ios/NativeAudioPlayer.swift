/// NativeAudioPlayer.swift
/// Provides audio-only playback using AVPlayer via a standalone platform channel.

import AVFoundation

// MARK: - Audio Player Handler

/// Handles audio player platform channel methods from Go.
enum AudioPlayerHandler {
    private static var player: AVPlayer?
    private static var timeObserver: Any?
    private static var statusObservation: NSKeyValueObservation?
    private static var timeControlObservation: NSKeyValueObservation?
    private static var itemStatusObservation: NSKeyValueObservation?
    private static var loopObserver: NSObjectProtocol?
    private static var playbackSpeed: Float = 1.0
    private static var isLooping: Bool = false

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        let argsMap = args as? [String: Any]

        switch method {
        case "load":
            return load(args: argsMap)
        case "play":
            return play()
        case "pause":
            return pause()
        case "stop":
            return stop()
        case "seekTo":
            return seekTo(args: argsMap)
        case "setVolume":
            return setVolume(args: argsMap)
        case "setLooping":
            return setLooping(args: argsMap)
        case "setPlaybackSpeed":
            return setPlaybackSpeed(args: argsMap)
        case "dispose":
            return dispose()
        default:
            return (nil, NSError(domain: "AudioPlayer", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    private static func ensurePlayer() -> AVPlayer {
        if let existing = player {
            return existing
        }

        do {
            try AVAudioSession.sharedInstance().setCategory(.playback)
            try AVAudioSession.sharedInstance().setActive(true)
        } catch {
            print("[drift] AVAudioSession setup failed: \(error)")
        }

        let newPlayer = AVPlayer()
        player = newPlayer

        // Observe time control status for playback state
        timeControlObservation = newPlayer.observe(\.timeControlStatus) { player, _ in
            let state: Int
            switch player.timeControlStatus {
            case .paused:
                if let item = player.currentItem,
                   item.duration.isNumeric,
                   CMTimeCompare(player.currentTime(), item.duration) >= 0 {
                    state = 4 // Completed
                } else {
                    state = 6 // Paused
                }
            case .waitingToPlayAtSpecifiedRate:
                state = 2 // Buffering
            case .playing:
                state = 3 // Playing
            @unknown default:
                state = 0 // Idle
            }
            sendStateEvent(state: state)
        }

        // Add periodic time observer for position updates
        let interval = CMTime(seconds: 0.25, preferredTimescale: CMTimeScale(NSEC_PER_SEC))
        timeObserver = newPlayer.addPeriodicTimeObserver(forInterval: interval, queue: .main) { time in
            guard let p = player else { return }
            let positionMs = Int64(CMTimeGetSeconds(time) * 1000)
            var durationMs: Int64 = 0
            var bufferedMs: Int64 = 0

            if let item = p.currentItem {
                if item.duration.isNumeric {
                    durationMs = Int64(CMTimeGetSeconds(item.duration) * 1000)
                }
                if let timeRange = item.loadedTimeRanges.last?.timeRangeValue {
                    let bufferedEnd = CMTimeAdd(timeRange.start, timeRange.duration)
                    bufferedMs = Int64(CMTimeGetSeconds(bufferedEnd) * 1000)
                }
            }

            let playbackState: Int
            switch p.timeControlStatus {
            case .paused:
                playbackState = 6
            case .waitingToPlayAtSpecifiedRate:
                playbackState = 2
            case .playing:
                playbackState = 3
            @unknown default:
                playbackState = 0
            }

            PlatformChannelManager.shared.sendEvent(
                channel: "drift/audio_player/events",
                data: [
                    "playbackState": playbackState,
                    "positionMs": positionMs,
                    "durationMs": max(durationMs, 0),
                    "bufferedMs": bufferedMs
                ]
            )
        }

        return newPlayer
    }

    private static func sendStateEvent(state: Int) {
        guard let p = player else { return }
        let positionMs = Int64(CMTimeGetSeconds(p.currentTime()) * 1000)
        var durationMs: Int64 = 0
        var bufferedMs: Int64 = 0

        if let item = p.currentItem {
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
                "playbackState": state,
                "positionMs": positionMs,
                "durationMs": max(durationMs, 0),
                "bufferedMs": bufferedMs
            ]
        )
    }

    private static func load(args: [String: Any]?) -> (Any?, Error?) {
        guard let url = args?["url"] as? String,
              let mediaURL = URL(string: url) else {
            return (nil, NSError(domain: "AudioPlayer", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing url"]))
        }

        let p = ensurePlayer()
        let item = AVPlayerItem(url: mediaURL)
        p.replaceCurrentItem(with: item)

        // Observe item status for errors
        itemStatusObservation?.invalidate()
        itemStatusObservation = item.observe(\.status) { item, _ in
            if item.status == .failed {
                PlatformChannelManager.shared.sendEvent(
                    channel: "drift/audio_player/errors",
                    data: [
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
            ) { _ in
                player?.seek(to: .zero)
                player?.play()
            }
        }

        return (nil, nil)
    }

    private static func play() -> (Any?, Error?) {
        let p = ensurePlayer()
        p.play()
        if playbackSpeed != 1.0 {
            p.rate = playbackSpeed
        }
        return (nil, nil)
    }

    private static func pause() -> (Any?, Error?) {
        player?.pause()
        return (nil, nil)
    }

    private static func stop() -> (Any?, Error?) {
        player?.pause()
        player?.seek(to: .zero)
        return (nil, nil)
    }

    private static func seekTo(args: [String: Any]?) -> (Any?, Error?) {
        let positionMs = args?["positionMs"] as? Int64 ?? 0
        let time = CMTime(seconds: Double(positionMs) / 1000.0, preferredTimescale: CMTimeScale(NSEC_PER_SEC))
        player?.seek(to: time)
        return (nil, nil)
    }

    private static func setVolume(args: [String: Any]?) -> (Any?, Error?) {
        let volume = (args?["volume"] as? NSNumber)?.floatValue ?? 1.0
        player?.volume = volume
        return (nil, nil)
    }

    private static func setLooping(args: [String: Any]?) -> (Any?, Error?) {
        let looping = args?["looping"] as? Bool ?? false
        isLooping = looping

        // Remove existing loop observer
        if let observer = loopObserver {
            NotificationCenter.default.removeObserver(observer)
            loopObserver = nil
        }

        if looping, let item = player?.currentItem {
            loopObserver = NotificationCenter.default.addObserver(
                forName: .AVPlayerItemDidPlayToEndTime,
                object: item,
                queue: .main
            ) { _ in
                player?.seek(to: .zero)
                player?.play()
            }
        }

        return (nil, nil)
    }

    private static func setPlaybackSpeed(args: [String: Any]?) -> (Any?, Error?) {
        let rate = (args?["rate"] as? NSNumber)?.floatValue ?? 1.0
        playbackSpeed = rate
        if let p = player, p.timeControlStatus == .playing {
            p.rate = rate
        }
        return (nil, nil)
    }

    private static func dispose() -> (Any?, Error?) {
        if let observer = timeObserver {
            player?.removeTimeObserver(observer)
            timeObserver = nil
        }
        statusObservation?.invalidate()
        statusObservation = nil
        timeControlObservation?.invalidate()
        timeControlObservation = nil
        itemStatusObservation?.invalidate()
        itemStatusObservation = nil
        if let observer = loopObserver {
            NotificationCenter.default.removeObserver(observer)
            loopObserver = nil
        }
        player?.pause()
        player?.replaceCurrentItem(with: nil)
        player = nil
        playbackSpeed = 1.0
        isLooping = false

        try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)

        return (nil, nil)
    }
}
