/// PushHandler.swift
/// Handles push notification registration and token management for the Drift platform channel.

import UIKit
import UserNotifications

enum PushHandler {
    private static var currentToken: String?
    private static var subscribedTopics: Set<String> = []

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "register":
            return register()
        case "getToken":
            return getToken()
        case "subscribeToTopic":
            return subscribeToTopic(args: args)
        case "unsubscribeFromTopic":
            return unsubscribeFromTopic(args: args)
        case "deleteToken":
            return deleteToken()
        default:
            return (nil, NSError(domain: "Push", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    private static func register() -> (Any?, Error?) {
        // First request notification authorization, then register for remote notifications
        UNUserNotificationCenter.current().requestAuthorization(options: [.alert, .sound, .badge]) { granted, error in
            if let error = error {
                sendError("authorization_failed", message: error.localizedDescription)
                return
            }

            if granted {
                DispatchQueue.main.async {
                    UIApplication.shared.registerForRemoteNotifications()
                }
            } else {
                sendError("authorization_denied", message: "User denied notification permission")
            }
        }
        return (nil, nil)
    }

    private static func getToken() -> (Any?, Error?) {
        return (["token": currentToken], nil)
    }

    private static func subscribeToTopic(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let topic = dict["topic"] as? String else {
            return (nil, NSError(domain: "Push", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing topic"]))
        }

        // iOS doesn't have native topic support like FCM
        // Topics are typically managed server-side with APNs
        // Store locally for reference
        subscribedTopics.insert(topic)

        // In a real implementation, you would send this to your backend
        // which would then manage the topic subscription with your push service

        return (nil, nil)
    }

    private static func unsubscribeFromTopic(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let topic = dict["topic"] as? String else {
            return (nil, NSError(domain: "Push", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing topic"]))
        }

        subscribedTopics.remove(topic)

        // In a real implementation, you would send this to your backend

        return (nil, nil)
    }

    private static func deleteToken() -> (Any?, Error?) {
        DispatchQueue.main.async {
            UIApplication.shared.unregisterForRemoteNotifications()
        }
        currentToken = nil
        return (nil, nil)
    }

    // MARK: - AppDelegate Integration

    static func handleDeviceToken(_ deviceToken: Data) {
        let token = deviceToken.map { String(format: "%02x", $0) }.joined()
        currentToken = token

        PlatformChannelManager.shared.sendEvent(channel: "drift/push/token", data: [
            "platform": "ios",
            "token": token,
            "timestamp": Int64(Date().timeIntervalSince1970 * 1000)
        ])
    }

    static func handleRegistrationError(_ error: Error) {
        sendError("registration_failed", message: error.localizedDescription)
    }

    private static func sendError(_ code: String, message: String) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/push/error", data: [
            "code": code,
            "message": message
        ])
    }
}
