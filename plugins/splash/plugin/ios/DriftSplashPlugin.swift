/// DriftSplashPlugin.swift
///
/// The splash plugin's native half, driven by DriftPlugins:
///   1. register: the `drift/splash` channel (the Go runtime calls
///      `preserve` / `remove`, both forwarding to SplashState.apply(±1)) and
///      the `drift/rendering/frame_events` observer (on `first_frame`, mark
///      state and dismiss if nothing preserves the splash). Once per process.
///   2. attach: install the overlay on the Drift view. DriftViewController
///      attaches before its window becomes visible, so the overlay takes
///      over from the launch storyboard without a flash.
///   3. detach: drop the overlay with the view.

import DriftPluginAPI
import OSLog
import UIKit

private let splashLog = OSLog(subsystem: "drift.splash", category: "plugin")

/// Public, with a public initializer: the app's generated registrant
/// creates it from outside this module (DriftPlugin_splash).
public final class DriftSplashPlugin: DriftPlugin {
    private var overlay: DriftSplashOverlayView?
    /// Set once the overlay has faded out; a later attach (a new Drift view)
    /// does not bring the splash back.
    private var dismissed = false

    public init() {}

    public func register(host: DriftPluginHost) {
        host.registerChannel("drift/splash") { [self] method, _, result in
            switch method {
            case "preserve":
                DriftSplashState.shared.apply(1)
                maybeDismiss()
                result.success(nil)
            case "remove":
                DriftSplashState.shared.apply(-1)
                maybeDismiss()
                result.success(nil)
            default:
                result.error(NSError(domain: "drift.splash", code: 1, userInfo: [
                    NSLocalizedDescriptionKey: "unknown splash method \(method)",
                ]))
            }
        }
        _ = host.observeEvent("drift/rendering/frame_events") { [self] data in
            guard let payload = data as? [String: Any],
                  let type = payload["type"] as? String,
                  type == "first_frame" else { return }
            DriftSplashState.shared.markFirstFrame()
            maybeDismiss()
        }
    }

    public func attach(_ binding: DriftViewBinding) {
        guard !dismissed else { return }
        let view = DriftSplashOverlayView()
        view.install(in: binding.overlayView)
        overlay = view
        os_log("splash overlay attached", log: splashLog, type: .debug)
    }

    public func detach() {
        overlay?.removeFromSuperview()
        overlay = nil
    }

    /// Called on the main thread, where the host runs plugin handlers and
    /// observers.
    private func maybeDismiss() {
        guard !dismissed,
              DriftSplashState.shared.canDismiss(),
              UIApplication.shared.applicationState == .active,
              let current = overlay else { return }
        dismissed = true
        current.fadeOut(durationMs: DriftSplashConfig.fadeDurationMs) { [self] in
            if overlay === current {
                overlay = nil
            }
        }
    }
}
