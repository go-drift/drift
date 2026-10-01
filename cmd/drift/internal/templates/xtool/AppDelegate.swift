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

    /// Creates Drift's platform channels and the app's plugins before
    /// SwiftUI creates the window, so plugins are registered before the
    /// Drift view exists and before any launch URL is routed.
    func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    ) -> Bool {
        DriftPlugins.shared.launch(application, options: launchOptions, host: PlatformChannelManager.shared)
        return true
    }

    func application(
        _ application: UIApplication,
        didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data
    ) {
        NotificationHandler.handleDeviceToken(deviceToken)
        DriftPlugins.shared.didRegisterForRemoteNotifications(deviceToken: deviceToken)
    }

    func application(
        _ application: UIApplication,
        didFailToRegisterForRemoteNotificationsWithError error: Error
    ) {
        NotificationHandler.handleRemoteNotificationError(error)
        DriftPlugins.shared.didFailToRegisterForRemoteNotifications(error: error)
    }

    func application(
        _ application: UIApplication,
        didReceiveRemoteNotification userInfo: [AnyHashable: Any],
        fetchCompletionHandler completionHandler: @escaping (UIBackgroundFetchResult) -> Void
    ) {
        NotificationHandler.handleRemoteNotification(userInfo, isForeground: application.applicationState == .active)
        DriftPlugins.shared.didReceiveRemoteNotification(userInfo, completion: completionHandler)
    }
}
