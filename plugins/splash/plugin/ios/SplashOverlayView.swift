/// SplashOverlayView.swift
///
/// Full-screen UIView that mirrors the LaunchScreen.storyboard layout so the
/// runtime overlay can attach with no visual seam after the system splash
/// tears down. Auto-layout constraints keep the image and optional branding
/// pinned through rotation.
///
/// `fadeOut(durationMs:completion:)` runs on the main thread; callers must
/// hop to main before invoking.

import UIKit

final class DriftSplashOverlayView: UIView {

    private let imageView = UIImageView()
    private var brandingView: UIImageView?
    private let backgroundLayerView = UIView()

    init() {
        super.init(frame: .zero)
        translatesAutoresizingMaskIntoConstraints = false

        backgroundLayerView.translatesAutoresizingMaskIntoConstraints = false
        backgroundLayerView.backgroundColor = parseHex(DriftSplashConfig.backgroundColor)
        addSubview(backgroundLayerView)

        imageView.translatesAutoresizingMaskIntoConstraints = false
        imageView.contentMode = .scaleAspectFit
        imageView.image = UIImage(named: "DriftSplash")
        addSubview(imageView)

        NSLayoutConstraint.activate([
            backgroundLayerView.topAnchor.constraint(equalTo: topAnchor),
            backgroundLayerView.bottomAnchor.constraint(equalTo: bottomAnchor),
            backgroundLayerView.leadingAnchor.constraint(equalTo: leadingAnchor),
            backgroundLayerView.trailingAnchor.constraint(equalTo: trailingAnchor),

            imageView.centerXAnchor.constraint(equalTo: centerXAnchor),
            imageView.centerYAnchor.constraint(equalTo: centerYAnchor),
            imageView.widthAnchor.constraint(lessThanOrEqualTo: widthAnchor, multiplier: 0.6),
            imageView.heightAnchor.constraint(lessThanOrEqualTo: heightAnchor, multiplier: 0.6),
        ])

        installBrandingIfConfigured()

        // Observe dark-mode changes so the image and background swap live
        // while the splash is up.
        applyTraitColours()
    }

    required init?(coder: NSCoder) { fatalError("not supported") }

    private func installBrandingIfConfigured() {
        guard UIImage(named: "DriftSplashBranding") != nil else { return }
        let view = UIImageView(image: UIImage(named: "DriftSplashBranding"))
        view.translatesAutoresizingMaskIntoConstraints = false
        view.contentMode = .scaleAspectFit
        addSubview(view)
        brandingView = view

        let bottom = view.bottomAnchor.constraint(equalTo: safeAreaLayoutGuide.bottomAnchor, constant: -24)
        let height = view.heightAnchor.constraint(lessThanOrEqualToConstant: 80)
        NSLayoutConstraint.activate([bottom, height])

        switch DriftSplashConfig.brandingPosition {
        case "bottom_left":
            view.leadingAnchor.constraint(equalTo: safeAreaLayoutGuide.leadingAnchor, constant: 24).isActive = true
        case "bottom_right":
            view.trailingAnchor.constraint(equalTo: safeAreaLayoutGuide.trailingAnchor, constant: -24).isActive = true
        default:
            view.centerXAnchor.constraint(equalTo: centerXAnchor).isActive = true
        }
    }

    override func traitCollectionDidChange(_ previousTraitCollection: UITraitCollection?) {
        super.traitCollectionDidChange(previousTraitCollection)
        applyTraitColours()
    }

    private func applyTraitColours() {
        // Image assets named in the Assets.xcassets ship as a light/dark
        // pair under the same name when the user supplies a `dark:` variant;
        // UIImage(named:) handles the appearance match automatically.
        // Background colour is the only thing the runtime resolves itself.
        let bgHex = traitCollection.userInterfaceStyle == .dark && !DriftSplashConfig.darkBackgroundColor.isEmpty
            ? DriftSplashConfig.darkBackgroundColor
            : DriftSplashConfig.backgroundColor
        backgroundLayerView.backgroundColor = parseHex(bgHex)
    }

    func fadeOut(durationMs: Int, completion: @escaping () -> Void) {
        UIView.animate(
            withDuration: TimeInterval(durationMs) / 1000.0,
            delay: 0,
            options: [.curveEaseOut],
            animations: { self.alpha = 0 },
            completion: { _ in
                self.removeFromSuperview()
                completion()
            }
        )
    }

    private func parseHex(_ s: String) -> UIColor {
        // Accepts #RRGGBB and #RRGGBBAA. Validated at build time, so a
        // malformed input here means the build-time validator was bypassed.
        let trimmed = s.hasPrefix("#") ? String(s.dropFirst()) : s
        var value: UInt64 = 0
        guard Scanner(string: trimmed).scanHexInt64(&value) else { return .white }
        let r, g, b, a: CGFloat
        switch trimmed.count {
        case 6:
            r = CGFloat((value & 0xFF0000) >> 16) / 255
            g = CGFloat((value & 0x00FF00) >> 8) / 255
            b = CGFloat(value & 0x0000FF) / 255
            a = 1
        case 8:
            r = CGFloat((value & 0xFF000000) >> 24) / 255
            g = CGFloat((value & 0x00FF0000) >> 16) / 255
            b = CGFloat((value & 0x0000FF00) >> 8) / 255
            a = CGFloat(value & 0x000000FF) / 255
        default:
            return .white
        }
        return UIColor(red: r, green: g, blue: b, alpha: a)
    }
}
