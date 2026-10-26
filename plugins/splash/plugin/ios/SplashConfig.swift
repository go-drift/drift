/// SplashConfig.swift
///
/// The splash config the build half records (codegen.go nativeConfig),
/// read from host.config. LaunchScreen.storyboard comes from the same
/// values, so the overlay matches it.

import DriftPluginAPI
import UIKit

struct SplashConfig {
    let backgroundColor: UIColor
    /// The image's on-screen size in points, as in LaunchScreen.storyboard.
    let imageSize: CGSize
    let fadeDurationMs: Int
    let maxDurationMs: Int
    let ceilingMs: Int

    /// Parses host.config; a config the build half did not write is a bug.
    init(_ config: [String: Any]) {
        func number(_ key: String) -> NSNumber {
            guard let value = config[key] as? NSNumber else {
                fatalError("drift splash: config has no \(key): \(config)")
            }
            return value
        }
        guard let rgba = config["backgroundColor"] as? [NSNumber], rgba.count == 4 else {
            fatalError("drift splash: config has no backgroundColor: \(config)")
        }
        backgroundColor = UIColor(
            red: CGFloat(rgba[0].doubleValue), green: CGFloat(rgba[1].doubleValue),
            blue: CGFloat(rgba[2].doubleValue), alpha: CGFloat(rgba[3].doubleValue)
        )
        imageSize = CGSize(width: number("imageWidth").doubleValue, height: number("imageHeight").doubleValue)
        fadeDurationMs = number("fadeDurationMs").intValue
        maxDurationMs = number("maxDurationMs").intValue
        ceilingMs = number("ceilingMs").intValue
    }
}
