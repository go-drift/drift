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
///   - sceneDidDisconnect: Scene released by the system (or closed by the
///     user); stops rendering and detaches plugins, then drops the window

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

        // URLs and activities that launched the app: plugins first, then
        // Drift's deep links.
        for context in connectionOptions.urlContexts {
            openURL(context.url, source: "launch")
        }
        for activity in connectionOptions.userActivities {
            continueUserActivity(activity, source: "launch")
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

    /// URLs opened while the app is running. In a scene-based app these
    /// arrive here, not at UIApplicationDelegate.application(_:open:options:).
    func scene(_ scene: UIScene, openURLContexts URLContexts: Set<UIOpenURLContext>) {
        for context in URLContexts {
            openURL(context.url, source: "open_url")
        }
    }

    /// User activities (universal links, Handoff) continued while the app is
    /// running. Scene-based apps receive these here, not at the app delegate.
    func scene(_ scene: UIScene, continue userActivity: NSUserActivity) {
        continueUserActivity(userActivity, source: "user_activity")
    }

    /// Offers url to plugins; Drift's deep links get it if none claims it.
    private func openURL(_ url: URL, source: String) {
        if !DriftPlugins.shared.openURL(url) {
            DeepLinkHandler.handle(url: url, source: source)
        }
    }

    /// Offers activity to plugins; Drift's deep links get its web URL if
    /// none claims it.
    private func continueUserActivity(_ activity: NSUserActivity, source: String) {
        if !DriftPlugins.shared.continueUserActivity(activity), let url = activity.webpageURL {
            DeepLinkHandler.handle(url: url, source: source)
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

    /// Called when the system releases the scene: closed in the App
    /// Switcher, or discarded in the background to free memory. The
    /// process may live on (and connect a new scene later), so plugins are
    /// detached here rather than waiting for the controller's deinit.
    func sceneDidDisconnect(_ scene: UIScene) {
        (window?.rootViewController as? DriftViewController)?.tearDown()
        window = nil
    }
}
