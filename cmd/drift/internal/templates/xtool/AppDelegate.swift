/// AppDelegate.swift
/// Application delegate for the Drift iOS application (xtool/SwiftUI version).
///
/// This class handles application-level lifecycle events. The entry point
/// is provided by DriftApp.swift using SwiftUI's @main App pattern.
/// Scene management is handled by SwiftUI's WindowGroup.

import UIKit
import UserNotifications

/// The application delegate that manages application-level lifecycle events.
class AppDelegate: NSObject, UIApplicationDelegate {

    func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    ) -> Bool {
        // Force eager init of PlatformChannelManager.shared. The singleton is
        // lazy; without this touch, plugin registration only runs the first
        // time Go invokes a method via the channel, which can land after the
        // system launch screen has torn down. Plugins that install UI
        // overlays during register (e.g. the native splash plugin) need to
        // attach synchronously while the launch screen is still visible to
        // avoid a one-frame flash. (The iOS template does this in
        // SceneDelegate; xtool uses SwiftUI's @main App pattern with no
        // SceneDelegate, so the touch lives here.)
        _ = PlatformChannelManager.shared

        NotificationHandler.start()
        return true
    }

    func application(
        _ application: UIApplication,
        open url: URL,
        options: [UIApplication.OpenURLOptionsKey: Any] = [:]
    ) -> Bool {
        DeepLinkHandler.handle(url: url, source: "open_url")
        return true
    }

    func application(
        _ application: UIApplication,
        didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data
    ) {
        NotificationHandler.handleDeviceToken(deviceToken)
    }

    func application(
        _ application: UIApplication,
        didFailToRegisterForRemoteNotificationsWithError error: Error
    ) {
        NotificationHandler.handleRemoteNotificationError(error)
    }

    func application(
        _ application: UIApplication,
        didReceiveRemoteNotification userInfo: [AnyHashable: Any],
        fetchCompletionHandler completionHandler: @escaping (UIBackgroundFetchResult) -> Void
    ) {
        NotificationHandler.handleRemoteNotification(userInfo, isForeground: application.applicationState == .active)
        completionHandler(.newData)
    }
}
