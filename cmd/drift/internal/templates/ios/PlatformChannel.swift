/// PlatformChannel.swift
/// Provides platform channel communication between Swift and the Go Drift engine.
///
/// This file implements the native side of platform channels, enabling Go code
/// to call iOS APIs (clipboard, haptics, etc.) and receive events from iOS.

import UIKit
import AudioToolbox
import UserNotifications

// MARK: - FFI Declarations

/// FFI declaration for handling method call results from Go.
@_silgen_name("DriftPlatformHandleMethodCall")
func DriftPlatformHandleMethodCall(
    _ channel: UnsafePointer<CChar>,
    _ method: UnsafePointer<CChar>,
    _ args: UnsafeRawPointer?,
    _ argsLen: Int32,
    _ result: UnsafeMutablePointer<UnsafeMutableRawPointer?>,
    _ resultLen: UnsafeMutablePointer<Int32>,
    _ error: UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>
) -> Int32

/// FFI declaration for sending events to Go.
@_silgen_name("DriftPlatformHandleEvent")
func DriftPlatformHandleEvent(
    _ channel: UnsafePointer<CChar>,
    _ data: UnsafeRawPointer?,
    _ dataLen: Int32
)

/// FFI declaration for sending event errors to Go.
@_silgen_name("DriftPlatformHandleEventError")
func DriftPlatformHandleEventError(
    _ channel: UnsafePointer<CChar>,
    _ code: UnsafePointer<CChar>,
    _ message: UnsafePointer<CChar>
)

/// FFI declaration for notifying Go that an event stream has ended.
@_silgen_name("DriftPlatformHandleEventDone")
func DriftPlatformHandleEventDone(_ channel: UnsafePointer<CChar>)

/// FFI declaration for checking if Go is listening to an event channel.
@_silgen_name("DriftPlatformIsStreamActive")
func DriftPlatformIsStreamActive(_ channel: UnsafePointer<CChar>) -> Int32

/// FFI declaration for freeing Go-allocated memory.
@_silgen_name("DriftPlatformFree")
func DriftPlatformFree(_ ptr: UnsafeMutableRawPointer?)

/// Type alias for the native method handler callback.
/// Must match the C typedef in bridge_platform.go.tmpl.
typealias DriftNativeMethodHandler = @convention(c) (
    UnsafePointer<CChar>,  // channel
    UnsafePointer<CChar>,  // method
    UnsafeRawPointer?,     // argsData
    Int32,                 // argsLen
    UnsafeMutablePointer<UnsafeMutableRawPointer?>,  // resultData
    UnsafeMutablePointer<Int32>,                     // resultLen
    UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>  // errorMsg
) -> Int32

/// FFI declaration for registering the native method handler with Go.
@_silgen_name("DriftPlatformSetNativeHandler")
func DriftPlatformSetNativeHandler(_ handler: DriftNativeMethodHandler?)

/// Registers the native method handler with the Go engine.
/// Must be called during app initialization before any platform channel calls.
func DriftPlatformRegisterHandler() {
    DriftPlatformSetNativeHandler(driftNativeMethodHandlerImpl)
}

/// The native method handler implementation that bridges Go calls to Swift.
/// This is a C-convention function that can be passed as a function pointer.
private let driftNativeMethodHandlerImpl: DriftNativeMethodHandler = { channelPtr, methodPtr, argsPtr, argsLen, resultPtr, resultLen, errorPtr in
    let channel = String(cString: channelPtr)
    let method = String(cString: methodPtr)

    var argsData: Data? = nil
    if argsLen > 0, let ptr = argsPtr {
        argsData = Data(bytes: ptr, count: Int(argsLen))
    }

    let (result, error) = PlatformChannelManager.shared.handleMethodCall(
        channel: channel,
        method: method,
        argsData: argsData
    )

    if let error = error {
        let errStr = encodeErrorPayload(error)
        errorPtr.pointee = strdup(errStr)
        return 1
    }

    if let result = result {
        let ptr = UnsafeMutableRawPointer.allocate(byteCount: result.count, alignment: 1)
        result.copyBytes(to: ptr.assumingMemoryBound(to: UInt8.self), count: result.count)
        resultPtr.pointee = ptr
        resultLen.pointee = Int32(result.count)
    }

    return 0
}

// MARK: - JSON Helpers

/// Simple JSON codec for basic types.
final class JsonCodec {
    func encode(_ value: Any?) -> Data {
        let normalized = normalize(value)
        // JSONSerialization requires top-level array/dict, so wrap primitives
        if JSONSerialization.isValidJSONObject(normalized) {
            return (try? JSONSerialization.data(withJSONObject: normalized, options: [])) ?? Data()
        }
        // For primitives, wrap in array then strip brackets
        let wrapper = [normalized]
        guard let data = try? JSONSerialization.data(withJSONObject: wrapper, options: []),
              data.count >= 2 else {
            return Data()
        }
        return data.subdata(in: 1..<data.count - 1)
    }

    func decode(_ data: Data) -> Any? {
        guard !data.isEmpty else { return nil }
        return try? JSONSerialization.jsonObject(with: data, options: [.fragmentsAllowed])
    }

    private func normalize(_ value: Any?) -> Any {
        switch value {
        case nil:
            return NSNull()
        case let bool as Bool:
            return bool
        case let number as NSNumber:
            return number
        case let string as String:
            return string
        case let array as [Any]:
            return array.map { normalize($0) }
        case let dict as [String: Any]:
            return dict.mapValues { normalize($0) }
        case let dict as [AnyHashable: Any]:
            var result: [String: Any] = [:]
            for (key, val) in dict {
                result[String(describing: key)] = normalize(val)
            }
            return result
        default:
            return NSNull()
        }
    }
}

// MARK: - Platform Channel Manager

/// Manages platform channel handlers and dispatches calls between Go and iOS.
final class PlatformChannelManager {
    static let shared = PlatformChannelManager()

    private var handlers: [String: MethodHandler] = [:]
    private let codec = JsonCodec()

    typealias MethodHandler = (String, Any?) -> (Any?, Error?)

    private init() {
        registerBuiltInChannels()
    }

    /// Registers a handler for a platform channel.
    func register(channel: String, handler: @escaping MethodHandler) {
        handlers[channel] = handler
    }

    /// Handles a method call from Go and returns the result.
    func handleMethodCall(channel: String, method: String, argsData: Data?) -> (Data?, Error?) {
        guard let handler = handlers[channel] else {
            return (nil, NSError(domain: "PlatformChannel", code: 404, userInfo: [NSLocalizedDescriptionKey: "Channel not found: \(channel)"]))
        }

        var args: Any? = nil
        if let argsData = argsData, !argsData.isEmpty {
            args = codec.decode(argsData)
        }

        let (result, error) = handler(method, args)

        if let error = error {
            return (nil, error)
        }

        let resultData = codec.encode(result)
        return (resultData, nil)
    }

    /// Sends an event to Go listeners.
    func sendEvent(channel: String, data: Any?) {
        let encoded = codec.encode(data)
        encoded.withUnsafeBytes { ptr in
            channel.withCString { channelPtr in
                DriftPlatformHandleEvent(channelPtr, ptr.baseAddress, Int32(encoded.count))
            }
        }
    }

    /// Sends an error to Go event listeners.
    func sendEventError(channel: String, code: String, message: String) {
        channel.withCString { channelPtr in
            code.withCString { codePtr in
                message.withCString { messagePtr in
                    DriftPlatformHandleEventError(channelPtr, codePtr, messagePtr)
                }
            }
        }
    }

    /// Notifies Go that an event stream has ended.
    func sendEventDone(channel: String) {
        channel.withCString { channelPtr in
            DriftPlatformHandleEventDone(channelPtr)
        }
    }

    // MARK: - Built-in Channels

    private func registerBuiltInChannels() {
        // Clipboard channel
        register(channel: "drift/clipboard") { method, args in
            return ClipboardHandler.handle(method: method, args: args)
        }

        // Haptics channel
        register(channel: "drift/haptics") { method, args in
            return HapticsHandler.handle(method: method, args: args)
        }

        // Share channel
        register(channel: "drift/share") { method, args in
            return ShareHandler.handle(method: method, args: args)
        }

        // Lifecycle channel
        register(channel: "drift/lifecycle") { method, args in
            return LifecycleHandler.handle(method: method, args: args)
        }

        // System UI channel
        register(channel: "drift/system_ui") { method, args in
            return SystemUIHandler.handle(method: method, args: args)
        }

        // Notifications channel
        NotificationHandler.start()
        register(channel: "drift/notifications") { method, args in
            return NotificationHandler.handle(method: method, args: args)
        }

        // Deep links channel
        register(channel: "drift/deeplinks") { method, args in
            return DeepLinkHandler.handle(method: method, args: args)
        }

        // Platform Views channel
        register(channel: "drift/platform_views") { method, args in
            return PlatformViewHandler.handle(method: method, args: args)
        }

        // Permissions channel
        register(channel: "drift/permissions") { method, args in
            return PermissionHandler.handle(method: method, args: args)
        }

        // Location channel
        register(channel: "drift/location") { method, args in
            return LocationHandler.handle(method: method, args: args)
        }

        // Storage channel
        register(channel: "drift/storage") { method, args in
            return StorageHandler.handle(method: method, args: args)
        }

        // Camera channel
        register(channel: "drift/camera") { method, args in
            return CameraHandler.handle(method: method, args: args)
        }

        // Push channel
        register(channel: "drift/push") { method, args in
            return PushHandler.handle(method: method, args: args)
        }

        // Background tasks channel
        register(channel: "drift/background") { method, args in
            return BackgroundHandler.handle(method: method, args: args)
        }

        // Accessibility channel
        register(channel: "drift/accessibility") { method, args in
            return AccessibilityHandler.handle(method: method, args: args)
        }

        // Secure Storage channel
        register(channel: "drift/secure_storage") { method, args in
            return SecureStorageHandler.handle(method: method, args: args)
        }

        // Date Picker channel
        register(channel: "drift/date_picker") { method, args in
            return DatePickerHandler.handle(method: method, args: args)
        }

        // Time Picker channel
        register(channel: "drift/time_picker") { method, args in
            return TimePickerHandler.handle(method: method, args: args)
        }

        // Audio Player channel
        register(channel: "drift/audio_player") { method, args in
            return AudioPlayerHandler.handle(method: method, args: args)
        }
    }
}

// MARK: - Clipboard Handler

enum ClipboardHandler {
    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "getText":
            let text = UIPasteboard.general.string ?? ""
            return (["text": text], nil)

        case "setText":
            guard let dict = args as? [String: Any],
                  let text = dict["text"] as? String else {
                return (nil, NSError(domain: "Clipboard", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing text argument"]))
            }
            UIPasteboard.general.string = text
            return (nil, nil)

        case "hasText":
            return (UIPasteboard.general.hasStrings, nil)

        case "clear":
            UIPasteboard.general.items = []
            return (nil, nil)

        default:
            return (nil, NSError(domain: "Clipboard", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }
}

// MARK: - Haptics Handler

enum HapticsHandler {
    private static let lightGenerator = UIImpactFeedbackGenerator(style: .light)
    private static let mediumGenerator = UIImpactFeedbackGenerator(style: .medium)
    private static let heavyGenerator = UIImpactFeedbackGenerator(style: .heavy)
    private static let selectionGenerator = UISelectionFeedbackGenerator()
    private static let notificationGenerator = UINotificationFeedbackGenerator()

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "impact":
            guard let dict = args as? [String: Any],
                  let style = dict["style"] as? String else {
                return (nil, NSError(domain: "Haptics", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing style argument"]))
            }

            switch style {
            case "light":
                lightGenerator.impactOccurred()
            case "medium":
                mediumGenerator.impactOccurred()
            case "heavy":
                heavyGenerator.impactOccurred()
            case "selection":
                selectionGenerator.selectionChanged()
            case "success":
                notificationGenerator.notificationOccurred(.success)
            case "warning":
                notificationGenerator.notificationOccurred(.warning)
            case "error":
                notificationGenerator.notificationOccurred(.error)
            default:
                mediumGenerator.impactOccurred()
            }
            return (nil, nil)

        case "vibrate":
            // iOS doesn't have a general vibrate API like Android
            // Use AudioServicesPlaySystemSound for a simple vibration
            AudioServicesPlaySystemSound(kSystemSoundID_Vibrate)
            return (nil, nil)

        default:
            return (nil, NSError(domain: "Haptics", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }
}

// MARK: - Share Handler

enum ShareHandler {
    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        guard method == "share" else {
            return (nil, NSError(domain: "Share", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }

        guard let dict = args as? [String: Any] else {
            return (nil, NSError(domain: "Share", code: 400, userInfo: [NSLocalizedDescriptionKey: "Invalid arguments"]))
        }

        var items: [Any] = []

        if let text = dict["text"] as? String {
            items.append(text)
        }

        if let urlString = dict["url"] as? String, let url = URL(string: urlString) {
            items.append(url)
        }

        if let filePath = dict["file"] as? String {
            let url = URL(fileURLWithPath: filePath)
            items.append(url)
        }

        if let files = dict["files"] as? [[String: Any]] {
            for file in files {
                if let path = file["path"] as? String {
                    items.append(URL(fileURLWithPath: path))
                }
            }
        }

        guard !items.isEmpty else {
            return (["result": "unavailable"], nil)
        }

        // Share must be done on main thread
        DispatchQueue.main.async {
            let controller = UIActivityViewController(activityItems: items, applicationActivities: nil)

            if let subject = dict["subject"] as? String {
                controller.setValue(subject, forKey: "subject")
            }

            // Find the top view controller to present from
            if let windowScene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
               let rootVC = windowScene.windows.first?.rootViewController {
                var topVC = rootVC
                while let presented = topVC.presentedViewController {
                    topVC = presented
                }

                // For iPad, we need to specify a source
                if let popover = controller.popoverPresentationController {
                    popover.sourceView = topVC.view
                    popover.sourceRect = CGRect(x: topVC.view.bounds.midX, y: topVC.view.bounds.midY, width: 0, height: 0)
                }

                topVC.present(controller, animated: true)
            }
        }

        return (["result": "success"], nil)
    }
}

// MARK: - System UI Handler

struct SystemUIStyle {
    var statusBarHidden: Bool
    var statusBarStyle: UIStatusBarStyle
    var transparent: Bool
    var backgroundColor: UIColor?

    static let `default` = SystemUIStyle(
        statusBarHidden: false,
        statusBarStyle: .default,
        transparent: false,
        backgroundColor: nil
    )
}

enum SystemUIHandler {
    static var currentStyle = SystemUIStyle.default

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        guard method == "setStyle" else {
            return (nil, NSError(domain: "SystemUI", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }

        guard let dict = args as? [String: Any] else {
            return (nil, NSError(domain: "SystemUI", code: 400, userInfo: [NSLocalizedDescriptionKey: "Invalid arguments"]))
        }

        let statusBarHidden = dict["statusBarHidden"] as? Bool ?? false
        let statusBarStyle = parseStatusBarStyle(dict["statusBarStyle"] as? String)
        let transparent = dict["transparent"] as? Bool ?? false
        let backgroundColor = parseColor(dict["backgroundColor"])

        let style = SystemUIStyle(
            statusBarHidden: statusBarHidden,
            statusBarStyle: statusBarStyle,
            transparent: transparent,
            backgroundColor: backgroundColor
        )

        apply(style)
        return (nil, nil)
    }

    static func apply(_ style: SystemUIStyle) {
        currentStyle = style
        DispatchQueue.main.async {
            applyToActiveController(style)
        }
    }

    private static func applyToActiveController(_ style: SystemUIStyle) {
        // Find the DriftViewController and tell it to update status bar appearance.
        // The style is already stored in currentStyle, which the controller reads
        // via prefersStatusBarHidden and preferredStatusBarStyle.
        if let controller = activeDriftController() {
            controller.applySystemUIStyle(style)
        }
    }

    private static func activeDriftController() -> DriftViewController? {
        guard let window = activeWindow() else { return nil }
        return findDriftController(from: window.rootViewController)
    }

    private static func findDriftController(from vc: UIViewController?) -> DriftViewController? {
        guard let vc = vc else { return nil }

        // Direct match
        if let drift = vc as? DriftViewController {
            return drift
        }

        // Check presented view controller
        if let presented = vc.presentedViewController,
           let drift = findDriftController(from: presented) {
            return drift
        }

        // Check children (needed for UIHostingController with UIViewControllerRepresentable)
        for child in vc.children {
            if let drift = findDriftController(from: child) {
                return drift
            }
        }

        // Check navigation controller
        if let nav = vc as? UINavigationController {
            return findDriftController(from: nav.visibleViewController)
        }

        return nil
    }

    private static func activeWindow() -> UIWindow? {
        return UIApplication.shared.connectedScenes
            .compactMap { $0 as? UIWindowScene }
            .first?.windows.first
    }

    private static func parseStatusBarStyle(_ value: String?) -> UIStatusBarStyle {
        switch value {
        case "light":
            return .lightContent
        case "dark":
            if #available(iOS 13.0, *) {
                return .darkContent
            }
            return .default
        default:
            return .default
        }
    }

    private static func parseColor(_ value: Any?) -> UIColor? {
        guard let number = value as? NSNumber else { return nil }
        let argb = UInt32(truncating: number)
        let a = CGFloat((argb >> 24) & 0xFF) / 255.0
        let r = CGFloat((argb >> 16) & 0xFF) / 255.0
        let g = CGFloat((argb >> 8) & 0xFF) / 255.0
        let b = CGFloat(argb & 0xFF) / 255.0
        return UIColor(red: r, green: g, blue: b, alpha: a)
    }
}

// MARK: - Notification Handler

final class NotificationHandler: NSObject, UNUserNotificationCenterDelegate {
    static let shared = NotificationHandler()
    private static let center = UNUserNotificationCenter.current()
    private static var started = false

    static func start() {
        guard !started else { return }
        started = true
        center.delegate = shared
    }

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "getSettings":
            return getSettings()
        case "schedule":
            return schedule(args: args)
        case "cancel":
            return cancel(args: args)
        case "cancelAll":
            return cancelAll()
        case "setBadge":
            return setBadge(args: args)
        default:
            return (nil, NSError(domain: "Notifications", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    private static func getSettings() -> (Any?, Error?) {
        var result: [String: Any] = ["status": "unknown"]
        let semaphore = DispatchSemaphore(value: 0)
        center.getNotificationSettings { settings in
            result["status"] = authorizationStatus(settings)
            result["alertsEnabled"] = settings.alertSetting == .enabled
            result["soundsEnabled"] = settings.soundSetting == .enabled
            result["badgesEnabled"] = settings.badgeSetting == .enabled
            semaphore.signal()
        }
        semaphore.wait()
        return (result, nil)
    }

    private static func schedule(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let id = dict["id"] as? String else {
            return (nil, NSError(domain: "Notifications", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing id argument"]))
        }

        let content = UNMutableNotificationContent()
        content.title = dict["title"] as? String ?? ""
        content.body = dict["body"] as? String ?? ""

        if let badge = dict["badge"] as? Int {
            content.badge = NSNumber(value: badge)
        } else if let badgeNumber = dict["badge"] as? NSNumber {
            content.badge = badgeNumber
        }

        if let soundName = dict["sound"] as? String, !soundName.isEmpty {
            if soundName == "default" {
                content.sound = .default
            } else {
                content.sound = UNNotificationSound(named: UNNotificationSoundName(soundName))
            }
        } else {
            content.sound = .default
        }

        if let data = dict["data"] as? [String: Any] {
            content.userInfo = data
        }

        let repeats = dict["repeats"] as? Bool ?? false
        var trigger: UNNotificationTrigger

        if let interval = dict["intervalSeconds"] as? NSNumber, interval.doubleValue > 0 {
            trigger = UNTimeIntervalNotificationTrigger(timeInterval: interval.doubleValue, repeats: repeats)
        } else if let atMillis = dict["at"] as? NSNumber {
            let scheduledDate = Date(timeIntervalSince1970: atMillis.doubleValue / 1000.0)
            let timeInterval = scheduledDate.timeIntervalSinceNow
            if timeInterval > 0 {
                trigger = UNTimeIntervalNotificationTrigger(timeInterval: timeInterval, repeats: repeats)
            } else {
                trigger = UNTimeIntervalNotificationTrigger(timeInterval: 1, repeats: false)
            }
        } else {
            trigger = UNTimeIntervalNotificationTrigger(timeInterval: 1, repeats: false)
        }

        let request = UNNotificationRequest(identifier: id, content: content, trigger: trigger)
        center.add(request)
        return (nil, nil)
    }

    private static func cancel(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let id = dict["id"] as? String else {
            return (nil, NSError(domain: "Notifications", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing id argument"]))
        }
        center.removePendingNotificationRequests(withIdentifiers: [id])
        center.removeDeliveredNotifications(withIdentifiers: [id])
        return (nil, nil)
    }

    private static func cancelAll() -> (Any?, Error?) {
        center.removeAllPendingNotificationRequests()
        center.removeAllDeliveredNotifications()
        return (nil, nil)
    }

    private static func setBadge(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any] else {
            return (nil, NSError(domain: "Notifications", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing count argument"]))
        }
        let count: Int
        if let intCount = dict["count"] as? Int {
            count = intCount
        } else if let numberCount = dict["count"] as? NSNumber {
            count = numberCount.intValue
        } else {
            return (nil, NSError(domain: "Notifications", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing count argument"]))
        }
        DispatchQueue.main.async {
            UIApplication.shared.applicationIconBadgeNumber = count
        }
        return (nil, nil)
    }

    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        let source = notification.request.trigger is UNPushNotificationTrigger ? "remote" : "local"
        NotificationHandler.sendReceived(notification: notification, isForeground: true, source: source)

        if #available(iOS 14.0, *) {
            completionHandler([.banner, .list, .sound, .badge])
        } else {
            completionHandler([.alert, .sound, .badge])
        }
    }

    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        let notification = response.notification
        let source = notification.request.trigger is UNPushNotificationTrigger ? "remote" : "local"
        NotificationHandler.sendOpened(notification: notification, action: response.actionIdentifier, source: source)
        completionHandler()
    }

    static func handleRemoteNotification(_ userInfo: [AnyHashable: Any], isForeground: Bool) {
        let payload = parsePayload(userInfo: userInfo)
        sendReceived(id: payload.id, title: payload.title, body: payload.body, data: payload.data, isForeground: isForeground, source: "remote")
    }

    static func handleDeviceToken(_ deviceToken: Data) {
        let token = deviceToken.map { String(format: "%02x", $0) }.joined()
        PlatformChannelManager.shared.sendEvent(channel: "drift/notifications/token", data: [
            "platform": "ios",
            "token": token,
            "timestamp": currentTimestamp(),
            "isRefresh": true
        ])
    }

    static func handleRemoteNotificationError(_ error: Error) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/notifications/error", data: [
            "code": "registration_failed",
            "message": error.localizedDescription,
            "platform": "ios"
        ])
    }

    private static func sendReceived(notification: UNNotification, isForeground: Bool, source: String) {
        let content = notification.request.content
        sendReceived(
            id: notification.request.identifier,
            title: content.title,
            body: content.body,
            data: content.userInfo as? [String: Any] ?? [:],
            isForeground: isForeground,
            source: source
        )
    }

    private static func sendReceived(id: String, title: String, body: String, data: [String: Any], isForeground: Bool, source: String) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/notifications/received", data: [
            "id": id,
            "title": title,
            "body": body,
            "data": data,
            "timestamp": currentTimestamp(),
            "isForeground": isForeground,
            "source": source
        ])
    }

    private static func sendOpened(notification: UNNotification, action: String, source: String) {
        let content = notification.request.content
        sendOpened(
            id: notification.request.identifier,
            data: content.userInfo as? [String: Any] ?? [:],
            action: action,
            source: source
        )
    }

    private static func sendOpened(id: String, data: [String: Any], action: String, source: String) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/notifications/opened", data: [
            "id": id,
            "data": data,
            "action": action,
            "source": source,
            "timestamp": currentTimestamp()
        ])
    }

    private static func currentTimestamp() -> Int64 {
        Int64(Date().timeIntervalSince1970 * 1000)
    }

    private static func authorizationStatus(_ settings: UNNotificationSettings) -> String {
        switch settings.authorizationStatus {
        case .authorized:
            return "granted"
        case .provisional:
            return "provisional"
        case .denied:
            return "denied"
        case .notDetermined:
            return "not_determined"
        case .ephemeral:
            return "provisional"
        @unknown default:
            return "unknown"
        }
    }

    private static func authorizationStatusSync() -> String {
        let semaphore = DispatchSemaphore(value: 0)
        var status = "unknown"
        center.getNotificationSettings { settings in
            status = authorizationStatus(settings)
            semaphore.signal()
        }
        semaphore.wait()
        return status
    }

    private static func notificationsEnabledSync() -> Bool {
        let semaphore = DispatchSemaphore(value: 0)
        var enabled = false
        center.getNotificationSettings { settings in
            enabled = settings.authorizationStatus == .authorized || settings.authorizationStatus == .provisional
            semaphore.signal()
        }
        semaphore.wait()
        return enabled
    }

    private static func parsePayload(userInfo: [AnyHashable: Any]) -> (id: String, title: String, body: String, data: [String: Any]) {
        var title = ""
        var body = ""
        if let aps = userInfo["aps"] as? [String: Any] {
            if let alert = aps["alert"] as? [String: Any] {
                title = alert["title"] as? String ?? ""
                body = alert["body"] as? String ?? ""
            } else if let alertString = aps["alert"] as? String {
                body = alertString
            }
        }
        var data: [String: Any] = [:]
        for (key, value) in userInfo {
            if let keyString = key as? String {
                data[keyString] = value
            }
        }
        let id = data["id"] as? String ?? UUID().uuidString
        return (id: id, title: title, body: body, data: data)
    }
}

// MARK: - Deep Link Handler

enum DeepLinkHandler {
    private static var initialLink: [String: Any]? = nil
    private static var lastLink: String? = nil

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "getInitial":
            let link = initialLink
            initialLink = nil
            return (link, nil)
        default:
            return (nil, NSError(domain: "DeepLink", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    static func handle(url: URL, source: String) {
        let urlString = url.absoluteString
        guard !urlString.isEmpty else { return }
        if lastLink == urlString {
            return
        }
        lastLink = urlString
        DriftLog.deeplink.info("Received deep link: \(urlString) (source=\(source))")
        let payload: [String: Any] = [
            "url": urlString,
            "source": source,
            "timestamp": Int(Date().timeIntervalSince1970 * 1000)
        ]
        if initialLink == nil {
            initialLink = payload
        }
        PlatformChannelManager.shared.sendEvent(channel: "drift/deeplinks/events", data: payload)
    }
}

// MARK: - Lifecycle Handler

enum LifecycleHandler {
    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "getState":
            let state: String
            switch UIApplication.shared.applicationState {
            case .active:
                state = "resumed"
            case .inactive:
                state = "inactive"
            case .background:
                state = "paused"
            @unknown default:
                state = "detached"
            }
            return (["state": state], nil)

        default:
            return (nil, NSError(domain: "Lifecycle", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    /// Called from AppDelegate to notify Go of lifecycle changes.
    static func notifyStateChange(_ state: String) {
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/lifecycle/events",
            data: ["state": state]
        )
    }
}

// MARK: - Safe Area Handler

enum SafeAreaHandler {
    static func sendInsetsUpdate() {
        guard let windowScene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
              let window = windowScene.windows.first else {
            return
        }
        let insets = window.safeAreaInsets
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/safe_area/events",
            data: [
                "top": Double(insets.top),
                "bottom": Double(insets.bottom),
                "left": Double(insets.left),
                "right": Double(insets.right)
            ]
        )
    }
}

// MARK: - C Bridge Functions

private func encodeErrorPayload(_ error: Error) -> String {
    let nsError = error as NSError
    var payload: [String: Any] = [
        "code": nsError.domain.isEmpty ? "native_error" : nsError.domain,
        "message": nsError.localizedDescription
    ]
    var details: [String: Any] = [:]
    if !nsError.domain.isEmpty {
        details["domain"] = nsError.domain
    }
    if nsError.code != 0 {
        details["code"] = nsError.code
    }
    if !details.isEmpty {
        payload["details"] = details
    }
    let codec = JsonCodec()
    let data = codec.encode(payload)
    return String(data: data, encoding: .utf8) ?? nsError.localizedDescription
}
