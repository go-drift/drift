/// DriftSplashPlugin.swift
///
/// The splash plugin's native half, driven by DriftPlugins on the main
/// thread:
///   1. register: the `drift/splash` channel (the Go runtime's Preserve and
///      Remove), the `drift/rendering/frame_events` observer (`first_frame`:
///      the app has drawn content) and the max_duration_ms timer, after
///      which Preserve no longer holds the splash.
///      Once per process.
///   2. attach: install the overlay on the binding's overlay view, unless
///      the splash may already go. DriftViewController attaches before its
///      window is visible, so the overlay takes over from the launch screen
///      without a flash.
///   3. detach: drop the overlay with the view.
///
/// Dismissal is state, not an event: every input calls reconcile(), so an
/// input arriving while detached or in the background is never lost.

import DriftPluginAPI
import OSLog
import UIKit

private let splashLog = OSLog(subsystem: "drift.splash", category: "plugin")

/// Public, with a public initializer: the app's generated registrant
/// creates it from outside this module (DriftPlugin_splash).
public final class DriftSplashPlugin: DriftPlugin {
    /// Outstanding Preserve calls.
    private var preserveCount = 0
    /// The app has drawn its first frame with content.
    private var contentShown = false
    /// max_duration_ms has passed: Preserve no longer holds the splash.
    private var timedOut = false
    /// The splash has gone for this process and never comes back.
    private var dismissed = false
    private var overlay: DriftSplashOverlayView?

    /// The splash always waits for content (a slow App.OnInit keeps it up,
    /// as Drift promises); the timeout only overrides a missed Remove.
    private var dismissible: Bool { contentShown && (preserveCount == 0 || timedOut) }

    public init() {}

    public func register(host: DriftPluginHost) {
        host.registerChannel("drift/splash") { channel in
            channel.method("preserve") { [self] _ in
                guard !dismissed else {
                    throw NSError(domain: "drift.splash", code: 2, userInfo: [
                        NSLocalizedDescriptionKey: "splash already dismissed; call Preserve before the first frame (App.OnInit or the root's InitState)",
                    ])
                }
                preserveCount += 1
                return nil
            }
            channel.method("remove") { [self] _ in
                preserveCount = max(0, preserveCount - 1)
                reconcile()
                return nil
            }
        }
        _ = host.observeEvent("drift/rendering/frame_events") { [self] data in
            guard let payload = data as? [String: Any],
                  payload["type"] as? String == "first_frame" else { return }
            contentShown = true
            reconcile()
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + .milliseconds(DriftSplashConfig.maxDurationMs)) { [self] in
            timedOut = true
            guard !dismissed, preserveCount > 0 else { return }
            os_log("splash held past max_duration_ms=%d by %d Preserve without Remove; ignoring them",
                   log: splashLog, type: .error,
                   DriftSplashConfig.maxDurationMs, preserveCount)
            reconcile()
        }
    }

    public func attach(_ binding: DriftViewBinding) {
        guard !dismissed else { return }
        if dismissible {
            dismissed = true
            return
        }
        let view = DriftSplashOverlayView()
        view.install(in: binding.overlayView)
        overlay = view
    }

    public func detach() {
        overlay?.removeFromSuperview()
        overlay = nil
    }

    /// Dismisses the splash once it may go. Fades the overlay if attached;
    /// otherwise the next attach installs none. In the background the fade
    /// completes at once, which is fine.
    private func reconcile() {
        guard !dismissed, dismissible else { return }
        dismissed = true
        guard let current = overlay else { return }
        overlay = nil
        current.fadeOut(durationMs: DriftSplashConfig.fadeDurationMs) {}
    }
}
