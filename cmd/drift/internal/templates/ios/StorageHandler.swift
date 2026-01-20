/// StorageHandler.swift
/// Handles file system access and document picking for the Drift platform channel.

import UIKit
import UniformTypeIdentifiers

final class StorageHandler: NSObject {
    static let shared = StorageHandler()

    private var pendingSaveData: Data?
    private var pendingRequestType: String = "pickFile"

    private override init() {
        super.init()
    }

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "pickFile":
            return shared.pickFile(args: args)
        case "pickDirectory":
            return shared.pickDirectory()
        case "saveFile":
            return shared.saveFile(args: args)
        case "readFile":
            return readFile(args: args)
        case "writeFile":
            return writeFile(args: args)
        case "deleteFile":
            return deleteFile(args: args)
        case "getFileInfo":
            return getFileInfo(args: args)
        case "getAppDirectory":
            return getAppDirectory(args: args)
        default:
            return (nil, NSError(domain: "Storage", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    private func pickFile(args: Any?) -> (Any?, Error?) {
        let dict = args as? [String: Any] ?? [:]
        let allowMultiple = dict["allowMultiple"] as? Bool ?? false
        let allowedTypes = dict["allowedTypes"] as? [String] ?? ["public.item"]

        pendingRequestType = "pickFile"

        DispatchQueue.main.async {
            self.presentDocumentPicker(allowMultiple: allowMultiple, allowedTypes: allowedTypes)
        }

        // Result will be delivered via drift/storage/result event channel
        return (["pending": true], nil)
    }

    private func presentDocumentPicker(allowMultiple: Bool, allowedTypes: [String]) {
        var contentTypes: [UTType] = []
        for typeString in allowedTypes {
            if let type = UTType(typeString) {
                contentTypes.append(type)
            } else if let type = UTType(mimeType: typeString) {
                contentTypes.append(type)
            }
        }

        if contentTypes.isEmpty {
            contentTypes = [.item]
        }

        let picker = UIDocumentPickerViewController(forOpeningContentTypes: contentTypes, asCopy: true)
        picker.allowsMultipleSelection = allowMultiple
        picker.delegate = self

        presentPicker(picker)
    }

    private func pickDirectory() -> (Any?, Error?) {
        pendingRequestType = "pickDirectory"

        DispatchQueue.main.async {
            let picker = UIDocumentPickerViewController(forOpeningContentTypes: [.folder])
            picker.delegate = self
            self.presentPicker(picker)
        }

        // Result will be delivered via drift/storage/result event channel
        return (["pending": true], nil)
    }

    private func saveFile(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any] else {
            return (nil, NSError(domain: "Storage", code: 400, userInfo: [NSLocalizedDescriptionKey: "Invalid arguments"]))
        }

        let suggestedName = dict["suggestedName"] as? String ?? "file"

        guard let data = StorageHandler.extractData(from: dict["data"]) else {
            return (nil, NSError(domain: "Storage", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing data"]))
        }

        // Save to temp file first, then present export picker
        let tempURL = FileManager.default.temporaryDirectory.appendingPathComponent(suggestedName)

        do {
            try data.write(to: tempURL)
            pendingSaveData = data

            DispatchQueue.main.async {
                let activityVC = UIActivityViewController(activityItems: [tempURL], applicationActivities: nil)
                activityVC.completionWithItemsHandler = { _, completed, _, error in
                    if completed {
                        self.sendSaveFileResult(tempURL.path)
                    } else if let error = error {
                        self.sendError("saveFile", message: error.localizedDescription)
                    } else {
                        self.sendCancelled("saveFile")
                    }
                    self.pendingSaveData = nil
                }

                if let windowScene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
                   let rootVC = windowScene.windows.first?.rootViewController {
                    var topVC = rootVC
                    while let presented = topVC.presentedViewController {
                        topVC = presented
                    }

                    if let popover = activityVC.popoverPresentationController {
                        popover.sourceView = topVC.view
                        popover.sourceRect = CGRect(x: topVC.view.bounds.midX, y: topVC.view.bounds.midY, width: 0, height: 0)
                    }

                    topVC.present(activityVC, animated: true)
                }
            }

            // Result will be delivered via event channel
            return (["pending": true], nil)
        } catch {
            return (nil, error)
        }
    }

    private static func readFile(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let path = dict["path"] as? String else {
            return (nil, NSError(domain: "Storage", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing path"]))
        }

        do {
            let url = URL(fileURLWithPath: path)
            let data = try Data(contentsOf: url)
            return (["data": [UInt8](data)], nil)
        } catch {
            return (nil, error)
        }
    }

    private static func writeFile(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let path = dict["path"] as? String else {
            return (nil, NSError(domain: "Storage", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing path"]))
        }

        guard let data = extractData(from: dict["data"]) else {
            return (nil, NSError(domain: "Storage", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing data"]))
        }

        do {
            let url = URL(fileURLWithPath: path)
            try data.write(to: url)
            return (nil, nil)
        } catch {
            return (nil, error)
        }
    }

    private static func deleteFile(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let path = dict["path"] as? String else {
            return (nil, NSError(domain: "Storage", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing path"]))
        }

        do {
            let url = URL(fileURLWithPath: path)
            try FileManager.default.removeItem(at: url)
            return (nil, nil)
        } catch {
            return (nil, error)
        }
    }

    private static func getFileInfo(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let path = dict["path"] as? String else {
            return (nil, NSError(domain: "Storage", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing path"]))
        }

        let url = URL(fileURLWithPath: path)

        do {
            let attrs = try FileManager.default.attributesOfItem(atPath: path)
            let isDirectory = (attrs[.type] as? FileAttributeType) == .typeDirectory
            let size = attrs[.size] as? Int64 ?? 0
            let modificationDate = attrs[.modificationDate] as? Date

            var mimeType = ""
            if let uti = UTType(filenameExtension: url.pathExtension) {
                mimeType = uti.preferredMIMEType ?? ""
            }

            return ([
                "name": url.lastPathComponent,
                "path": path,
                "size": size,
                "mimeType": mimeType,
                "isDirectory": isDirectory,
                "lastModified": Int64((modificationDate?.timeIntervalSince1970 ?? 0) * 1000)
            ], nil)
        } catch {
            return (nil, error)
        }
    }

    private static func getAppDirectory(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let directory = dict["directory"] as? String else {
            return (nil, NSError(domain: "Storage", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing directory"]))
        }

        let fileManager = FileManager.default
        let path: String?

        switch directory {
        case "documents":
            path = fileManager.urls(for: .documentDirectory, in: .userDomainMask).first?.path
        case "cache":
            path = fileManager.urls(for: .cachesDirectory, in: .userDomainMask).first?.path
        case "temp":
            path = NSTemporaryDirectory()
        case "support":
            path = fileManager.urls(for: .applicationSupportDirectory, in: .userDomainMask).first?.path
        default:
            path = fileManager.urls(for: .documentDirectory, in: .userDomainMask).first?.path
        }

        return (["path": path], nil)
    }

    private static func extractData(from value: Any?) -> Data? {
        if let data = value as? Data {
            return data
        }
        if let bytes = value as? [UInt8] {
            return Data(bytes)
        }
        if let bytes = value as? [Int] {
            return Data(bytes.map { UInt8(truncatingIfNeeded: $0) })
        }
        if let string = value as? String {
            return string.data(using: .utf8)
        }
        return nil
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

    // MARK: - Event Sending

    private func sendPickFileResult(_ files: [[String: Any]]) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/storage/result", data: [
            "type": "pickFile",
            "files": files
        ])
    }

    private func sendPickDirectoryResult(_ path: String?) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/storage/result", data: [
            "type": "pickDirectory",
            "path": path as Any
        ])
    }

    private func sendSaveFileResult(_ path: String) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/storage/result", data: [
            "type": "saveFile",
            "path": path
        ])
    }

    private func sendCancelled(_ requestType: String) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/storage/result", data: [
            "type": requestType,
            "cancelled": true
        ])
    }

    private func sendError(_ requestType: String, message: String) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/storage/result", data: [
            "type": requestType,
            "error": message
        ])
    }
}

// MARK: - UIDocumentPickerDelegate

extension StorageHandler: UIDocumentPickerDelegate {
    func documentPicker(_ controller: UIDocumentPickerViewController, didPickDocumentsAt urls: [URL]) {
        var files: [[String: Any]] = []

        for url in urls {
            // Start accessing security-scoped resource
            let accessing = url.startAccessingSecurityScopedResource()
            defer {
                if accessing {
                    url.stopAccessingSecurityScopedResource()
                }
            }

            do {
                let attrs = try FileManager.default.attributesOfItem(atPath: url.path)
                let isDirectory = (attrs[.type] as? FileAttributeType) == .typeDirectory
                let size = attrs[.size] as? Int64 ?? 0

                var mimeType = ""
                if let uti = UTType(filenameExtension: url.pathExtension) {
                    mimeType = uti.preferredMIMEType ?? ""
                }

                files.append([
                    "name": url.lastPathComponent,
                    "path": url.path,
                    "uri": url.absoluteString,
                    "mimeType": mimeType,
                    "size": size,
                    "isDirectory": isDirectory
                ])
            } catch {
                // Skip files we can't read
            }
        }

        // Check if this was a directory picker (single folder selection)
        if files.count == 1, let file = files.first, file["isDirectory"] as? Bool == true {
            sendPickDirectoryResult(file["path"] as? String)
        } else {
            sendPickFileResult(files)
        }
    }

    func documentPickerWasCancelled(_ controller: UIDocumentPickerViewController) {
        sendCancelled(pendingRequestType)
    }
}
