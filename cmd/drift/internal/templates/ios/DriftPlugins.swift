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

import UIKit

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
    /// previous one first.
    func attach(_ viewController: UIViewController) {
        dispatchPrecondition(condition: .onQueue(.main))
        precondition(launched, "drift: DriftPlugins.attach before launch; call DriftPlugins.shared.launch from the app delegate")
        if attachedTo != nil {
            detachAll()
        }
        attachedTo = ObjectIdentifier(viewController)
        let binding = DriftViewBinding(rootView: viewController.view, viewController: viewController)
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

    /// Offers url to each plugin in drift.yaml order. Returns true if one
    /// claimed it.
    func open(_ url: URL) -> Bool {
        plugins.contains { $0.open(url) }
    }

    /// Offers userActivity to each plugin in drift.yaml order. Returns true
    /// if one claimed it.
    func continueUserActivity(_ userActivity: NSUserActivity) -> Bool {
        plugins.contains { $0.continueUserActivity(userActivity) }
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

    func didReceiveRemoteNotification(
        _ userInfo: [AnyHashable: Any],
        completion: @escaping (UIBackgroundFetchResult) -> Void
    ) {
        let handlers = plugins.map { plugin -> DriftPluginCoordinator.BackgroundFetchHandler in
            { userInfo, done in plugin.didReceiveRemoteNotification(userInfo, completion: done) }
        }
        DriftPluginCoordinator.dispatchBackgroundFetch(
            userInfo: userInfo,
            handlers: handlers,
            completionHandler: completion
        )
    }
}
