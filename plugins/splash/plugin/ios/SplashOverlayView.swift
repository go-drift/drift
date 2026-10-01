/// SplashOverlayView.swift
///
/// Full-screen view laid out exactly like LaunchScreen.storyboard (both come
/// from the same SplashConfig values), so it takes over from the launch
/// screen with no visible seam. Main thread only.

import UIKit

final class DriftSplashOverlayView: UIView {

    init() {
        super.init(frame: .zero)
        backgroundColor = DriftSplashConfig.backgroundColor

        guard let image = UIImage(named: "DriftSplash") else {
            preconditionFailure("drift splash: DriftSplash image missing from the app bundle")
        }
        let imageView = UIImageView(image: image)
        imageView.contentMode = .scaleAspectFit
        imageView.translatesAutoresizingMaskIntoConstraints = false
        addSubview(imageView)

        let size = DriftSplashConfig.imageSize
        NSLayoutConstraint.activate([
            imageView.centerXAnchor.constraint(equalTo: centerXAnchor),
            imageView.centerYAnchor.constraint(equalTo: centerYAnchor),
            imageView.widthAnchor.constraint(equalToConstant: size.width),
            imageView.heightAnchor.constraint(equalToConstant: size.height),
        ])
    }

    required init?(coder: NSCoder) { fatalError("not supported") }

    /// Adds the overlay to host, filling it.
    func install(in host: UIView) {
        translatesAutoresizingMaskIntoConstraints = false
        host.addSubview(self)
        NSLayoutConstraint.activate([
            topAnchor.constraint(equalTo: host.topAnchor),
            bottomAnchor.constraint(equalTo: host.bottomAnchor),
            leadingAnchor.constraint(equalTo: host.leadingAnchor),
            trailingAnchor.constraint(equalTo: host.trailingAnchor),
        ])
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
}
