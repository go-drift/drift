/// StorageHandler.swift
/// The drift/storage channel: document pickers and file system access.
///
/// The pickers reply when the user finishes ({"files": [...]} or
/// {"path": ...}) or cancels (nil); one picker at a time, a second fails
/// with `storage_busy`. File I/O runs off the main thread. File contents
/// cross the channel as base64.

import DriftPluginAPI
import UIKit
import UniformTypeIdentifiers

final class StorageHandler: NSObject {
    static let shared = StorageHandler()

    /// The open picker's reply; nil when none is open. Main thread only.
    private var pending: DriftResult?
    private var pendingIsDirectory = false

    private let io = DispatchQueue(label: "drift.storage.io", qos: .userInitiated)

    static func start(host: DriftPluginHost) {
        host.registerChannel("drift/storage") { channel in
            channel.asyncMethod("pickFile") { args, result in shared.pickFile(args, result) }
            channel.asyncMethod("pickDirectory") { _, result in shared.pickDirectory(result) }
            channel.asyncMethod("saveFile") { args, result in shared.saveFile(args, result) }
            channel.asyncMethod("readFile") { args, result in
                shared.background(result) { ["data": try Data(contentsOf: URL(fileURLWithPath: try path(args))).base64EncodedString()] }
            }
            channel.asyncMethod("writeFile") { args, result in
                shared.background(result) {
                    try data(args).write(to: URL(fileURLWithPath: try path(args)))
                    return nil
                }
            }
            channel.asyncMethod("deleteFile") { args, result in
                shared.background(result) {
                    try FileManager.default.removeItem(atPath: try path(args))
                    return nil
                }
            }
            channel.asyncMethod("getFileInfo") { args, result in
                shared.background(result) { try fileInfo(path(args)) }
            }
            channel.method("getAppDirectory") { args in
                guard let directory = (args as? [String: Any])?["directory"] as? String,
                      let path = appDirectory(directory) else {
                    throw invalid("missing or unknown directory")
                }
                return ["path": path]
            }
        }
    }

    // MARK: - Pickers

    private func pickFile(_ args: Any?, _ result: DriftResult) {
        let dict = args as? [String: Any] ?? [:]
        let allowMultiple = dict["allowMultiple"] as? Bool ?? false
        let allowedTypes = dict["allowedTypes"] as? [String] ?? []
        var contentTypes = allowedTypes.compactMap { UTType($0) ?? UTType(mimeType: $0) }
        if contentTypes.isEmpty {
            contentTypes = [.item]
        }
        let picker = UIDocumentPickerViewController(forOpeningContentTypes: contentTypes, asCopy: true)
        picker.allowsMultipleSelection = allowMultiple
        present(picker, isDirectory: false, result)
    }

    private func pickDirectory(_ result: DriftResult) {
        present(UIDocumentPickerViewController(forOpeningContentTypes: [.folder]), isDirectory: true, result)
    }

    private func present(_ picker: UIDocumentPickerViewController, isDirectory: Bool, _ result: DriftResult) {
        guard begin(result) else { return }
        guard let top = Self.topViewController() else {
            finish()?.error(Self.noScreen())
            return
        }
        pendingIsDirectory = isDirectory
        picker.delegate = self
        top.present(picker, animated: true)
    }

    private func saveFile(_ args: Any?, _ result: DriftResult) {
        let dict = args as? [String: Any] ?? [:]
        let bytes: Data
        do {
            bytes = try Self.data(args)
        } catch {
            result.error(error)
            return
        }
        guard begin(result) else { return }
        guard let top = Self.topViewController() else {
            finish()?.error(Self.noScreen())
            return
        }
        let suggestedName = (dict["suggestedName"] as? String).flatMap { $0.isEmpty ? nil : $0 } ?? "file"
        let url = FileManager.default.temporaryDirectory.appendingPathComponent(suggestedName)
        do {
            try bytes.write(to: url)
        } catch {
            finish()?.error(error)
            return
        }
        let share = UIActivityViewController(activityItems: [url], applicationActivities: nil)
        share.completionWithItemsHandler = { [weak self] _, completed, _, error in
            guard let reply = self?.finish() else { return }
            if let error {
                reply.error(error)
            } else {
                reply.success(completed ? ["path": url.path] : nil)
            }
        }
        if let popover = share.popoverPresentationController {
            popover.sourceView = top.view
            popover.sourceRect = CGRect(x: top.view.bounds.midX, y: top.view.bounds.midY, width: 0, height: 0)
        }
        top.present(share, animated: true)
    }

    /// Claims the picker slot for result; false (and result fails) if a
    /// picker is open.
    private func begin(_ result: DriftResult) -> Bool {
        guard pending == nil else {
            result.error(DriftCallError("storage_busy", "a storage picker is already open"))
            return false
        }
        pending = result
        return true
    }

    /// Releases the picker slot, returning its reply.
    private func finish() -> DriftResult? {
        defer { pending = nil }
        return pending
    }

    private static func topViewController() -> UIViewController? {
        guard let scene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
              var top = scene.windows.first?.rootViewController else {
            return nil
        }
        while let presented = top.presentedViewController {
            top = presented
        }
        return top
    }

    // MARK: - File system

    /// Runs work on the I/O queue and replies with its value or error.
    private func background(_ result: DriftResult, _ work: @escaping () throws -> Any?) {
        io.async {
            do {
                result.success(try work())
            } catch {
                result.error(error)
            }
        }
    }

    private static func fileInfo(_ path: String) throws -> [String: Any] {
        let url = URL(fileURLWithPath: path)
        let attrs = try FileManager.default.attributesOfItem(atPath: path)
        let modified = attrs[.modificationDate] as? Date
        return [
            "name": url.lastPathComponent,
            "path": path,
            "size": (attrs[.size] as? NSNumber)?.int64Value ?? 0,
            "mimeType": mimeType(url),
            "isDirectory": (attrs[.type] as? FileAttributeType) == .typeDirectory,
            "lastModified": Int64((modified?.timeIntervalSince1970 ?? 0) * 1000),
        ]
    }

    private static func appDirectory(_ directory: String) -> String? {
        let fileManager = FileManager.default
        switch directory {
        case "documents":
            return fileManager.urls(for: .documentDirectory, in: .userDomainMask).first?.path
        case "cache":
            return fileManager.urls(for: .cachesDirectory, in: .userDomainMask).first?.path
        case "temp":
            return NSTemporaryDirectory()
        case "support":
            return fileManager.urls(for: .applicationSupportDirectory, in: .userDomainMask).first?.path
        default:
            return nil
        }
    }

    private static func mimeType(_ url: URL) -> String {
        UTType(filenameExtension: url.pathExtension)?.preferredMIMEType ?? ""
    }

    private static func path(_ args: Any?) throws -> String {
        guard let path = (args as? [String: Any])?["path"] as? String else {
            throw invalid("missing path")
        }
        return path
    }

    /// The base64 `data` argument.
    private static func data(_ args: Any?) throws -> Data {
        guard let encoded = (args as? [String: Any])?["data"] as? String,
              let data = Data(base64Encoded: encoded) else {
            throw invalid("missing or non-base64 data")
        }
        return data
    }

    private static func invalid(_ message: String) -> DriftCallError {
        DriftCallError.invalidArguments("drift/storage: \(message)")
    }

    private static func noScreen() -> DriftCallError {
        DriftCallError("platform_unavailable", "no screen to show the picker over")
    }
}

// MARK: - UIDocumentPickerDelegate

extension StorageHandler: UIDocumentPickerDelegate {
    func documentPicker(_ controller: UIDocumentPickerViewController, didPickDocumentsAt urls: [URL]) {
        guard let reply = finish() else { return }
        if pendingIsDirectory {
            reply.success(urls.first.map { ["path": $0.path] })
            return
        }
        var files: [[String: Any]] = []
        for url in urls {
            let accessing = url.startAccessingSecurityScopedResource()
            defer {
                if accessing {
                    url.stopAccessingSecurityScopedResource()
                }
            }
            let size = ((try? FileManager.default.attributesOfItem(atPath: url.path))?[.size] as? NSNumber)?.int64Value ?? 0
            files.append([
                "name": url.lastPathComponent,
                "path": url.path,
                "uri": url.absoluteString,
                "mimeType": Self.mimeType(url),
                "size": size,
            ])
        }
        reply.success(["files": files])
    }

    func documentPickerWasCancelled(_ controller: UIDocumentPickerViewController) {
        finish()?.success(nil)
    }
}
