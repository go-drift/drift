/// DriftMediaPlugin.swift
///
/// The media plugin's native half on iOS: the `drift/media/video` platform
/// view (VideoPlayerView) and the `drift/media/audio` channel
/// (AudioPlayers). When Drift warms up its platform views at launch, an
/// AVPlayer is created and dropped so the first video opens faster.

import AVFoundation
import DriftPluginAPI

/// Public, with a public initializer: the app's generated registrant
/// creates it from outside this module (DriftPlugin_media).
public final class DriftMediaPlugin: DriftPlugin {
    private var audio: AudioPlayers?

    public init() {}

    public func register(host: DriftPluginHost) {
        host.registerPlatformView("drift/media/video") { _, params, events in
            VideoPlayerView(params: params, events: events)
        }
        let audio = AudioPlayers { [weak host] channel, data in
            host?.sendEvent(channel, data: data)
        }
        self.audio = audio
        host.registerChannel(AudioPlayers.channel) { audio.declare($0) }
        _ = host.observeEvent(driftPlatformViewWarmUpEvent) { _ in
            _ = AVPlayer()
        }
    }
}
