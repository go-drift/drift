/// AppDelegate.swift
/// Application delegate for the Drift iOS application.
///
/// This class serves as the entry point for the iOS application, handling
/// application-level lifecycle events. It uses the modern scene-based lifecycle
/// (introduced in iOS 13) where most UI setup is delegated to SceneDelegate.
///
/// Architecture:
///
///     iOS System
///         │
///         ▼ App launch
///     AppDelegate (this file)
///         │
///         ▼ Scene configuration
///     SceneDelegate
///         │
///         ▼ Window setup
///     DriftViewController
///
/// Scene-Based Lifecycle:
///   In iOS 13+, a single app can have multiple windows (scenes), each with its
///   own lifecycle. AppDelegate handles app-wide events, while SceneDelegate
///   handles per-window events. For this single-window app, there's one scene.
///
/// Why @main:
///   The @main attribute designates this class as the application's entry point,
///   replacing the older UIApplicationMain call. Swift generates the main()
///   function automatically.

import UIKit
import UserNotifications

/// The application delegate that manages application-level lifecycle events.
///
/// Conforms to UIApplicationDelegate to receive callbacks for app-level events
/// such as launch, termination, and scene session management.
@main
class AppDelegate: UIResponder, UIApplicationDelegate {

    /// Called when the application finishes launching.
    ///
    /// All built-in dispatchers (NotificationHandler.start, etc.) and plugin
    /// registrant fan-out happen inside DriftPluginRegistrant. AppDelegate
    /// methods are thin pass-throughs; the registrant is the single dispatch
    /// path. UIKit's launch Bool is "did I handle the launch URL," not
    /// "abort launch," so the registrant is Void and AppDelegate returns
    /// `true` unconditionally.
    func application(
        _ application: UIApplication,
        didFinishLaunchingWithOptions launchOptions: [UIApplication.LaunchOptionsKey: Any]?
    ) -> Bool {
        DriftPluginRegistrant.didFinishLaunching(application: application, launchOptions: launchOptions)
        return true
    }

    func application(
        _ application: UIApplication,
        didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data
    ) {
        DriftPluginRegistrant.didRegisterForRemoteNotifications(application: application, deviceToken: deviceToken)
    }

    func application(
        _ application: UIApplication,
        didFailToRegisterForRemoteNotificationsWithError error: Error
    ) {
        DriftPluginRegistrant.didFailToRegisterForRemoteNotifications(application: application, error: error)
    }

    func application(
        _ application: UIApplication,
        didReceiveRemoteNotification userInfo: [AnyHashable: Any],
        fetchCompletionHandler completionHandler: @escaping (UIBackgroundFetchResult) -> Void
    ) {
        DriftPluginRegistrant.didReceiveRemoteNotification(
            application: application,
            userInfo: userInfo,
            completionHandler: completionHandler
        )
    }

    /// Provides the configuration for a new scene session.
    ///
    /// Called when the system is about to create a new scene (window). This method
    /// returns a UISceneConfiguration that specifies the scene delegate class and
    /// storyboard to use.
    ///
    /// - Parameters:
    ///   - application: The singleton app instance.
    ///   - connectingSceneSession: The session object for the new scene.
    ///   - options: Options that may affect scene configuration (e.g., user activities).
    ///
    /// - Returns: A scene configuration specifying how to set up the new scene.
    ///            The "Default Configuration" name must match Info.plist.
    func application(
        _ application: UIApplication,
        configurationForConnecting connectingSceneSession: UISceneSession,
        options: UIScene.ConnectionOptions
    ) -> UISceneConfiguration {
        // Create a configuration using the "Default Configuration" defined in Info.plist.
        // This configuration specifies SceneDelegate as the delegate class.
        UISceneConfiguration(name: "Default Configuration", sessionRole: connectingSceneSession.role)
    }
}
