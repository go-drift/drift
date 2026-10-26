/// AccessibilityHandler.swift
/// Handles accessibility platform channel messages for Drift.

import DriftPluginAPI
import UIKit

final class AccessibilityHandler {
    static let shared = AccessibilityHandler()

    private var bridge: AccessibilityBridge?

    private init() {
        // Listen for VoiceOver state changes
        NotificationCenter.default.addObserver(
            self,
            selector: #selector(voiceOverStatusChanged),
            name: UIAccessibility.voiceOverStatusDidChangeNotification,
            object: nil
        )
    }

    deinit {
        NotificationCenter.default.removeObserver(self)
    }

    /// Initializes the accessibility handler with the host view.
    func initialize(hostView: UIView) {
        bridge = AccessibilityBridge(hostView: hostView)

        // Set up the accessibility elements provider on the Metal view
        if let metalView = hostView as? DriftMetalView {
            metalView.accessibilityElementsProvider = { [weak self] in
                self?.bridge?.accessibilityElements()
            }
        }

        // Notify Go side of initial accessibility state
        notifyAccessibilityState()
    }

    /// Registers the drift/accessibility channel.
    static func start(host: DriftPluginHost) {
        host.registerChannel("drift/accessibility") { channel in
            channel.method("updateSemantics") { args in try shared.updateSemantics(args: args) }
            channel.method("announce") { args in try shared.announce(args: args) }
            channel.method("setAccessibilityFocus") { args in try shared.setAccessibilityFocus(args: args) }
            channel.method("clearAccessibilityFocus") { _ in
                shared.bridge?.clearAccessibilityFocus()
                return nil
            }
            channel.method("isAccessibilityEnabled") { _ in ["enabled": isEnabled] }
        }
    }

    // MARK: - Method Handlers

    private func updateSemantics(args: Any?) throws -> Any? {
        guard let dict = args as? [String: Any] else {
            throw DriftCallError.invalidArguments("Invalid arguments")
        }

        let updates = dict["updates"] as? [[String: Any]] ?? []
        let removals: [Int64] = (dict["removals"] as? [Any])?.compactMap { value in
            if let num = value as? NSNumber {
                return num.int64Value
            }
            return nil
        } ?? []

        bridge?.updateSemantics(updates: updates, removals: removals)
        return nil
    }

    private func announce(args: Any?) throws -> Any? {
        guard let dict = args as? [String: Any],
              let message = dict["message"] as? String else {
            throw DriftCallError.invalidArguments("Missing message")
        }

        let politeness = dict["politeness"] as? String ?? "polite"
        bridge?.announce(message: message, politeness: politeness)
        return nil
    }

    private func setAccessibilityFocus(args: Any?) throws -> Any? {
        guard let dict = args as? [String: Any],
              let nodeId = (dict["nodeId"] as? NSNumber)?.int64Value else {
            throw DriftCallError.invalidArguments("Missing nodeId")
        }

        bridge?.setAccessibilityFocus(nodeId: nodeId)
        return nil
    }

    private static var isEnabled: Bool {
        UIAccessibility.isVoiceOverRunning ||
            UIAccessibility.isSwitchControlRunning ||
            UIAccessibility.isSpeakScreenEnabled
    }

    // MARK: - Notifications

    @objc private func voiceOverStatusChanged() {
        notifyAccessibilityState()
    }

    private func notifyAccessibilityState() {
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/accessibility/state",
            data: ["enabled": Self.isEnabled]
        )
    }
}
