/// DriftImagePickerPlugin.swift
///
/// The image picker plugin's native half on iOS, on the main thread:
/// UIImagePickerController for the camera, PHPickerViewController for the
/// library (no permission needed), presented over the Drift view. Images
/// are written as JPEG to the caches directory. One picker at a time.
///
/// Errors Go maps to its sentinels (runtime package): `busy`,
/// `camera_unavailable`, `camera_not_configured`, `permission_denied`.

import AVFoundation
import DriftPluginAPI
import PhotosUI
import UIKit

/// Public, with a public initializer: the app's generated registrant
/// creates it from outside this module (DriftPlugin_imagepicker).
public final class DriftImagePickerPlugin: NSObject, DriftPlugin {
    private static let channel = "drift/imagepicker"

    private var viewController: UIViewController?
    /// The open picker's reply, nil when none is open.
    private var pending: DriftResult?

    public override init() {
        super.init()
    }

    public func register(host: DriftPluginHost) {
        host.registerChannel(Self.channel) { channel in
            channel.asyncMethod("capture") { [self] args, result in
                let front = (args as? [String: Any])?["frontCamera"] as? Bool ?? false
                begin(result) { try capture(front: front) }
            }
            channel.asyncMethod("pick") { [self] args, result in
                let multiple = (args as? [String: Any])?["multiple"] as? Bool ?? false
                begin(result) { try pick(multiple: multiple) }
            }
        }
    }

    public func attach(_ binding: DriftViewBinding) {
        viewController = binding.viewController
    }

    public func detach() {
        viewController = nil
    }

    /// Opens a picker for result, or fails it.
    private func begin(_ result: DriftResult, _ open: () throws -> UIViewController) {
        guard pending == nil else {
            result.error(DriftCallError("busy", "an image picker is already open"))
            return
        }
        do {
            guard let host = viewController else {
                throw DriftCallError("no_activity", "no Drift view to present the picker over")
            }
            let picker = try open()
            pending = result
            var top = host
            while let presented = top.presentedViewController { top = presented }
            top.present(picker, animated: true)
        } catch {
            result.error(error)
        }
    }

    private func capture(front: Bool) throws -> UIViewController {
        guard Bundle.main.object(forInfoDictionaryKey: "NSCameraUsageDescription") != nil else {
            throw DriftCallError("camera_not_configured", "add camera: to the imagepicker plugin config to take photos")
        }
        guard UIImagePickerController.isSourceTypeAvailable(.camera) else {
            throw DriftCallError("camera_unavailable", "this device has no camera")
        }
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .denied, .restricted:
            throw DriftCallError("permission_denied", "camera access denied; the user can allow it in Settings")
        default:
            // Not asked yet: the picker asks.
            break
        }
        let picker = UIImagePickerController()
        picker.sourceType = .camera
        if front, UIImagePickerController.isCameraDeviceAvailable(.front) {
            picker.cameraDevice = .front
        }
        picker.delegate = self
        return picker
    }

    private func pick(multiple: Bool) throws -> UIViewController {
        var config = PHPickerConfiguration()
        config.selectionLimit = multiple ? 0 : 1
        config.filter = .images
        let picker = PHPickerViewController(configuration: config)
        picker.delegate = self
        return picker
    }

    private func finish(_ reply: (DriftResult) -> Void) {
        guard let result = pending else { return }
        pending = nil
        reply(result)
    }

    // MARK: - Files

    /// Writes image as JPEG to the caches directory and describes it. The
    /// JPEG keeps the photo's pixels as stored, with its EXIF orientation,
    /// which Drift's image.Decode applies; the size is as displayed (UIImage
    /// size accounts for orientation), as on Android.
    private static func save(_ image: UIImage) throws -> [String: Any] {
        guard let data = image.jpegData(compressionQuality: 0.9) else {
            throw DriftCallError("native_error", "could not encode the image as JPEG")
        }
        let caches = try FileManager.default.url(for: .cachesDirectory, in: .userDomainMask, appropriateFor: nil, create: true)
        let url = caches.appendingPathComponent("drift_image_\(UUID().uuidString).jpg")
        try data.write(to: url)
        return [
            "path": url.path,
            "mimeType": "image/jpeg",
            "width": Int(image.size.width * image.scale),
            "height": Int(image.size.height * image.scale),
            "size": data.count,
        ]
    }
}

// MARK: - Camera

extension DriftImagePickerPlugin: UIImagePickerControllerDelegate, UINavigationControllerDelegate {
    public func imagePickerController(
        _ picker: UIImagePickerController,
        didFinishPickingMediaWithInfo info: [UIImagePickerController.InfoKey: Any]
    ) {
        let image = info[.originalImage] as? UIImage
        picker.dismiss(animated: true) { [self] in
            finish { result in
                guard let image else {
                    result.success(["image": NSNull()])
                    return
                }
                do {
                    result.success(["image": try Self.save(image)])
                } catch {
                    result.error(error)
                }
            }
        }
    }

    public func imagePickerControllerDidCancel(_ picker: UIImagePickerController) {
        picker.dismiss(animated: true) { [self] in
            finish { $0.success(["image": NSNull()]) }
        }
    }
}

// MARK: - Library

extension DriftImagePickerPlugin: PHPickerViewControllerDelegate {
    public func picker(_ picker: PHPickerViewController, didFinishPicking results: [PHPickerResult]) {
        picker.dismiss(animated: true)
        let providers = results.map(\.itemProvider).filter { $0.canLoadObject(ofClass: UIImage.self) }
        var images = [[String: Any]?](repeating: nil, count: providers.count)
        var failure: Error?
        let group = DispatchGroup()
        for (i, provider) in providers.enumerated() {
            group.enter()
            provider.loadObject(ofClass: UIImage.self) { object, error in
                DispatchQueue.main.async {
                    defer { group.leave() }
                    do {
                        guard let image = object as? UIImage else {
                            throw error ?? DriftCallError("native_error", "could not load the picked image")
                        }
                        images[i] = try Self.save(image)
                    } catch {
                        failure = failure ?? error
                    }
                }
            }
        }
        group.notify(queue: .main) { [self] in
            finish { result in
                if let failure {
                    result.error(failure)
                } else {
                    result.success(["images": images.compactMap { $0 }])
                }
            }
        }
    }
}
