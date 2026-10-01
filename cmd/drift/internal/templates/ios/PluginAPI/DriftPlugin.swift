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
///   4. The app-level hooks, as UIKit delivers them.
///
/// Every requirement except register(host:) has a default no-op
/// implementation, so a plugin implements only what it uses.

import UIKit

public protocol DriftPlugin: AnyObject {
    func register(host: DriftPluginHost)

    func didFinishLaunching(_ application: UIApplication, options: [UIApplication.LaunchOptionsKey: Any]?)

    func attach(_ binding: DriftViewBinding)
    func detach()

    /// Offers the plugin a URL the app was opened with. Return true to claim
    /// it: the first plugin (drift.yaml order) to claim a URL stops dispatch,
    /// and Drift's deep-link channel does not see it.
    func open(_ url: URL) -> Bool

    /// Same claim semantics as open(_:), for universal links and Handoff.
    func continueUserActivity(_ userActivity: NSUserActivity) -> Bool

    func didRegisterForRemoteNotifications(deviceToken: Data)
    func didFailToRegisterForRemoteNotifications(error: Error)

    /// Return true if the plugin will call completion exactly once, false to
    /// decline. iOS gets the merged result of the plugins that accept (see
    /// DriftPluginCoordinator), within its background-fetch deadline.
    func didReceiveRemoteNotification(
        _ userInfo: [AnyHashable: Any],
        completion: @escaping (UIBackgroundFetchResult) -> Void
    ) -> Bool
}

public extension DriftPlugin {
    func didFinishLaunching(_ application: UIApplication, options: [UIApplication.LaunchOptionsKey: Any]?) {}
    func attach(_ binding: DriftViewBinding) {}
    func detach() {}
    func open(_ url: URL) -> Bool { false }
    func continueUserActivity(_ userActivity: NSUserActivity) -> Bool { false }
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
    /// The full-screen view Drift renders into. Overlays added as subviews
    /// should follow its bounds (autoresizing mask or constraints) so
    /// rotation needs no extra work.
    public let rootView: UIView
    /// The view controller that owns rootView, for presenting UI.
    public let viewController: UIViewController
}
