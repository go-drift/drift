/// DriftApp.swift
/// SwiftUI App entry point for xtool compatibility.
///
/// xtool expects a SwiftUI App with @main attribute. This wrapper hosts
/// the UIKit view controller hierarchy within a SwiftUI lifecycle.

import SwiftUI
import UIKit

@main
struct DriftApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) var appDelegate
    @Environment(\.scenePhase) private var scenePhase

    var body: some Scene {
        WindowGroup {
            DriftViewControllerRepresentable()
                .ignoresSafeArea()
                // Plugins first, then Drift's deep links. SwiftUI delivers
                // only the activity types asked for: universal links.
                .onOpenURL { url in
                    if !DriftPlugins.shared.openURL(url) {
                        DeepLinkHandler.handle(url: url, source: "open_url")
                    }
                }
                .onContinueUserActivity(NSUserActivityTypeBrowsingWeb) { activity in
                    if !DriftPlugins.shared.continueUserActivity(activity), let url = activity.webpageURL {
                        DeepLinkHandler.handle(url: url, source: "user_activity")
                    }
                }
        }
        .onChange(of: scenePhase) { newPhase in
            switch newPhase {
            case .active:
                LifecycleHandler.notifyStateChange("resumed")
            case .inactive:
                LifecycleHandler.notifyStateChange("inactive")
            case .background:
                LifecycleHandler.notifyStateChange("paused")
            @unknown default:
                break
            }
        }
    }
}

/// UIViewControllerRepresentable that wraps the main DriftViewController
struct DriftViewControllerRepresentable: UIViewControllerRepresentable {
    func makeUIViewController(context: Context) -> DriftViewController {
        return DriftViewController()
    }

    func updateUIViewController(_ uiViewController: DriftViewController, context: Context) {
        // No updates needed
    }

    /// SwiftUI removes the controller when its scene goes away: stop
    /// rendering and detach plugins (SceneDelegate.sceneDidDisconnect on
    /// the xcodeproj path).
    static func dismantleUIViewController(_ uiViewController: DriftViewController, coordinator: ()) {
        uiViewController.tearDown()
    }
}
