/// DriftPlugins.swift
///
/// Owns the app's plugin instances and fans app, scene and view events out
/// to them. The AppDelegate, SceneDelegate (DriftApp on xtool) and
/// DriftViewController templates call in; the generated
/// DriftPluginRegistrant only lists the plugins. Drift's own handling of the
/// same events (deep links, notifications) stays in those templates.
///
/// Hand-written, shared by every build path: the scaffold copies it with the
/// iOS templates and EnsureRunnerSupport keeps ejected projects' copy
/// current. Main thread only.

@_spi(DriftHost) import DriftPluginAPI
import UIKit
import UserNotifications

final class DriftPlugins {
    static let shared = DriftPlugins()

    private var plugins: [DriftPlugin] = []
    private var launched = false
    /// The view controller plugins are attached to. An identity rather than
    /// a reference, so DriftViewController can detach from its deinit.
    private var attachedTo: ObjectIdentifier?

    private init() {}

    /// Creates the plugins, registers each with host, then delivers
    /// didFinishLaunching. Called once, from the app delegate. host must be
    /// fully constructed: this runs outside its initializer so a plugin can
    /// use the host (or PlatformChannelManager.shared) while registering.
    func launch(
        _ application: UIApplication,
        options: [UIApplication.LaunchOptionsKey: Any]?,
        host: DriftPluginHost
    ) {
        dispatchPrecondition(condition: .onQueue(.main))
        precondition(!launched, "drift: DriftPlugins.launch called twice")
        launched = true
        plugins = DriftPluginRegistrant.makePlugins()
        for plugin in plugins {
            plugin.register(host: host)
        }
        for plugin in plugins {
            plugin.didFinishLaunching(application, options: options)
        }
    }

    /// Attaches every plugin to the Drift view controller, detaching from a
    /// previous one first. overlayView is the controller's plugin overlay
    /// host (DriftViewBinding.overlayView).
    func attach(_ viewController: UIViewController, overlayView: UIView) {
        dispatchPrecondition(condition: .onQueue(.main))
        precondition(launched, "drift: DriftPlugins.attach before launch; call DriftPlugins.shared.launch from the app delegate")
        if attachedTo != nil {
            detachAll()
        }
        attachedTo = ObjectIdentifier(viewController)
        let binding = DriftViewBinding(overlayView: overlayView, viewController: viewController)
        for plugin in plugins {
            plugin.attach(binding)
        }
    }

    /// Detaches every plugin if they are attached to viewController.
    func detach(from viewController: ObjectIdentifier) {
        guard attachedTo == viewController else { return }
        detachAll()
    }

    private func detachAll() {
        attachedTo = nil
        for plugin in plugins.reversed() {
            plugin.detach()
        }
    }

    func didRegisterForRemoteNotifications(deviceToken: Data) {
        for plugin in plugins {
            plugin.didRegisterForRemoteNotifications(deviceToken: deviceToken)
        }
    }

    func didFailToRegisterForRemoteNotifications(error: Error) {
        for plugin in plugins {
            plugin.didFailToRegisterForRemoteNotifications(error: error)
        }
    }

    /// Offers a remote notification to each plugin in drift.yaml order; the
    /// one that claims it owns completion. Unclaimed: .noData.
    func didReceiveRemoteNotification(
        _ userInfo: [AnyHashable: Any],
        completion: @escaping (UIBackgroundFetchResult) -> Void
    ) {
        if plugins.contains(where: { $0.didReceiveRemoteNotification(userInfo, completion: completion) }) {
            return
        }
        completion(.noData)
    }

    /// Offers a foreground notification to each plugin in drift.yaml order.
    /// Returns the claiming plugin's presentation options, or nil.
    func willPresentNotification(_ notification: UNNotification) -> UNNotificationPresentationOptions? {
        dispatchPrecondition(condition: .onQueue(.main))
        for plugin in plugins {
            if let options = plugin.willPresentNotification(notification) {
                return options
            }
        }
        return nil
    }

    /// Offers a notification response to each plugin in drift.yaml order.
    /// Returns true if one claimed it.
    func didReceiveNotificationResponse(_ response: UNNotificationResponse) -> Bool {
        dispatchPrecondition(condition: .onQueue(.main))
        return plugins.contains { $0.didReceiveNotificationResponse(response) }
    }
}
