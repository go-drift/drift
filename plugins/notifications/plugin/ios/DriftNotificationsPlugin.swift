/// DriftNotificationsPlugin.swift
///
/// The notifications plugin's native half on iOS, on the main thread.
/// Schedules local notifications through UNUserNotificationCenter, marking
/// each with `markerKey` in its userInfo. Drift's notification-center
/// delegate offers every notification to plugins; this one claims only the
/// marked ones, shows them in the foreground too, and sends them to Go
/// (`received` when shown while running, `opened` when tapped).

import DriftPluginAPI
import UIKit
import UserNotifications

/// Public, with a public initializer: the app's generated registrant
/// creates it from outside this module (DriftPlugin_notifications).
public final class DriftNotificationsPlugin: DriftPlugin {
    private static let channel = "drift/notifications"
    private static let receivedChannel = "drift/notifications/received"
    private static let openedChannel = "drift/notifications/opened"
    /// The userInfo key marking this plugin's notifications; its value is
    /// the notification ID.
    private static let markerKey = "drift.notification_id"

    private var host: DriftPluginHost?
    private let center = UNUserNotificationCenter.current()

    public init() {}

    public func register(host: DriftPluginHost) {
        self.host = host
        host.registerChannel(Self.channel) { channel in
            // Async: UNUserNotificationCenter reports a failed add only
            // through its completion handler.
            channel.asyncMethod("schedule") { [self] args, result in
                let request: UNNotificationRequest
                do {
                    request = try Self.request(args as? [String: Any] ?? [:])
                } catch {
                    result.error(error)
                    return
                }
                center.add(request) { error in
                    if let error {
                        result.error(error)
                    } else {
                        result.success(nil)
                    }
                }
            }
            channel.method("cancel") { [self] args in
                guard let id = (args as? [String: Any])?["id"] as? String else {
                    throw Self.invalid("cancel needs an id")
                }
                center.removePendingNotificationRequests(withIdentifiers: [id])
                center.removeDeliveredNotifications(withIdentifiers: [id])
                return nil
            }
            channel.method("cancelAll") { [self] _ in
                cancelAll()
                return nil
            }
            channel.method("setBadge") { args in
                guard let count = (args as? [String: Any])?["count"] as? Int else {
                    throw Self.invalid("setBadge needs a count")
                }
                if #available(iOS 17.0, *) {
                    UNUserNotificationCenter.current().setBadgeCount(count)
                } else {
                    UIApplication.shared.applicationIconBadgeNumber = count
                }
                return nil
            }
            channel.asyncMethod("getSettings") { [self] _, result in
                center.getNotificationSettings { settings in
                    result.success([
                        "alertsEnabled": settings.alertSetting == .enabled,
                        "soundsEnabled": settings.soundSetting == .enabled,
                        "badgesEnabled": settings.badgeSetting == .enabled,
                    ])
                }
            }
        }
    }

    public func willPresentNotification(_ notification: UNNotification) -> UNNotificationPresentationOptions? {
        guard let id = Self.id(of: notification) else { return nil }
        let content = notification.request.content
        host?.sendEvent(Self.receivedChannel, data: [
            "id": id,
            "title": content.title,
            "body": content.body,
            "data": Self.data(of: content),
            "foreground": true,
        ])
        return [.banner, .list, .sound, .badge]
    }

    public func didReceiveNotificationResponse(_ response: UNNotificationResponse) -> Bool {
        guard let id = Self.id(of: response.notification) else { return false }
        host?.sendEvent(Self.openedChannel, data: [
            "id": id,
            "data": Self.data(of: response.notification.request.content),
        ])
        return true
    }

    // MARK: - Scheduling

    /// The request scheduling args describes.
    private static func request(_ args: [String: Any]) throws -> UNNotificationRequest {
        guard let id = args["id"] as? String, !id.isEmpty else {
            throw invalid("schedule needs an id")
        }
        let content = UNMutableNotificationContent()
        content.title = args["title"] as? String ?? ""
        content.body = args["body"] as? String ?? ""
        if let badge = args["badge"] as? Int {
            content.badge = NSNumber(value: badge)
        }
        if let sound = args["sound"] as? String, !sound.isEmpty {
            content.sound = UNNotificationSound(named: UNNotificationSoundName(sound))
        } else {
            content.sound = .default
        }
        var userInfo = args["data"] as? [String: Any] ?? [:]
        userInfo[markerKey] = id
        content.userInfo = userInfo

        // Go checked the combinations: At wins; Repeats has no At and an
        // interval of at least a minute.
        let interval = (args["intervalSeconds"] as? NSNumber)?.doubleValue ?? 0
        let repeats = args["repeats"] as? Bool ?? false
        let delay: TimeInterval
        if let atMillis = args["at"] as? NSNumber {
            delay = Date(timeIntervalSince1970: atMillis.doubleValue / 1000).timeIntervalSinceNow
        } else {
            delay = interval
        }
        // A time-interval trigger needs a positive interval.
        let trigger = UNTimeIntervalNotificationTrigger(timeInterval: max(delay, 1), repeats: repeats)
        return UNNotificationRequest(identifier: id, content: content, trigger: trigger)
    }

    /// Removes this plugin's notifications only; others (push) stay.
    private func cancelAll() {
        center.getPendingNotificationRequests { [center] requests in
            let ids = requests.filter { $0.content.userInfo[Self.markerKey] != nil }.map(\.identifier)
            center.removePendingNotificationRequests(withIdentifiers: ids)
        }
        center.getDeliveredNotifications { [center] delivered in
            let ids = delivered.compactMap(Self.id(of:))
            center.removeDeliveredNotifications(withIdentifiers: ids)
        }
    }

    // MARK: - Payloads

    /// The ID of one of this plugin's notifications, nil for others.
    private static func id(of notification: UNNotification) -> String? {
        notification.request.content.userInfo[markerKey] as? String
    }

    /// The app's data, without the marker.
    private static func data(of content: UNNotificationContent) -> [String: Any] {
        var data: [String: Any] = [:]
        for (key, value) in content.userInfo {
            guard let key = key as? String, key != markerKey else { continue }
            data[key] = value
        }
        return data
    }

    private static func invalid(_ message: String) -> NSError {
        NSError(domain: "invalid_arguments", code: 0, userInfo: [NSLocalizedDescriptionKey: message])
    }
}
