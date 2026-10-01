/// SplashOverlayView.swift
///
/// Full-screen UIView that mirrors the LaunchScreen.storyboard layout so the
/// runtime overlay can attach with no visual seam after the system splash
/// tears down. Auto-layout constraints keep the image pinned through
/// rotation.
///
/// `fadeOut(durationMs:completion:)` runs on the main thread; callers must
/// hop to main before invoking.

import UIKit

final class DriftSplashOverlayView: UIView {

    private let imageView = UIImageView()
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
    }

    required init?(coder: NSCoder) { fatalError("not supported") }

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
