/// PermissionHandler.swift
///
/// The drift/permissions channel: Drift's permission request mechanism.
/// Permission kinds are registered by plugins (DriftPluginHost
/// registerPermission) and by Drift itself (notifications, below); Go's
/// platform.Permission reaches a kind by name. "request" is an async method
/// that replies with the status once the user has answered, and reports it
/// on drift/permissions/changes.

import AVFoundation
import Contacts
import DriftPluginAPI
import EventKit
import Photos
import UIKit
import UserNotifications

enum PermissionHandler {
    private static var kinds: [String: DriftPermission] = [:]
    private static let kindsLock = NSLock()

    /// Registers the permission `kind`; a second registration traps.
    static func register(_ kind: String, _ permission: DriftPermission) {
        kindsLock.lock()
        defer { kindsLock.unlock() }
        if kinds[kind] != nil {
            fatalError("drift: permission kind \"\(kind)\" is already registered")
        }
        kinds[kind] = permission
    }

    /// Registers the drift/permissions channel and Drift's own kinds.
    static func start(host: DriftPluginHost) {
        host.registerChannel("drift/permissions") { channel in
            channel.method("check") { args in
                ["status": try lookup(args).permission.status().rawValue]
            }
            channel.asyncMethod("request") { args, result in
                let entry: (kind: String, permission: DriftPermission)
                do {
                    entry = try lookup(args)
                } catch {
                    result.error(error)
                    return
                }
                request(entry.kind, entry.permission, options: args as? [String: Any] ?? [:], result: result)
            }
            channel.method("shouldShowRationale") { args in
                _ = try lookup(args)
                // iOS has no rationale concept.
                return ["shouldShow": false]
            }
            channel.method("openSettings") { _ in
                if let url = URL(string: UIApplication.openSettingsURLString) {
                    UIApplication.shared.open(url)
                }
                return nil
            }
        }

        host.registerPermission("notifications", NotificationsPermission())
        // Moving to plugins (Phase 5); registered here until each moves.
        host.registerPermission("camera", CapturePermission(.video))
        host.registerPermission("microphone", CapturePermission(.audio))
        host.registerPermission("photos", PhotosPermission())
        host.registerPermission("contacts", ContactsPermission())
        host.registerPermission("calendar", CalendarPermission())
    }

    private static func lookup(_ args: Any?) throws -> (kind: String, permission: DriftPermission) {
        guard let kind = (args as? [String: Any])?["permission"] as? String else {
            throw NSError(domain: "invalid_arguments", code: 0, userInfo: [
                NSLocalizedDescriptionKey: "drift/permissions: missing permission kind",
            ])
        }
        kindsLock.lock()
        let permission = kinds[kind]
        kindsLock.unlock()
        guard let permission else {
            throw NSError(domain: "permission_not_registered", code: 0, userInfo: [
                NSLocalizedDescriptionKey: "no plugin registered the permission kind \"\(kind)\"; add the plugin that provides it to drift.yaml",
            ])
        }
        return (kind, permission)
    }

    private static func request(_ kind: String, _ permission: DriftPermission, options: [String: Any], result: DriftResult) {
        let current = permission.status()
        switch current {
        case .granted, .permanentlyDenied, .restricted:
            // No prompt can change these.
            result.success(["status": current.rawValue])
            return
        default:
            break
        }
        var options = options
        options.removeValue(forKey: "permission")
        permission.request(options: options) { status in
            DispatchQueue.main.async {
                PlatformChannelManager.shared.sendEvent(channel: "drift/permissions/changes", data: [
                    "permission": kind,
                    "status": status.rawValue,
                ])
            }
            result.success(["status": status.rawValue])
        }
    }
}

// MARK: - Notifications (Drift's own kind)

/// Notification authorization. No Info.plist key is needed; the alert,
/// sound, badge and provisional options come from Go's RequestWithOptions
/// (alert, sound and badge default to on).
private final class NotificationsPermission: DriftPermission {
    func status() -> DriftPermissionStatus {
        // UNUserNotificationCenter answers on its own queue, so waiting on
        // the main thread cannot deadlock.
        var status = DriftPermissionStatus.unknown
        let done = DispatchSemaphore(value: 0)
        UNUserNotificationCenter.current().getNotificationSettings { settings in
            status = Self.map(settings.authorizationStatus)
            done.signal()
        }
        done.wait()
        return status
    }

    func request(options: [String: Any], completion: @escaping (DriftPermissionStatus) -> Void) {
        var authOptions: UNAuthorizationOptions = []
        if options["alert"] as? Bool ?? true { authOptions.insert(.alert) }
        if options["sound"] as? Bool ?? true { authOptions.insert(.sound) }
        if options["badge"] as? Bool ?? true { authOptions.insert(.badge) }
        if options["provisional"] as? Bool ?? false { authOptions.insert(.provisional) }
        let center = UNUserNotificationCenter.current()
        center.requestAuthorization(options: authOptions) { _, _ in
            center.getNotificationSettings { settings in
                completion(Self.map(settings.authorizationStatus))
            }
        }
    }

    private static func map(_ status: UNAuthorizationStatus) -> DriftPermissionStatus {
        switch status {
        case .authorized: return .granted
        // iOS asks once; after a denial only Settings can change it.
        case .denied: return .permanentlyDenied
        case .notDetermined: return .notDetermined
        case .provisional, .ephemeral: return .provisional
        @unknown default: return .unknown
        }
    }
}

// MARK: - Kinds moving to plugins

/// Camera or microphone access.
private final class CapturePermission: DriftPermission {
    private let media: AVMediaType

    init(_ media: AVMediaType) {
        self.media = media
    }

    func status() -> DriftPermissionStatus {
        switch AVCaptureDevice.authorizationStatus(for: media) {
        case .authorized: return .granted
        case .denied: return .permanentlyDenied
        case .restricted: return .restricted
        case .notDetermined: return .notDetermined
        @unknown default: return .unknown
        }
    }

    func request(options: [String: Any], completion: @escaping (DriftPermissionStatus) -> Void) {
        AVCaptureDevice.requestAccess(for: media) { granted in
            completion(granted ? .granted : .permanentlyDenied)
        }
    }
}

private final class PhotosPermission: DriftPermission {
    func status() -> DriftPermissionStatus {
        Self.map(PHPhotoLibrary.authorizationStatus(for: .readWrite))
    }

    func request(options: [String: Any], completion: @escaping (DriftPermissionStatus) -> Void) {
        PHPhotoLibrary.requestAuthorization(for: .readWrite) { status in
            completion(Self.map(status))
        }
    }

    private static func map(_ status: PHAuthorizationStatus) -> DriftPermissionStatus {
        switch status {
        case .authorized: return .granted
        case .limited: return .limited
        case .denied: return .permanentlyDenied
        case .restricted: return .restricted
        case .notDetermined: return .notDetermined
        @unknown default: return .unknown
        }
    }
}

private final class ContactsPermission: DriftPermission {
    func status() -> DriftPermissionStatus {
        switch CNContactStore.authorizationStatus(for: .contacts) {
        case .authorized: return .granted
        case .limited: return .limited
        case .denied: return .permanentlyDenied
        case .restricted: return .restricted
        case .notDetermined: return .notDetermined
        @unknown default: return .unknown
        }
    }

    func request(options: [String: Any], completion: @escaping (DriftPermissionStatus) -> Void) {
        CNContactStore().requestAccess(for: .contacts) { [self] _, _ in
            completion(status())
        }
    }
}

private final class CalendarPermission: DriftPermission {
    func status() -> DriftPermissionStatus {
        switch EKEventStore.authorizationStatus(for: .event) {
        case .authorized, .fullAccess: return .granted
        case .writeOnly: return .limited
        case .denied: return .permanentlyDenied
        case .restricted: return .restricted
        case .notDetermined: return .notDetermined
        @unknown default: return .unknown
        }
    }

    func request(options: [String: Any], completion: @escaping (DriftPermissionStatus) -> Void) {
        let done: (Bool, Error?) -> Void = { [self] _, _ in completion(status()) }
        if #available(iOS 17.0, *) {
            EKEventStore().requestFullAccessToEvents(completion: done)
        } else {
            EKEventStore().requestAccess(to: .event, completion: done)
        }
    }
}
