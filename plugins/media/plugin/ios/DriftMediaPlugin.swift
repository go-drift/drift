/// DriftMediaPlugin.swift
///
/// The media plugin's native half on iOS: the `drift/media/video` platform
/// view (VideoPlayerView) and the `drift/media/audio` channel
/// (AudioPlayers). When Drift warms up its platform views at launch, an
/// AVPlayer is created and dropped so the first video opens faster.
///
/// Playback stops when the app goes to the background: every player
/// pauses when the app enters the background, and stays paused when the
/// app returns.

import AVFoundation
import DriftPluginAPI
import UIKit

/// Public, with a public initializer: the app's generated registrant
/// creates it from outside this module (DriftPlugin_media).
public final class DriftMediaPlugin: DriftPlugin {
    private var audio: AudioPlayers?
    /// The video views not yet disposed.
    private var videos: [ObjectIdentifier: VideoPlayerView] = [:]
    private var backgroundObserver: NSObjectProtocol?

    public init() {}

    public func register(host: DriftPluginHost) {
        host.registerPlatformView("drift/media/video") { [self] _, params, events in
            let view = VideoPlayerView(params: params, events: events) { [weak self] in
                self?.videos[ObjectIdentifier($0)] = nil
            }
            videos[ObjectIdentifier(view)] = view
            return view
        }
        let audio = AudioPlayers { [weak host] channel, data in
            host?.sendEvent(channel, data: data)
        }
        self.audio = audio
        host.registerChannel(AudioPlayers.channel) { audio.declare($0) }
        _ = host.observeEvent(driftPlatformViewWarmUpEvent) { _ in
            _ = AVPlayer()
        }
        backgroundObserver = NotificationCenter.default.addObserver(
            forName: UIApplication.didEnterBackgroundNotification,
            object: nil,
            queue: .main
        ) { [weak self] _ in
            self?.pauseAll()
        }
    }

    /// Pauses every player; each reports Paused to Go if it was playing.
    private func pauseAll() {
        audio?.pauseAll()
        videos.values.forEach { $0.pause() }
    }
}
