/// DriftOverlayHost.swift
///
/// Capability protocol adopted by `PlatformChannelManager` so plugins can
/// install full-screen overlay views (splash screens, debug HUDs, etc.)
/// without depending on host internals. Plugins down-cast the host they
/// receive from `DriftPluginRegistrant.registerAll(host:)`:
///
///     guard let overlayHost = host as? DriftOverlayHost,
///           let rootView = overlayHost.driftRootView() else { return }
///     rootView.addSubview(myOverlayView)
///
/// Kept in a separate file from `DriftPluginHost` so the core channel
/// protocol stays narrow. Plugins that don't need overlay access don't
/// see this surface.

import UIKit

protocol DriftOverlayHost: AnyObject {
    /// Returns the active root view onto which plugins may install overlay
    /// subviews. Returns nil if no scene/window is currently connected.
    ///
    /// Overlays installed via this hook should pin to the returned view's
    /// bounds using auto-layout constraints, not static frames, so rotation
    /// tracks for free.
    func driftRootView() -> UIView?
}
