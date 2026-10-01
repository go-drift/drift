/// DriftFirebasePlugin.swift
///
/// The Firebase plugin's native half on iOS, driven by DriftPlugins on the
/// main thread:
///   1. didFinishLaunching: configure Firebase from GoogleService-Info.plist,
///      become the Messaging delegate and register with APNs (no user
///      permission needed for a token; showing alerts needs it).
///   2. The APNs token goes to Messaging, which turns it into the FCM token
///      sent to Go on `drift/firebase/messaging/token`, at every launch and
///      on refresh.
///   3. Notifications carrying `gcm.message_id` are Firebase's: this plugin
///      claims them and sends them to Go, as `message` (received while
///      running) or `opened` (tapped). Others are left to Drift.
///
/// Firebase's swizzling is off (FirebaseAppDelegateProxyEnabled = NO);
/// every event arrives through the DriftPlugin hooks.

import DriftPluginAPI
import FirebaseCore
import FirebaseMessaging
import OSLog
import UIKit
import UserNotifications

private let firebaseLog = OSLog(subsystem: "drift.firebase", category: "messaging")

/// Public, with a public initializer: the app's generated registrant
/// creates it from outside this module (DriftPlugin_firebase).
public final class DriftFirebasePlugin: NSObject, DriftPlugin, MessagingDelegate {
    private static let tokenChannel = "drift/firebase/messaging/token"
    private static let messageChannel = "drift/firebase/messaging/message"
    private static let openedChannel = "drift/firebase/messaging/opened"

    private var host: DriftPluginHost?
    /// The last message sent to Go, so one delivered through two hooks (a
    /// notification with content-available, in the foreground) is sent once.
    private var lastMessageID: String?

    public override init() {
        super.init()
    }

    public func register(host: DriftPluginHost) {
        self.host = host
    }

    public func didFinishLaunching(_ application: UIApplication, options: [UIApplication.LaunchOptionsKey: Any]?) {
        FirebaseApp.configure()
        Messaging.messaging().delegate = self
        application.registerForRemoteNotifications()
    }

    public func didRegisterForRemoteNotifications(deviceToken: Data) {
        Messaging.messaging().apnsToken = deviceToken
    }

    public func didFailToRegisterForRemoteNotifications(error: Error) {
        os_log("APNs registration failed: %{public}@", log: firebaseLog, type: .error, error.localizedDescription)
    }

    public func didReceiveRemoteNotification(
        _ userInfo: [AnyHashable: Any],
        completion: @escaping (UIBackgroundFetchResult) -> Void
    ) -> Bool {
        guard let id = Self.messageID(userInfo) else { return false }
        Messaging.messaging().appDidReceiveMessage(userInfo)
        let alert = Self.alert(userInfo)
        sendMessage(id: id, title: alert.title, body: alert.body, userInfo: userInfo,
                    foreground: UIApplication.shared.applicationState == .active)
        completion(.newData)
        return true
    }

    public func willPresentNotification(_ notification: UNNotification) -> UNNotificationPresentationOptions? {
        let content = notification.request.content
        guard let id = Self.messageID(content.userInfo) else { return nil }
        Messaging.messaging().appDidReceiveMessage(content.userInfo)
        sendMessage(id: id, title: content.title, body: content.body, userInfo: content.userInfo, foreground: true)
        // The app decides what to show, as on Android.
        return []
    }

    public func didReceiveNotificationResponse(_ response: UNNotificationResponse) -> Bool {
        let content = response.notification.request.content
        guard let id = Self.messageID(content.userInfo) else { return false }
        Messaging.messaging().appDidReceiveMessage(content.userInfo)
        host?.sendEvent(Self.openedChannel, data: Self.payload(
            id: id, title: content.title, body: content.body, userInfo: content.userInfo, foreground: false))
        return true
    }

    // MARK: - MessagingDelegate

    public func messaging(_ messaging: Messaging, didReceiveRegistrationToken fcmToken: String?) {
        guard let token = fcmToken else { return }
        DispatchQueue.main.async { [self] in
            host?.sendEvent(Self.tokenChannel, data: ["token": token])
        }
    }

    // MARK: - Payloads

    private func sendMessage(id: String, title: String, body: String, userInfo: [AnyHashable: Any], foreground: Bool) {
        guard id != lastMessageID else { return }
        lastMessageID = id
        host?.sendEvent(Self.messageChannel, data: Self.payload(
            id: id, title: title, body: body, userInfo: userInfo, foreground: foreground))
    }

    /// The FCM message ID, present only in Firebase's notifications.
    private static func messageID(_ userInfo: [AnyHashable: Any]) -> String? {
        userInfo["gcm.message_id"] as? String
    }

    /// Title and body from the aps alert, for hooks without a UNNotification.
    private static func alert(_ userInfo: [AnyHashable: Any]) -> (title: String, body: String) {
        let aps = userInfo["aps"] as? [String: Any]
        if let alert = aps?["alert"] as? [String: Any] {
            return (alert["title"] as? String ?? "", alert["body"] as? String ?? "")
        }
        return ("", aps?["alert"] as? String ?? "")
    }

    /// The wire form of a message (see runtime/messaging): the sender's data
    /// keys only, without APNs' and Firebase's own.
    private static func payload(
        id: String, title: String, body: String, userInfo: [AnyHashable: Any], foreground: Bool
    ) -> [String: Any] {
        var data: [String: String] = [:]
        for (key, value) in userInfo {
            guard let key = key as? String,
                  key != "aps", key != "fcm_options",
                  !key.hasPrefix("gcm."), !key.hasPrefix("google.") else { continue }
            data[key] = value as? String ?? "\(value)"
        }
        return ["id": id, "title": title, "body": body, "data": data, "foreground": foreground]
    }
}
