/// DriftApp.swift
/// SwiftUI App entry point that wraps the UIKit-based Drift application.
///
/// xtool expects a SwiftUI App with @main attribute. This wrapper hosts
/// the UIKit view controller hierarchy within a SwiftUI lifecycle.

import SwiftUI
import UIKit

@main
struct DriftApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) var appDelegate

    var body: some Scene {
        WindowGroup {
            DriftViewControllerRepresentable()
                .ignoresSafeArea()
                .onOpenURL { url in
                    // Handle custom URL schemes and universal links
                    DeepLinkHandler.handle(url: url, source: "open_url")
                }
                .onContinueUserActivity(NSUserActivityTypeBrowsingWeb) { activity in
                    // Handle universal links via user activity
                    if let url = activity.webpageURL {
                        DeepLinkHandler.handle(url: url, source: "user_activity")
                    }
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
}
