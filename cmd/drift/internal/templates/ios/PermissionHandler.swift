/// PermissionHandler.swift
/// Handles runtime permission requests for the Drift platform channel.

import UIKit
import AVFoundation
import Photos
import CoreLocation
import Contacts
import EventKit
import UserNotifications

enum PermissionHandler {
    private static let locationManager = CLLocationManager()
    private static var locationDelegate: LocationPermissionDelegate?

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "check":
            return check(args: args)
        case "request":
            return request(args: args)
        case "requestMultiple":
            return requestMultiple(args: args)
        case "openSettings":
            return openSettings()
        case "shouldShowRationale":
            // iOS doesn't have this concept
            return (["shouldShow": false], nil)
        default:
            return (nil, NSError(domain: "Permissions", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    private static func check(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let permission = dict["permission"] as? String else {
            return (nil, NSError(domain: "Permissions", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing permission"]))
        }

        let status = checkPermissionStatus(permission)
        return (["status": status], nil)
    }

    private static func request(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let permission = dict["permission"] as? String else {
            return (nil, NSError(domain: "Permissions", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing permission"]))
        }

        let currentStatus = checkPermissionStatus(permission)
        requestPermission(permission)
        return (["status": currentStatus], nil)
    }

    private static func requestMultiple(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let permissions = dict["permissions"] as? [String] else {
            return (nil, NSError(domain: "Permissions", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing permissions"]))
        }

        var results: [String: String] = [:]
        for permission in permissions {
            results[permission] = checkPermissionStatus(permission)
            requestPermission(permission)
        }

        return (["results": results], nil)
    }

    private static func openSettings() -> (Any?, Error?) {
        DispatchQueue.main.async {
            if let url = URL(string: UIApplication.openSettingsURLString) {
                UIApplication.shared.open(url)
            }
        }
        return (nil, nil)
    }

    private static func checkPermissionStatus(_ permission: String) -> String {
        switch permission {
        case "camera":
            return cameraStatus()
        case "microphone":
            return microphoneStatus()
        case "photos":
            return photosStatus()
        case "location":
            return locationStatus(always: false)
        case "location_always":
            return locationStatus(always: true)
        case "contacts":
            return contactsStatus()
        case "calendar":
            return calendarStatus()
        case "notifications":
            return notificationsStatus()
        default:
            return "unknown"
        }
    }

    private static func requestPermission(_ permission: String) {
        switch permission {
        case "camera":
            requestCamera()
        case "microphone":
            requestMicrophone()
        case "photos":
            requestPhotos()
        case "location":
            requestLocation(always: false)
        case "location_always":
            requestLocation(always: true)
        case "contacts":
            requestContacts()
        case "calendar":
            requestCalendar()
        case "notifications":
            requestNotifications()
        default:
            break
        }
    }

    // MARK: - Camera

    private static func cameraStatus() -> String {
        switch AVCaptureDevice.authorizationStatus(for: .video) {
        case .authorized:
            return "granted"
        case .denied:
            return "permanently_denied"
        case .restricted:
            return "restricted"
        case .notDetermined:
            return "not_determined"
        @unknown default:
            return "unknown"
        }
    }

    private static func requestCamera() {
        AVCaptureDevice.requestAccess(for: .video) { granted in
            let status = granted ? "granted" : "denied"
            sendPermissionChange("camera", status: status)
        }
    }

    // MARK: - Microphone

    private static func microphoneStatus() -> String {
        switch AVCaptureDevice.authorizationStatus(for: .audio) {
        case .authorized:
            return "granted"
        case .denied:
            return "permanently_denied"
        case .restricted:
            return "restricted"
        case .notDetermined:
            return "not_determined"
        @unknown default:
            return "unknown"
        }
    }

    private static func requestMicrophone() {
        AVCaptureDevice.requestAccess(for: .audio) { granted in
            let status = granted ? "granted" : "denied"
            sendPermissionChange("microphone", status: status)
        }
    }

    // MARK: - Photos

    private static func photosStatus() -> String {
        let status: PHAuthorizationStatus
        if #available(iOS 14, *) {
            status = PHPhotoLibrary.authorizationStatus(for: .readWrite)
        } else {
            status = PHPhotoLibrary.authorizationStatus()
        }

        switch status {
        case .authorized:
            return "granted"
        case .denied:
            return "permanently_denied"
        case .restricted:
            return "restricted"
        case .notDetermined:
            return "not_determined"
        case .limited:
            return "limited"
        @unknown default:
            return "unknown"
        }
    }

    private static func requestPhotos() {
        if #available(iOS 14, *) {
            PHPhotoLibrary.requestAuthorization(for: .readWrite) { status in
                let statusStr: String
                switch status {
                case .authorized:
                    statusStr = "granted"
                case .limited:
                    statusStr = "limited"
                default:
                    statusStr = "denied"
                }
                sendPermissionChange("photos", status: statusStr)
            }
        } else {
            PHPhotoLibrary.requestAuthorization { status in
                let statusStr = status == .authorized ? "granted" : "denied"
                sendPermissionChange("photos", status: statusStr)
            }
        }
    }

    // MARK: - Location

    private static func locationStatus(always: Bool) -> String {
        let status = locationManager.authorizationStatus

        switch status {
        case .authorizedAlways:
            return "granted"
        case .authorizedWhenInUse:
            return always ? "denied" : "granted"
        case .denied:
            return "permanently_denied"
        case .restricted:
            return "restricted"
        case .notDetermined:
            return "not_determined"
        @unknown default:
            return "unknown"
        }
    }

    private static func requestLocation(always: Bool) {
        locationDelegate = LocationPermissionDelegate(always: always)
        locationManager.delegate = locationDelegate

        if always {
            locationManager.requestAlwaysAuthorization()
        } else {
            locationManager.requestWhenInUseAuthorization()
        }
    }

    // MARK: - Contacts

    private static func contactsStatus() -> String {
        switch CNContactStore.authorizationStatus(for: .contacts) {
        case .authorized:
            return "granted"
        case .denied:
            return "permanently_denied"
        case .restricted:
            return "restricted"
        case .notDetermined:
            return "not_determined"
        case .limited:
            return "limited"
        @unknown default:
            return "unknown"
        }
    }

    private static func requestContacts() {
        CNContactStore().requestAccess(for: .contacts) { granted, _ in
            let status = granted ? "granted" : "denied"
            sendPermissionChange("contacts", status: status)
        }
    }

    // MARK: - Calendar

    private static func calendarStatus() -> String {
        switch EKEventStore.authorizationStatus(for: .event) {
        case .authorized:
            return "granted"
        case .denied:
            return "permanently_denied"
        case .restricted:
            return "restricted"
        case .notDetermined:
            return "not_determined"
        case .fullAccess:
            return "granted"
        case .writeOnly:
            return "limited"
        @unknown default:
            return "unknown"
        }
    }

    private static func requestCalendar() {
        if #available(iOS 17.0, *) {
            EKEventStore().requestFullAccessToEvents { granted, _ in
                let status = granted ? "granted" : "denied"
                sendPermissionChange("calendar", status: status)
            }
        } else {
            EKEventStore().requestAccess(to: .event) { granted, _ in
                let status = granted ? "granted" : "denied"
                sendPermissionChange("calendar", status: status)
            }
        }
    }

    // MARK: - Notifications

    private static func notificationsStatus() -> String {
        var status = "unknown"
        let semaphore = DispatchSemaphore(value: 0)

        UNUserNotificationCenter.current().getNotificationSettings { settings in
            switch settings.authorizationStatus {
            case .authorized:
                status = "granted"
            case .denied:
                status = "permanently_denied"
            case .notDetermined:
                status = "not_determined"
            case .provisional:
                status = "provisional"
            case .ephemeral:
                status = "provisional"
            @unknown default:
                status = "unknown"
            }
            semaphore.signal()
        }

        semaphore.wait()
        return status
    }

    private static func requestNotifications() {
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge]) { granted, _ in
            let status = granted ? "granted" : "denied"
            sendPermissionChange("notifications", status: status)
        }
    }

    // MARK: - Helpers

    private static func sendPermissionChange(_ permission: String, status: String) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/permissions/changes", data: [
            "permission": permission,
            "status": status
        ])
    }
}

// MARK: - Location Permission Delegate

private class LocationPermissionDelegate: NSObject, CLLocationManagerDelegate {
    private let always: Bool

    init(always: Bool) {
        self.always = always
    }

    func locationManagerDidChangeAuthorization(_ manager: CLLocationManager) {
        let permission = always ? "location_always" : "location"
        let status: String

        switch manager.authorizationStatus {
        case .authorizedAlways:
            status = "granted"
        case .authorizedWhenInUse:
            status = always ? "denied" : "granted"
        case .denied:
            status = "permanently_denied"
        case .restricted:
            status = "restricted"
        case .notDetermined:
            return // Still waiting
        @unknown default:
            status = "unknown"
        }

        PlatformChannelManager.shared.sendEvent(channel: "drift/permissions/changes", data: [
            "permission": permission,
            "status": status
        ])
    }
}
