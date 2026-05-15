/// DriftSplashPlugin.swift
///
/// Entry point invoked from the generated `DriftPluginRegistrant.registerAll`.
/// Responsibilities:
///   1. Install the overlay UIView synchronously on the active window before
///      the system launch screen tears down. (Eager
///      `PlatformChannelManager.shared` access in SceneDelegate guarantees
///      this register call runs in time.)
///   2. Register the `drift/splash` channel handler. The Go runtime calls
///      `preserve` / `remove`; both forward to `SplashState.apply(±1)`.
///   3. Subscribe to `drift/rendering/frame_events`. On `first_frame`, mark
///      state and dismiss if there are no outstanding preserves.

import OSLog
import UIKit

private let splashLog = OSLog(subsystem: "drift.splash", category: "plugin")

enum DriftSplashPlugin {

    private static var overlay: DriftSplashOverlayView?
    private static var frameSubscription: DriftSubscription?

    static func register(host: DriftPluginHost) {
        installOverlay(host: host)
        registerChannelHandler(host: host)
        observeFirstFrame(host: host)
    }

    /// 1. Install the overlay synchronously. DriftOverlayHost is an optional
    ///    capability protocol; bail with a logged warning if the host
    ///    doesn't adopt it or has no active root view. Channel + observer
    ///    still install in those cases so Preserve/Remove + state tracking
    ///    keep working; only the visible overlay is missing.
    private static func installOverlay(host: DriftPluginHost) {
        guard let overlayHost = host as? DriftOverlayHost else {
            os_log("host does not implement DriftOverlayHost; runtime overlay disabled (likely framework/plugin version mismatch)",
                   log: splashLog, type: .info)
            return
        }
        guard let rootView = overlayHost.driftRootView() else {
            os_log("DriftOverlayHost.driftRootView() returned nil; runtime overlay disabled (no active scene?)",
                   log: splashLog, type: .info)
            return
        }
        let view = DriftSplashOverlayView()
        view.frame = rootView.bounds
        view.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        rootView.addSubview(view)
        overlay = view
    }

    /// 2. Channel handler: named methods matching the Go API.
    ///    SplashState.apply(±1) is the single saturation site; clamps a
    ///    Remove without a matching Preserve to zero.
    private static func registerChannelHandler(host: DriftPluginHost) {
        host.registerChannel("drift/splash") { method, _ in
            switch method {
            case "preserve":
                DriftSplashState.shared.apply(1)
                maybeDismiss()
                return (nil, nil)
            case "remove":
                DriftSplashState.shared.apply(-1)
                maybeDismiss()
                return (nil, nil)
            default:
                return (nil, NSError(domain: "drift.splash", code: 1, userInfo: [
                    NSLocalizedDescriptionKey: "unknown splash method \(method)",
                ]))
            }
        }
    }

    /// 3. Subscribe to frame events via the host. The subscription token is
    ///    retained on the plugin so cleanup after iOS scene-recreate
    ///    scenarios in v2 stays straightforward.
    private static func observeFirstFrame(host: DriftPluginHost) {
        frameSubscription = host.observeEvent("drift/rendering/frame_events") { data in
            guard let payload = data as? [String: Any],
                  let type = payload["type"] as? String,
                  type == "first_frame" else { return }
            DriftSplashState.shared.markFirstFrame()
            maybeDismiss()
        }
    }

    private static func maybeDismiss() {
        guard DriftSplashState.shared.canDismiss() else { return }
        DispatchQueue.main.async {
            guard UIApplication.shared.applicationState == .active else { return }
            overlay?.fadeOut(durationMs: DriftSplashConfig.fadeDurationMs) {
                overlay = nil
            }
        }
    }
}
