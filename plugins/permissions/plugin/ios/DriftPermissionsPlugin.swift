/// DriftPermissionsPlugin.swift
///
/// The permissions plugin's native half on iOS: registers each permission
/// kind enabled in drift.yaml (host.config's "kinds", from the build).
/// Drift's permission mechanism does the rest.

import AVFoundation
import Contacts
import DriftPluginAPI
import EventKit
import Photos

/// Public, with a public initializer: the app's generated registrant
/// creates it from outside this module (DriftPlugin_permissions).
public final class DriftPermissionsPlugin: DriftPlugin {
    public init() {}

    public func register(host: DriftPluginHost) {
        let all: [String: () -> DriftPermission] = [
            "contacts": { ContactsPermission() },
            "calendar": { CalendarPermission() },
            "microphone": { MicrophonePermission() },
            "photos": { PhotosPermission() },
        ]
        guard let kinds = host.config["kinds"] as? [String] else {
            fatalError("drift permissions plugin: config has no kinds: \(host.config)")
        }
        for kind in kinds {
            guard let make = all[kind] else {
                fatalError("drift permissions plugin: unknown kind \(kind)")
            }
            host.registerPermission(kind, make())
        }
    }
}

private final class ContactsPermission: DriftPermission {
    func status() -> DriftPermissionStatus {
        switch CNContactStore.authorizationStatus(for: .contacts) {
        case .authorized: return .granted
        case .limited: return .limited
        // iOS asks once; after a denial only Settings can change it.
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

private final class MicrophonePermission: DriftPermission {
    func status() -> DriftPermissionStatus {
        switch AVCaptureDevice.authorizationStatus(for: .audio) {
        case .authorized: return .granted
        case .denied: return .permanentlyDenied
        case .restricted: return .restricted
        case .notDetermined: return .notDetermined
        @unknown default: return .unknown
        }
    }

    func request(options: [String: Any], completion: @escaping (DriftPermissionStatus) -> Void) {
        AVCaptureDevice.requestAccess(for: .audio) { [self] _ in
            completion(status())
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
