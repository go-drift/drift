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
