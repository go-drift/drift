/// NativeActivityIndicator.swift
/// Provides native UIActivityIndicatorView embedded in Drift UI.

import UIKit

// MARK: - Native Activity Indicator Container

/// Platform view container for native activity indicator.
class NativeActivityIndicatorContainer: NSObject, PlatformViewContainer {
    let viewId: Int
    let view: UIView
    private let indicator: UIActivityIndicatorView

    init(viewId: Int, params: [String: Any]) {
        self.viewId = viewId

        // Determine style based on size parameter
        let sizeParam = params["size"] as? Int ?? 0 // Default to medium
        let style: UIActivityIndicatorView.Style
        if #available(iOS 13.0, *) {
            switch sizeParam {
            case 1: // Small
                style = .medium
            case 2: // Large
                style = .large
            default: // Medium (0)
                style = .medium
            }
        } else {
            switch sizeParam {
            case 1: // Small
                style = .white
            case 2: // Large
                style = .whiteLarge
            default: // Medium (0)
                style = .white
            }
        }

        self.indicator = UIActivityIndicatorView(style: style)
        self.view = indicator

        super.init()

        // Apply color if provided (arrives as NSNumber from JSON/MessagePack)
        if let colorNumber = params["color"] as? NSNumber {
            let colorValue = colorNumber.uint32Value
            if colorValue != 0 {
                indicator.color = UIColor(argb: colorValue)
            }
        }

        // Start animating if requested (default: true)
        let animating = params["animating"] as? Bool ?? true
        if animating {
            indicator.startAnimating()
        }

        // Don't hide when stopped - we control visibility separately
        indicator.hidesWhenStopped = false
    }

    func dispose() {
        indicator.stopAnimating()
        view.removeFromSuperview()
    }

    func setAnimating(_ animating: Bool) {
        if animating {
            indicator.startAnimating()
        } else {
            indicator.stopAnimating()
        }
    }

    func updateConfig(_ params: [String: Any]) {
        // Update color (arrives as NSNumber from JSON/MessagePack)
        if let colorNumber = params["color"] as? NSNumber {
            let colorValue = colorNumber.uint32Value
            if colorValue != 0 {
                indicator.color = UIColor(argb: colorValue)
            }
        }

        // Update animating state
        if let animating = params["animating"] as? Bool {
            if animating {
                indicator.startAnimating()
            } else {
                indicator.stopAnimating()
            }
        }

        // Note: Changing size requires recreating the indicator, which we don't support
        // for simplicity. Size should be set at creation time.
    }
}
