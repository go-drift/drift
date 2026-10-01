/// DriftPlugin.swift
///
/// The native half of a Drift plugin on iOS. The generated
/// DriftPluginRegistrant creates one instance of each configured plugin per
/// process, in drift.yaml order, and DriftPlugins drives it on the main
/// thread:
///
///   1. register(host:) once at launch, before any view exists. Register
///      channels and event observers here.
///   2. didFinishLaunching(_:options:) once, after every plugin registered.
///   3. attach(_:) when the Drift view has loaded, and detach() when it goes
///      away. View-bound state (overlays, view references) lives between the
///      two.
///   4. The app-level hooks, as UIKit delivers them. A hook returning Bool
///      or an optional claims the event: the first plugin (drift.yaml
///      order) to claim it stops dispatch.
///
/// Every requirement except register(host:) has a default no-op
/// implementation, so a plugin implements only what it uses.

import UIKit

public protocol DriftPlugin: AnyObject {
    func register(host: DriftPluginHost)

    func didFinishLaunching(_ application: UIApplication, options: [UIApplication.LaunchOptionsKey: Any]?)

    func attach(_ binding: DriftViewBinding)
    func detach()

    func didRegisterForRemoteNotifications(deviceToken: Data)
    func didFailToRegisterForRemoteNotifications(error: Error)

    /// Offers a remote notification the app received (a content-available
    /// push, in the foreground or woken in the background). Return true to
    /// claim it, then call completion exactly once within iOS's 30-second
    /// background-fetch deadline; return false to leave it to later plugins
    /// (drift.yaml order). Push payloads are provider-specific, so a plugin
    /// claims only its own provider's. Unclaimed: iOS is told .noData.
    func didReceiveRemoteNotification(
        _ userInfo: [AnyHashable: Any],
        completion: @escaping (UIBackgroundFetchResult) -> Void
    ) -> Bool
}

public extension DriftPlugin {
    func didFinishLaunching(_ application: UIApplication, options: [UIApplication.LaunchOptionsKey: Any]?) {}
    func attach(_ binding: DriftViewBinding) {}
    func detach() {}
    func didRegisterForRemoteNotifications(deviceToken: Data) {}
    func didFailToRegisterForRemoteNotifications(error: Error) {}
    func didReceiveRemoteNotification(
        _ userInfo: [AnyHashable: Any],
        completion: @escaping (UIBackgroundFetchResult) -> Void
    ) -> Bool { false }
}

/// The Drift view a plugin is attached to. Valid from attach(_:) until
/// detach(); a plugin must not keep it past detach().
public struct DriftViewBinding {
    /// A full-screen view for plugin overlays (a splash, say), kept above
    /// Drift's content and its platform views. Touches pass through where
    /// it has no subviews. Overlays should follow its bounds (constraints or
    /// autoresizing mask) so rotation needs no extra work.
    public let overlayView: UIView
    /// The view controller hosting the Drift view, for presenting UI.
    public let viewController: UIViewController

    @_spi(DriftHost) public init(overlayView: UIView, viewController: UIViewController) {
        self.overlayView = overlayView
        self.viewController = viewController
    }
}
