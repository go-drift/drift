/// NativeSwitch.swift
/// Provides native UISwitch embedded in Drift UI.

import DriftPluginAPI
import UIKit

// MARK: - Native Switch Container

/// Platform view container for native switch.
class NativeSwitchContainer: NSObject, DriftPlatformView {
    let viewId: Int
    let view: UIView
    private let switchControl: UISwitch

    init(viewId: Int, params: [String: Any]) {
        self.viewId = viewId
        self.switchControl = UISwitch()
        self.view = switchControl

        super.init()

        // Apply styling
        if let onTint = params["onTintColor"] as? UInt32 {
            switchControl.onTintColor = UIColor(argb: onTint)
        }
        if let thumbTint = params["thumbTintColor"] as? UInt32 {
            switchControl.thumbTintColor = UIColor(argb: thumbTint)
        }

        // Set initial value
        if let value = params["value"] as? Bool {
            switchControl.isOn = value
        }

        // Add target for value changes
        switchControl.addTarget(self, action: #selector(valueChanged), for: .valueChanged)
    }

    func dispose() {
        switchControl.removeTarget(self, action: #selector(valueChanged), for: .valueChanged)
        view.removeFromSuperview()
    }

    @objc private func valueChanged() {
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/platform_views",
            data: [
                "method": "onSwitchChanged",
                "viewId": viewId,
                "value": switchControl.isOn
            ]
        )
    }

    func setValue(_ value: Bool) {
        switchControl.setOn(value, animated: true)
    }

    func updateConfig(_ params: [String: Any]) {
        if let onTint = params["onTintColor"] as? UInt32 {
            switchControl.onTintColor = UIColor(argb: onTint)
        }
        if let thumbTint = params["thumbTintColor"] as? UInt32 {
            switchControl.thumbTintColor = UIColor(argb: thumbTint)
        }
    }
}

extension NativeSwitchContainer {
    func handle(_ method: String, _ args: [String: Any]) throws {
        switch method {
        case "setValue":
            if let value = args["value"] as? Bool {
                setValue(value)
            }
        case "updateConfig": updateConfig(args)
        default: throw unknownViewMethod(method)
        }
    }
}
