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
                .onOpenURL { url in
                    DeepLinkHandler.handle(url: url, source: "open_url")
                }
                .onContinueUserActivity(NSUserActivityTypeBrowsingWeb) { activity in
                    if let url = activity.webpageURL {
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
}
