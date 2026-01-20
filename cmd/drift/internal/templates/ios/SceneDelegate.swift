/// SceneDelegate.swift
/// Scene delegate for the Drift iOS application.
///
/// This class manages the lifecycle of a single window (scene) in the app.
/// It's responsible for creating the window hierarchy and setting up the
/// root view controller that displays the Drift engine output.
///
/// Architecture:
///
///     UIScene (managed by iOS)
///         │
///         ▼ willConnectTo
///     SceneDelegate (this file)
///         │
///         ▼ Creates window
///     UIWindow
///         │
///         ▼ rootViewController
///     DriftViewController
///         │
///         ▼ view
///     DriftMetalView
///
/// Scene Lifecycle:
///   - scene(_:willConnectTo:options:): Initial scene setup, creates window
///   - sceneDidBecomeActive: App became foreground (could resume rendering)
///   - sceneWillResignActive: App going to background (could pause rendering)
///   - sceneDidDisconnect: Scene destroyed (cleanup resources)
///
/// For this demo, we only implement the initial connection since the
/// DriftViewController handles display link lifecycle automatically.

import UIKit

/// Manages the window and UI lifecycle for a single scene.
///
/// In iOS 13+, each window is represented by a UIScene, and each scene has
/// a delegate that handles its lifecycle. This delegate creates and manages
/// the main window for the Drift application.
class SceneDelegate: UIResponder, UIWindowSceneDelegate {

    /// The main window for this scene.
    ///
    /// UIWindow is the container for all views and coordinates touch delivery.
    /// We hold a strong reference to keep the window alive for the scene's lifetime.
    var window: UIWindow?

    /// Called when a scene is about to connect to a session.
    ///
    /// This is where we create the window hierarchy and set up the initial UI.
    /// It's called once when the scene is first created.
    ///
    /// - Parameters:
    ///   - scene: The scene object being connected. Must be cast to UIWindowScene.
    ///   - session: The session object containing scene configuration.
    ///   - connectionOptions: Options for the connection (e.g., URL contexts).
    func scene(
        _ scene: UIScene,
        willConnectTo session: UISceneSession,
        options connectionOptions: UIScene.ConnectionOptions
    ) {
        // Ensure we have a window scene (not some other scene type).
        // This guard handles the case where scene is not a UIWindowScene.
        guard let windowScene = scene as? UIWindowScene else { return }

        if !connectionOptions.urlContexts.isEmpty {
            for context in connectionOptions.urlContexts {
                DeepLinkHandler.handle(url: context.url, source: "launch")
            }
        }
        if !connectionOptions.userActivities.isEmpty {
            for activity in connectionOptions.userActivities {
                if let url = activity.webpageURL {
                    DeepLinkHandler.handle(url: url, source: "launch")
                }
            }
        }

        // Create a new window attached to this window scene.
        // The window will fill the entire screen.
        let window = UIWindow(windowScene: windowScene)

        // Create the Drift view controller as the root.
        // This controller manages the Metal view and display link.
        window.rootViewController = DriftViewController()

        // Make the window visible and key (receives events).
        // This must be called for the window to appear on screen.
        window.makeKeyAndVisible()

        // Store the window reference to keep it alive.
        self.window = window
    }

    func scene(_ scene: UIScene, openURLContexts URLContexts: Set<UIOpenURLContext>) {
        for context in URLContexts {
            DeepLinkHandler.handle(url: context.url, source: "open_url")
        }
    }

    func scene(_ scene: UIScene, continue userActivity: NSUserActivity) {
        if let url = userActivity.webpageURL {
            DeepLinkHandler.handle(url: url, source: "user_activity")
        }
    }

    /// Called when the scene has moved to the foreground and is active.
    ///
    /// The app is visible and receiving events. This is where rendering
    /// should be at full speed.
    func sceneDidBecomeActive(_ scene: UIScene) {
        LifecycleHandler.notifyStateChange("resumed")
    }

    /// Called when the scene is about to move from the active state.
    ///
    /// This may be due to a system interruption (e.g., phone call) or
    /// the user switching to another app. Rendering can continue but
    /// the app isn't receiving user input.
    func sceneWillResignActive(_ scene: UIScene) {
        LifecycleHandler.notifyStateChange("inactive")
    }

    /// Called when the scene has moved to the background.
    ///
    /// The app is no longer visible. This is a good time to save state
    /// and reduce resource usage.
    func sceneDidEnterBackground(_ scene: UIScene) {
        LifecycleHandler.notifyStateChange("paused")
    }

    /// Called when the scene is about to enter the foreground.
    ///
    /// The app is about to become visible again. Prepare to resume
    /// normal operation.
    func sceneWillEnterForeground(_ scene: UIScene) {
        LifecycleHandler.notifyStateChange("inactive")
    }
}
