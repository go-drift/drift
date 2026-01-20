/// CameraHandler.swift
/// Handles camera capture and gallery picking for the Drift platform channel.

import UIKit
import Photos
import PhotosUI
import AVFoundation

final class CameraHandler: NSObject {
    static let shared = CameraHandler()

    private override init() {
        super.init()
    }

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "capturePhoto":
            return shared.capturePhoto(args: args)
        case "captureVideo":
            return shared.captureVideo(args: args)
        case "pickFromGallery":
            return shared.pickFromGallery(args: args)
        default:
            return (nil, NSError(domain: "Camera", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    private func capturePhoto(args: Any?) -> (Any?, Error?) {
        let dict = args as? [String: Any] ?? [:]
        let useFrontCamera = dict["useFrontCamera"] as? Bool ?? false

        DispatchQueue.main.async {
            self.presentImagePicker(
                sourceType: .camera,
                mediaTypes: ["public.image"],
                cameraDevice: useFrontCamera ? .front : .rear
            )
        }

        // Result will be delivered via drift/camera/result event channel
        return (["pending": true], nil)
    }

    private func captureVideo(args: Any?) -> (Any?, Error?) {
        let dict = args as? [String: Any] ?? [:]
        let useFrontCamera = dict["useFrontCamera"] as? Bool ?? false
        let maxDurationMs = dict["maxDurationMs"] as? Int64

        DispatchQueue.main.async {
            self.presentImagePicker(
                sourceType: .camera,
                mediaTypes: ["public.movie"],
                cameraDevice: useFrontCamera ? .front : .rear,
                maxDuration: maxDurationMs.map { TimeInterval($0) / 1000.0 }
            )
        }

        // Result will be delivered via drift/camera/result event channel
        return (["pending": true], nil)
    }

    private func pickFromGallery(args: Any?) -> (Any?, Error?) {
        let dict = args as? [String: Any] ?? [:]
        let allowMultiple = dict["allowMultiple"] as? Bool ?? false
        let mediaType = dict["mediaType"] as? String ?? "all"
        let maxSelection = dict["maxSelection"] as? Int ?? (allowMultiple ? 10 : 1)

        DispatchQueue.main.async {
            self.presentPhotoPicker(
                allowMultiple: allowMultiple,
                mediaType: mediaType,
                maxSelection: maxSelection
            )
        }

        // Result will be delivered via drift/camera/result event channel
        return (["pending": true], nil)
    }

    private func presentImagePicker(
        sourceType: UIImagePickerController.SourceType,
        mediaTypes: [String],
        cameraDevice: UIImagePickerController.CameraDevice,
        maxDuration: TimeInterval? = nil
    ) {
        guard UIImagePickerController.isSourceTypeAvailable(sourceType) else {
            sendCancelled("capture")
            return
        }

        let picker = UIImagePickerController()
        picker.sourceType = sourceType
        picker.mediaTypes = mediaTypes
        picker.delegate = self

        if sourceType == .camera {
            picker.cameraDevice = cameraDevice
            if let maxDuration = maxDuration {
                picker.videoMaximumDuration = maxDuration
            }
        }

        presentPicker(picker)
    }

    private func presentPhotoPicker(allowMultiple: Bool, mediaType: String, maxSelection: Int) {
        var config = PHPickerConfiguration(photoLibrary: .shared())
        config.selectionLimit = allowMultiple ? maxSelection : 1

        switch mediaType {
        case "image":
            config.filter = .images
        case "video":
            config.filter = .videos
        default:
            config.filter = .any(of: [.images, .videos])
        }

        let picker = PHPickerViewController(configuration: config)
        picker.delegate = self
        presentPicker(picker)
    }

    private func presentPicker(_ picker: UIViewController) {
        if let windowScene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
           let rootVC = windowScene.windows.first?.rootViewController {
            var topVC = rootVC
            while let presented = topVC.presentedViewController {
                topVC = presented
            }
            topVC.present(picker, animated: true)
        }
    }

    private func getMediaInfo(url: URL) -> [String: Any] {
        var info: [String: Any] = [
            "path": url.path,
            "mimeType": mimeType(for: url),
            "size": fileSize(at: url)
        ]

        if let imageSource = CGImageSourceCreateWithURL(url as CFURL, nil),
           let properties = CGImageSourceCopyPropertiesAtIndex(imageSource, 0, nil) as? [String: Any] {
            info["width"] = properties[kCGImagePropertyPixelWidth as String] as? Int ?? 0
            info["height"] = properties[kCGImagePropertyPixelHeight as String] as? Int ?? 0
            info["durationMs"] = 0
        } else {
            let asset = AVAsset(url: url)
            if let track = asset.tracks(withMediaType: .video).first {
                let size = track.naturalSize.applying(track.preferredTransform)
                info["width"] = Int(abs(size.width))
                info["height"] = Int(abs(size.height))
                info["durationMs"] = Int64(CMTimeGetSeconds(asset.duration) * 1000)
            }
        }

        return info
    }

    private func mimeType(for url: URL) -> String {
        let ext = url.pathExtension.lowercased()
        switch ext {
        case "jpg", "jpeg":
            return "image/jpeg"
        case "png":
            return "image/png"
        case "gif":
            return "image/gif"
        case "heic":
            return "image/heic"
        case "mov":
            return "video/quicktime"
        case "mp4":
            return "video/mp4"
        default:
            return "application/octet-stream"
        }
    }

    private func fileSize(at url: URL) -> Int64 {
        do {
            let attrs = try FileManager.default.attributesOfItem(atPath: url.path)
            return attrs[.size] as? Int64 ?? 0
        } catch {
            return 0
        }
    }

    // MARK: - Event Sending

    private func sendCaptureResult(_ result: [String: Any]) {
        var payload = result
        payload["type"] = "capture"
        PlatformChannelManager.shared.sendEvent(channel: "drift/camera/result", data: payload)
    }

    private func sendGalleryResult(_ mediaList: [[String: Any]]) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/camera/result", data: [
            "type": "gallery",
            "media": mediaList
        ])
    }

    private func sendCancelled(_ requestType: String) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/camera/result", data: [
            "type": requestType,
            "cancelled": true
        ])
    }
}

// MARK: - UIImagePickerControllerDelegate

extension CameraHandler: UIImagePickerControllerDelegate, UINavigationControllerDelegate {
    func imagePickerController(_ picker: UIImagePickerController, didFinishPickingMediaWithInfo info: [UIImagePickerController.InfoKey: Any]) {
        picker.dismiss(animated: true)

        if let mediaURL = info[.mediaURL] as? URL {
            // Video
            sendCaptureResult(getMediaInfo(url: mediaURL))
        } else if let imageURL = info[.imageURL] as? URL {
            // Image with URL
            sendCaptureResult(getMediaInfo(url: imageURL))
        } else if let image = info[.originalImage] as? UIImage {
            // Image without URL - save to temp
            let tempURL = FileManager.default.temporaryDirectory
                .appendingPathComponent(UUID().uuidString)
                .appendingPathExtension("jpg")

            if let data = image.jpegData(compressionQuality: 0.9) {
                do {
                    try data.write(to: tempURL)
                    sendCaptureResult([
                        "path": tempURL.path,
                        "mimeType": "image/jpeg",
                        "width": Int(image.size.width),
                        "height": Int(image.size.height),
                        "size": Int64(data.count),
                        "durationMs": 0
                    ])
                } catch {
                    sendCancelled("capture")
                }
            } else {
                sendCancelled("capture")
            }
        } else {
            sendCancelled("capture")
        }
    }

    func imagePickerControllerDidCancel(_ picker: UIImagePickerController) {
        picker.dismiss(animated: true)
        sendCancelled("capture")
    }
}

// MARK: - PHPickerViewControllerDelegate

extension CameraHandler: PHPickerViewControllerDelegate {
    func picker(_ picker: PHPickerViewController, didFinishPicking results: [PHPickerResult]) {
        picker.dismiss(animated: true)

        guard !results.isEmpty else {
            sendCancelled("gallery")
            return
        }

        // Use a serial queue for thread-safe array access
        let syncQueue = DispatchQueue(label: "com.drift.camera.gallery")
        var mediaItems: [[String: Any]] = []
        let group = DispatchGroup()

        for result in results {
            group.enter()

            let provider = result.itemProvider

            if provider.hasItemConformingToTypeIdentifier("public.movie") {
                provider.loadFileRepresentation(forTypeIdentifier: "public.movie") { url, error in
                    guard let url = url else {
                        group.leave()
                        return
                    }

                    // Copy to temp location
                    let tempURL = FileManager.default.temporaryDirectory
                        .appendingPathComponent(UUID().uuidString)
                        .appendingPathExtension(url.pathExtension)

                    do {
                        try FileManager.default.copyItem(at: url, to: tempURL)
                        let info = self.getMediaInfo(url: tempURL)
                        syncQueue.sync {
                            mediaItems.append(info)
                        }
                    } catch {
                        // Ignore failed items
                    }
                    group.leave()
                }
            } else if provider.hasItemConformingToTypeIdentifier("public.image") {
                provider.loadFileRepresentation(forTypeIdentifier: "public.image") { url, error in
                    guard let url = url else {
                        group.leave()
                        return
                    }

                    // Copy to temp location
                    let tempURL = FileManager.default.temporaryDirectory
                        .appendingPathComponent(UUID().uuidString)
                        .appendingPathExtension(url.pathExtension)

                    do {
                        try FileManager.default.copyItem(at: url, to: tempURL)
                        let info = self.getMediaInfo(url: tempURL)
                        syncQueue.sync {
                            mediaItems.append(info)
                        }
                    } catch {
                        // Ignore failed items
                    }
                    group.leave()
                }
            } else {
                group.leave()
            }
        }

        group.notify(queue: .main) {
            syncQueue.sync {
                self.sendGalleryResult(mediaItems)
            }
        }
    }
}
