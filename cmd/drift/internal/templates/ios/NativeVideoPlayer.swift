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
    private let player: AVQueuePlayer
    private var timeObserver: Any?
    private var timeControlObservation: NSKeyValueObservation?
    private var itemStatusObservation: NSKeyValueObservation?
    private var endOfItemObserver: NSObjectProtocol?
    private var playerLooper: AVPlayerLooper?
    private var playbackSpeed: Float = 1.0
    private var isLooping: Bool = false
    private var hasReachedEnd: Bool = false
    private var isStopped: Bool = false

    init(viewId: Int, params: [String: Any]) {
        self.viewId = viewId

        DriftMediaSession.activate()

        let player = AVQueuePlayer()
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
            PlatformChannelManager.shared.sendEvent(
                channel: "drift/platform_views",
                data: [
                    "method": "onPlaybackStateChanged",
                    "viewId": self.viewId,
                    "state": state
                ]
            )
        }

        // Load media if URL provided
        if let urlString = params["url"] as? String, let url = URL(string: urlString) {
            loadItem(url: url)

            if autoPlay {
                player.play()
            }
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
    }

    private func stopPositionUpdates() {
        if let observer = timeObserver {
            player.removeTimeObserver(observer)
            timeObserver = nil
        }
    }

    /// Loads an AVPlayerItem from a URL, setting up observers and looping.
    private func loadItem(url: URL) {
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
                let error = item.error
                PlatformChannelManager.shared.sendEvent(
                    channel: "drift/platform_views",
                    data: [
                        "method": "onVideoError",
                        "viewId": self.viewId,
                        "code": mediaErrorCode(for: error),
                        "message": error?.localizedDescription ?? "Unknown playback error"
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
            PlatformChannelManager.shared.sendEvent(
                channel: "drift/platform_views",
                data: [
                    "method": "onPlaybackStateChanged",
                    "viewId": self.viewId,
                    "state": 3 // Completed
                ]
            )
        }

        // Create looper if looping is active
        if isLooping {
            playerLooper = AVPlayerLooper(player: player, templateItem: item)
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
        view.removeFromSuperview()

        DriftMediaSession.deactivate()
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
            PlatformChannelManager.shared.sendEvent(
                channel: "drift/platform_views",
                data: [
                    "method": "onPlaybackStateChanged",
                    "viewId": self.viewId,
                    "state": 0 // Idle with position reset to zero
                ]
            )
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

    func load(_ urlString: String) {
        guard let url = URL(string: urlString) else { return }
        loadItem(url: url)
    }
}
