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

    /// Creates the plugins, registers each with its own view of host (its
    /// config, and the drift/ channel names closed to third-party plugins),
    /// then delivers didFinishLaunching. Called once, from the app delegate.
    /// host must be fully constructed: this runs outside its initializer so
    /// a plugin can use the host (or PlatformChannelManager.shared) while
    /// registering.
    func launch(
        _ application: UIApplication,
        options: [UIApplication.LaunchOptionsKey: Any]?,
        host: DriftPluginHost
    ) {
        dispatchPrecondition(condition: .onQueue(.main))
        precondition(!launched, "drift: DriftPlugins.launch called twice")
        launched = true
        let entries = DriftPluginRegistrant.makePlugins()
        plugins = entries.map(\.plugin)
        for entry in entries {
            entry.plugin.register(host: PluginHost(host, entry))
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

    /// Detaches every plugin if they are attached to viewController. Called
    /// from DriftViewController.tearDown when its scene disconnects, and
    /// from its deinit.
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

    /// Offers a URL the app was opened with to each plugin in drift.yaml
    /// order. Returns true if one claimed it; otherwise the caller hands it
    /// to DeepLinkHandler.
    func openURL(_ url: URL) -> Bool {
        dispatchPrecondition(condition: .onQueue(.main))
        return plugins.contains { $0.openURL(url) }
    }

    /// Offers a continued user activity to each plugin in drift.yaml order.
    /// Returns true if one claimed it; otherwise the caller hands its
    /// webpageURL to DeepLinkHandler.
    func continueUserActivity(_ activity: NSUserActivity) -> Bool {
        dispatchPrecondition(condition: .onQueue(.main))
        return plugins.contains { $0.continueUserActivity(activity) }
    }
}

/// One plugin the registrant lists: its Name(), whether it ships in the
/// drift module, and its instance.
struct DriftPluginEntry {
    let name: String
    let firstParty: Bool
    let plugin: DriftPlugin
}

/// The host one plugin registers with: Drift's, with the plugin's own
/// config, and Drift's reserved drift/ channel names closed unless the
/// plugin is first-party.
private final class PluginHost: DriftPluginHost {
    private let host: DriftPluginHost
    private let entry: DriftPluginEntry
    let config: [String: Any]

    init(_ host: DriftPluginHost, _ entry: DriftPluginEntry) {
        self.host = host
        self.entry = entry
        config = Self.readConfig(entry.name)
    }

    /// The config the build shipped for plugin name
    /// (protocol.IOSPluginConfigFile), or none.
    private static func readConfig(_ name: String) -> [String: Any] {
        guard let url = Bundle.main.url(forResource: "DriftPluginConfig-\(name)", withExtension: "json") else {
            return [:]
        }
        guard let data = try? Data(contentsOf: url),
              let config = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] else {
            fatalError("drift: plugin \(name)'s config file \(url.lastPathComponent) is not a JSON object")
        }
        return config
    }

    func registerChannel(_ name: String, _ declare: (DriftChannel) -> Void) {
        if name.hasPrefix("drift/") && !entry.firstParty {
            fatalError("drift: plugin \(entry.name) registers channel \"\(name)\"; drift/ is reserved for Drift, so name it <vendor>/<feature>")
        }
        host.registerChannel(name, declare)
    }

    func sendEvent(_ channel: String, data: Any?) {
        host.sendEvent(channel, data: data)
    }

    func sendEventError(_ channel: String, _ error: Error) {
        host.sendEventError(channel, error)
    }

    func sendEventDone(_ channel: String) {
        host.sendEventDone(channel)
    }

    func observeEvent(_ channel: String, handler: @escaping (Any?) -> Void) -> DriftSubscription {
        host.observeEvent(channel, handler: handler)
    }

    func registerPermission(_ kind: String, _ permission: DriftPermission) {
        host.registerPermission(kind, permission)
    }

    func registerPlatformView(_ type: String, _ factory: @escaping DriftPlatformViewFactory) {
        host.registerPlatformView(type, factory)
    }
}
