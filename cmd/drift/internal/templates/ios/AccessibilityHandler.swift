/// AccessibilityHandler.swift
/// Handles accessibility platform channel messages for Drift.

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

    /// Handles platform channel method calls.
    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "updateSemantics":
            return shared.updateSemantics(args: args)
        case "announce":
            return shared.announce(args: args)
        case "setAccessibilityFocus":
            return shared.setAccessibilityFocus(args: args)
        case "clearAccessibilityFocus":
            return shared.clearAccessibilityFocus()
        case "isAccessibilityEnabled":
            return shared.isAccessibilityEnabled()
        default:
            return (nil, NSError(
                domain: "AccessibilityHandler",
                code: 404,
                userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]
            ))
        }
    }

    // MARK: - Method Handlers

    private func updateSemantics(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any] else {
            return (nil, NSError(
                domain: "AccessibilityHandler",
                code: 400,
                userInfo: [NSLocalizedDescriptionKey: "Invalid arguments"]
            ))
        }

        let updates = dict["updates"] as? [[String: Any]] ?? []
        let removals: [Int64] = (dict["removals"] as? [Any])?.compactMap { value in
            if let num = value as? NSNumber {
                return num.int64Value
            }
            return nil
        } ?? []

        bridge?.updateSemantics(updates: updates, removals: removals)
        return (nil, nil)
    }

    private func announce(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let message = dict["message"] as? String else {
            return (nil, NSError(
                domain: "AccessibilityHandler",
                code: 400,
                userInfo: [NSLocalizedDescriptionKey: "Missing message"]
            ))
        }

        let politeness = dict["politeness"] as? String ?? "polite"
        bridge?.announce(message: message, politeness: politeness)
        return (nil, nil)
    }

    private func setAccessibilityFocus(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let nodeId = (dict["nodeId"] as? NSNumber)?.int64Value else {
            return (nil, NSError(
                domain: "AccessibilityHandler",
                code: 400,
                userInfo: [NSLocalizedDescriptionKey: "Missing nodeId"]
            ))
        }

        bridge?.setAccessibilityFocus(nodeId: nodeId)
        return (nil, nil)
    }

    private func clearAccessibilityFocus() -> (Any?, Error?) {
        bridge?.clearAccessibilityFocus()
        return (nil, nil)
    }

    private func isAccessibilityEnabled() -> (Any?, Error?) {
        let enabled = UIAccessibility.isVoiceOverRunning ||
                      UIAccessibility.isSwitchControlRunning ||
                      UIAccessibility.isSpeakScreenEnabled
        return (["enabled": enabled], nil)
    }

    // MARK: - Notifications

    @objc private func voiceOverStatusChanged() {
        notifyAccessibilityState()
    }

    private func notifyAccessibilityState() {
        let enabled = UIAccessibility.isVoiceOverRunning ||
                      UIAccessibility.isSwitchControlRunning ||
                      UIAccessibility.isSpeakScreenEnabled

        PlatformChannelManager.shared.sendEvent(
            channel: "drift/accessibility/state",
            data: ["enabled": enabled]
        )
    }
}
