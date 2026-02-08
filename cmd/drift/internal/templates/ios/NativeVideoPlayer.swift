/// NativeVideoPlayer.swift
/// Provides native AVPlayer video playback embedded in Drift UI.

import UIKit
import AVKit

// MARK: - Native Video Player Container

/// Platform view container for native video player using AVPlayerViewController.
/// Provides full native controls (play/pause, seek, AirPlay, PiP, playback speed).
class NativeVideoPlayerContainer: NSObject, PlatformViewContainer {
    let viewId: Int
    let view: UIView
    private let playerVC: AVPlayerViewController
    private let player: AVPlayer
    private var timeObserver: Any?
    private var statusObservation: NSKeyValueObservation?
    private var timeControlObservation: NSKeyValueObservation?
    private var itemStatusObservation: NSKeyValueObservation?
    private var playbackSpeed: Float = 1.0
    private var isLooping: Bool = false

    init(viewId: Int, params: [String: Any]) {
        self.viewId = viewId

        DriftMediaSession.activate()

        let player = AVPlayer()
        self.player = player

        let playerVC = AVPlayerViewController()
        playerVC.player = player
        playerVC.showsPlaybackControls = true
        self.playerVC = playerVC

        // Use the player view controller's view as our container view
        self.view = playerVC.view
        self.view.backgroundColor = .black

        super.init()

        // Configure from params
        let looping = params["looping"] as? Bool ?? false
        let volume = (params["volume"] as? NSNumber)?.floatValue ?? 1.0
        let autoPlay = params["autoPlay"] as? Bool ?? false

        self.isLooping = looping
        player.volume = volume

        // Observe player time control status for playback state
        timeControlObservation = player.observe(\.timeControlStatus) { [weak self] player, _ in
            guard let self = self else { return }
            let state: Int
            switch player.timeControlStatus {
            case .paused:
                // Check if playback completed
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
            PlatformChannelManager.shared.sendEvent(
                channel: "drift/platform_views",
                data: [
                    "method": "onPlaybackStateChanged",
                    "viewId": self.viewId,
                    "state": state
                ]
            )
        }

        // Add periodic time observer for position updates
        let interval = CMTime(seconds: 0.25, preferredTimescale: CMTimeScale(NSEC_PER_SEC))
        timeObserver = player.addPeriodicTimeObserver(forInterval: interval, queue: .main) { [weak self] time in
            guard let self = self else { return }
            let positionMs = Int64(CMTimeGetSeconds(time) * 1000)
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
                channel: "drift/platform_views",
                data: [
                    "method": "onPositionChanged",
                    "viewId": self.viewId,
                    "positionMs": positionMs,
                    "durationMs": max(durationMs, 0),
                    "bufferedMs": bufferedMs
                ]
            )
        }

        // Load media if URL provided
        if let urlString = params["url"] as? String, let url = URL(string: urlString) {
            let item = AVPlayerItem(url: url)
            player.replaceCurrentItem(with: item)

            // Observe item status for errors
            itemStatusObservation = item.observe(\.status) { [weak self] item, _ in
                guard let self = self else { return }
                if item.status == .failed {
                    let error = item.error
                    PlatformChannelManager.shared.sendEvent(
                        channel: "drift/platform_views",
                        data: [
                            "method": "onVideoError",
                            "viewId": self.viewId,
                            "code": Self.errorCode(for: error),
                            "message": error?.localizedDescription ?? "Unknown playback error"
                        ]
                    )
                }
            }

            if autoPlay {
                player.play()
            }

            // Handle looping
            if looping {
                NotificationCenter.default.addObserver(
                    self,
                    selector: #selector(playerDidFinishPlaying),
                    name: .AVPlayerItemDidPlayToEndTime,
                    object: item
                )
            }
        }
    }

    @objc private func playerDidFinishPlaying(_ notification: Notification) {
        player.seek(to: .zero)
        player.play()
    }

    /// Maps an AVPlayer error to a canonical Drift error code string.
    /// Aligns with the Android ExoPlayer mapping so that both platforms
    /// produce the same set of codes: "source_error", "decoder_error",
    /// "playback_failed".
    static func errorCode(for error: Error?) -> String {
        guard let error = error else { return "playback_failed" }

        if let avError = error as? AVError {
            switch avError.code {
            case .decoderNotFound, .decoderTemporarilyUnavailable,
                 .contentIsNotAuthorized:
                return "decoder_error"
            case .fileFormatNotRecognized, .failedToParse:
                return "source_error"
            default:
                break
            }
        }

        if (error as NSError).domain == NSURLErrorDomain {
            return "source_error"
        }

        return "playback_failed"
    }

    func dispose() {
        if let observer = timeObserver {
            player.removeTimeObserver(observer)
            timeObserver = nil
        }
        statusObservation?.invalidate()
        statusObservation = nil
        timeControlObservation?.invalidate()
        timeControlObservation = nil
        itemStatusObservation?.invalidate()
        itemStatusObservation = nil
        NotificationCenter.default.removeObserver(self)
        player.pause()
        player.replaceCurrentItem(with: nil)
        view.removeFromSuperview()

        DriftMediaSession.deactivate()
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
        // Remove existing observer
        NotificationCenter.default.removeObserver(self, name: .AVPlayerItemDidPlayToEndTime, object: nil)
        if looping, let item = player.currentItem {
            NotificationCenter.default.addObserver(
                self,
                selector: #selector(playerDidFinishPlaying),
                name: .AVPlayerItemDidPlayToEndTime,
                object: item
            )
        }
    }

    func setPlaybackSpeed(_ rate: Float) {
        playbackSpeed = rate
        if player.timeControlStatus == .playing {
            player.rate = rate
        }
    }

    func loadUrl(_ urlString: String) {
        guard let url = URL(string: urlString) else { return }
        let item = AVPlayerItem(url: url)
        player.replaceCurrentItem(with: item)

        // Re-observe item status for errors
        itemStatusObservation?.invalidate()
        itemStatusObservation = item.observe(\.status) { [weak self] item, _ in
            guard let self = self else { return }
            if item.status == .failed {
                let error = item.error
                PlatformChannelManager.shared.sendEvent(
                    channel: "drift/platform_views",
                    data: [
                        "method": "onVideoError",
                        "viewId": self.viewId,
                        "code": Self.errorCode(for: error),
                        "message": error?.localizedDescription ?? "Unknown playback error"
                    ]
                )
            }
        }

        // Re-attach loop observer to the new item if looping is active
        if isLooping {
            NotificationCenter.default.removeObserver(self, name: .AVPlayerItemDidPlayToEndTime, object: nil)
            NotificationCenter.default.addObserver(
                self,
                selector: #selector(playerDidFinishPlaying),
                name: .AVPlayerItemDidPlayToEndTime,
                object: item
            )
        }
    }
}
