/// PlatformChannel.swift
/// Provides platform channel communication between Swift and the Go Drift engine.
///
/// This file implements the native side of platform channels, enabling Go code
/// to call iOS APIs (clipboard, haptics, etc.) and receive events from iOS.

@_spi(DriftHost) import DriftPluginAPI
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
    _ errorJSON: UnsafePointer<CChar>
)

/// FFI declaration for notifying Go that an event stream has ended.
@_silgen_name("DriftPlatformHandleEventDone")
func DriftPlatformHandleEventDone(_ channel: UnsafePointer<CChar>)

/// FFI declaration for freeing Go-allocated memory.
@_silgen_name("DriftPlatformFree")
func DriftPlatformFree(_ ptr: UnsafeMutableRawPointer?)

/// FFI declaration for replying to a Go method call (see
/// `DriftNativeMethodHandler`). Go copies the bytes.
@_silgen_name("DriftPlatformReply")
func DriftPlatformReply(
    _ callID: Int64,
    _ data: UnsafeRawPointer?,
    _ dataLen: Int32,
    _ errorJSON: UnsafePointer<CChar>?
)

/// Type alias for the native method handler callback.
/// Must match the C typedef in bridge_platform.go.tmpl: Go calls it on the
/// calling thread, and the call is answered through DriftPlatformReply
/// exactly once, in place or later. Returns 0 once the call is taken.
typealias DriftNativeMethodHandler = @convention(c) (
    Int64,                 // callID
    UnsafePointer<CChar>,  // channel
    UnsafePointer<CChar>,  // method
    UnsafeRawPointer?,     // argsData
    Int32                  // argsLen
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
private let driftNativeMethodHandlerImpl: DriftNativeMethodHandler = { callID, channelPtr, methodPtr, argsPtr, argsLen in
    var argsData: Data? = nil
    if argsLen > 0, let ptr = argsPtr {
        argsData = Data(bytes: ptr, count: Int(argsLen))
    }
    PlatformChannelManager.shared.handleMethodCall(
        channel: String(cString: channelPtr),
        method: String(cString: methodPtr),
        argsData: argsData
    ) { outcome in
        switch outcome {
        case .success(let data):
            data.withUnsafeBytes { DriftPlatformReply(callID, $0.baseAddress, Int32(data.count), nil) }
        case .failure(let error):
            encodeErrorPayload(error).withCString { DriftPlatformReply(callID, nil, 0, $0) }
        }
    }
    return 0
}

// MARK: - JSON Helpers

/// Why a value cannot cross a platform channel: some part of it is not
/// JSON-like. `description` locates that part, as in `value.items[2]`.
struct JsonEncodeError: Error, CustomStringConvertible {
    let description: String
}

/// JSON codec for the values crossing a platform channel: nil, booleans,
/// finite numbers, strings, lists and string-keyed maps. Mirrors the
/// Android JsonCodec.
final class JsonCodec {
    /// Encodes a JSON-like value, or fails naming the first part that is not.
    func encode(_ value: Any?) -> Result<Data, JsonEncodeError> {
        let normalized: Any
        do {
            normalized = try normalize(value, at: "value")
        } catch {
            return .failure(error)
        }
        // normalize admits only what JSONSerialization writes, so this
        // cannot raise its uncatchable invalid-object exception.
        guard let data = try? JSONSerialization.data(withJSONObject: normalized, options: [.fragmentsAllowed]) else {
            return .failure(JsonEncodeError(description: "value could not be serialized"))
        }
        return .success(data)
    }

    func decode(_ data: Data) -> Any? {
        guard !data.isEmpty else { return nil }
        return try? JSONSerialization.jsonObject(with: data, options: [.fragmentsAllowed])
    }

    /// The JSONSerialization form of `value`, found at `path`.
    private func normalize(_ value: Any?, at path: String) throws(JsonEncodeError) -> Any {
        guard let value = value else { return NSNull() }
        switch value {
        case let optional as BoxedOptional:
            return try normalize(optional.unboxed, at: path)
        case is NSNull, is Void:
            return NSNull()
        case let number as NSNumber:
            // A Bool bridges to a CFBoolean. Any other NSNumber is a number,
            // even a 0 or 1 that `as Bool` would also match.
            if CFGetTypeID(number) == CFBooleanGetTypeID() {
                return number
            }
            guard number.doubleValue.isFinite else {
                throw JsonEncodeError(description: "\(path) is \(number), not a finite number")
            }
            return number
        case let string as String:
            return string
        case let array as [Any]:
            var result: [Any] = []
            result.reserveCapacity(array.count)
            for (index, item) in array.enumerated() {
                result.append(try normalize(item, at: "\(path)[\(index)]"))
            }
            return result
        case let dict as [AnyHashable: Any]:
            var result: [String: Any] = [:]
            for (key, item) in dict {
                let name = String(describing: key)
                result[name] = try normalize(item, at: "\(path).\(name)")
            }
            return result
        default:
            throw JsonEncodeError(
                description: "\(path) is a \(type(of: value)); values must be null, booleans, finite numbers, strings, lists or string-keyed maps"
            )
        }
    }
}

/// An Optional boxed in `Any`, such as a `String?` value in a dictionary
/// literal, which `normalize` unwraps.
private protocol BoxedOptional {
    var unboxed: Any? { get }
}

extension Optional: BoxedOptional {
    fileprivate var unboxed: Any? { map { $0 } }
}

// MARK: - Platform Channel Manager

/// Handler for a built-in channel: synchronous, run on the calling (Go)
/// thread. Plugins declare their methods on a `DriftChannel` instead.
typealias MethodHandler = (String, Any?) -> (Any?, Error?)

/// Manages platform channel handlers and dispatches calls between Go and iOS.
///
/// Adopts `DriftPluginHost` so third-party plugins can register channels and
/// send events without depending on host internals.
///
/// The initializer registers Drift's built-in channels only. Plugins
/// register later, from `DriftPlugins.launch`, once `shared` exists, so a
/// plugin touching `shared` while registering cannot re-enter this
/// initializer.
final class PlatformChannelManager: DriftPluginHost {
    static let shared = PlatformChannelManager()

    /// A registered channel: a built-in's handler runs on the calling
    /// thread, a plugin's declared methods on the main thread.
    private enum Channel {
        case builtIn(MethodHandler)
        case plugin(DriftChannel)
    }

    private var channels: [String: Channel] = [:]
    private let channelsLock = NSLock()
    private var eventObservers: [String: [UUID: (Any?) -> Void]] = [:]
    private let eventObserversLock = NSLock()
    private let codec = JsonCodec()

    private init() {
        registerBuiltInChannels()
    }

    /// Registers a built-in channel handler.
    func register(channel: String, handler: @escaping MethodHandler) {
        add(channel, .builtIn(handler))
    }

    /// `DriftPluginHost`: register a plugin channel and its methods.
    func registerChannel(_ name: String, _ declare: (DriftChannel) -> Void) {
        let channel = DriftChannel(name: name)
        declare(channel)
        channel.seal()
        add(name, .plugin(channel))
    }

    private func add(_ name: String, _ channel: Channel) {
        channelsLock.lock()
        defer { channelsLock.unlock() }
        if channels[name] != nil {
            fatalError(
                "drift: platform channel \"\(name)\" is already registered. " +
                    "Built-in channels run first; plugins must choose a unique " +
                    "<vendor>/<feature> namespace."
            )
        }
        channels[name] = channel
    }

    /// `DriftPluginHost`: register a platform view type.
    func registerPlatformView(_ type: String, _ factory: @escaping DriftPlatformViewFactory) {
        PlatformViewHandler.register(type, factory)
    }

    /// `DriftPluginHost`: register a permission kind.
    func registerPermission(_ kind: String, _ permission: DriftPermission) {
        PermissionHandler.register(kind, permission)
    }

    /// `DriftPluginHost`-shaped event sender. Wraps the existing labeled
    /// method so the dozens of in-tree callers (`sendEvent(channel:data:)`)
    /// continue to work without a sweeping rename.
    func sendEvent(_ channel: String, data: Any?) {
        sendEvent(channel: channel, data: data)
    }

    /// `DriftPluginHost`: fail Go's stream on channel with error, encoded
    /// as a failed call's error is (encodeErrorPayload).
    func sendEventError(_ channel: String, _ error: Error) {
        channel.withCString { channelPtr in
            encodeErrorPayload(error).withCString { errorPtr in
                DriftPlatformHandleEventError(channelPtr, errorPtr)
            }
        }
    }

    /// `DriftPluginHost`-shaped done sender. Mirrors `sendEvent(_:data:)`.
    func sendEventDone(_ channel: String) {
        sendEventDone(channel: channel)
    }

    /// Handles a method call from Go, answering through `reply` exactly
    /// once with the encoded result or an error.
    ///
    /// Go calls on its own thread. A plugin method runs on the main thread:
    /// in place when Go called from it, otherwise queued there with the
    /// reply coming later. The main thread is then blocked in Go until this
    /// returns, so a method that replies asynchronously is refused there
    /// before it runs: its reply could need the main thread.
    func handleMethodCall(
        channel: String,
        method: String,
        argsData: Data?,
        reply: @escaping (Result<Data, Error>) -> Void
    ) {
        let codec = self.codec
        let answer: (Result<Any?, Error>) -> Void = { outcome in
            reply(outcome.flatMap { value in
                codec.encode(value).mapError { error -> Error in
                    DriftCallError(Self.notEncodable, self.notEncodableMessage("the reply from \(channel).\(method)", error))
                }
            })
        }
        channelsLock.lock()
        let registered = channels[channel]
        channelsLock.unlock()
        guard let registered = registered else {
            answer(.failure(DriftCallError("channel_not_found", "channel not found: \(channel)")))
            return
        }

        var args: Any? = nil
        if let argsData = argsData, !argsData.isEmpty {
            args = codec.decode(argsData)
        }

        switch registered {
        case .builtIn(let handler):
            let (value, error) = handler(method, args)
            if let error = error {
                answer(.failure(error))
            } else {
                answer(.success(value))
            }
        case .plugin(let declared):
            switch declared.lookup(method) {
            case nil:
                answer(.failure(unknownMethod(channel, method)))
            case .sync(let handler)?:
                let run = { answer(Result(catching: { try handler(args) })) }
                if Thread.isMainThread {
                    run()
                } else {
                    DispatchQueue.main.async(execute: run)
                }
            case .async(let handler)?:
                guard !Thread.isMainThread else {
                    answer(.failure(DriftCallError(
                        "blocks_ui_thread",
                        "drift: \(channel).\(method) replies asynchronously; Go must call it from a goroutine, not the UI thread"
                    )))
                    return
                }
                DispatchQueue.main.async {
                    handler(args, DriftResult(call: "\(channel).\(method)", deliver: answer))
                }
            }
        }
    }

    /// The code Go receives when a reply or an event is not JSON-like.
    private static let notEncodable = "not_encodable"

    /// Why `what` (a reply or an event) cannot reach Go, logged.
    private func notEncodableMessage(_ what: String, _ error: JsonEncodeError) -> String {
        let message = "drift: \(what) is not JSON-like: \(error)"
        DriftLog.platform.error("\(message)")
        return message
    }

    /// Sends an event to Go listeners. Also fans out to any native-side
    /// observers registered via `observeEvent(_:handler:)`, so plugins can
    /// react to events originating from other native modules (e.g. the
    /// splash plugin reacting to `drift/rendering/frame_events` posted by
    /// `DriftRenderer`).
    func sendEvent(channel: String, data: Any?) {
        notifyEventObservers(channel: channel, data: data)
        switch codec.encode(data) {
        case .success(let encoded):
            encoded.withUnsafeBytes { ptr in
                channel.withCString { channelPtr in
                    DriftPlatformHandleEvent(channelPtr, ptr.baseAddress, Int32(encoded.count))
                }
            }
        case .failure(let error):
            let message = notEncodableMessage("an event on \(channel)", error)
            sendEventError(channel, DriftCallError(Self.notEncodable, message))
        }
    }

    /// Implements `DriftPluginHost.observeEvent`. Returns a
    /// `DriftSubscription` token; the token's `cancel()` unsubscribes.
    /// Multiple observers per channel are supported.
    ///
    /// Observers run asynchronously on the main thread, in the order events
    /// were sent.
    func observeEvent(_ channel: String, handler: @escaping (Any?) -> Void) -> DriftSubscription {
        let id = UUID()
        eventObserversLock.lock()
        if eventObservers[channel] == nil {
            eventObservers[channel] = [:]
        }
        eventObservers[channel]?[id] = handler
        eventObserversLock.unlock()
        return DriftSubscription { [weak self] in
            guard let self = self else { return }
            self.eventObserversLock.lock()
            self.eventObservers[channel]?.removeValue(forKey: id)
            if self.eventObservers[channel]?.isEmpty ?? false {
                self.eventObservers.removeValue(forKey: channel)
            }
            self.eventObserversLock.unlock()
        }
    }

    /// Delivers data to the channel's observers on the main thread, skipping
    /// any cancelled before delivery. Called directly for native-only
    /// events, which Go never sees.
    func notifyEventObservers(channel: String, data: Any?) {
        eventObserversLock.lock()
        let ids = eventObservers[channel].map { Array($0.keys) } ?? []
        eventObserversLock.unlock()
        guard !ids.isEmpty else { return }
        DispatchQueue.main.async { [self] in
            for id in ids {
                eventObserversLock.lock()
                let handler = eventObservers[channel]?[id]
                eventObserversLock.unlock()
                handler?(data)
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

        // The notification-center delegate, which offers notifications to
        // plugins
        NotificationHandler.start()

        // Deep links channel
        register(channel: "drift/deeplinks") { method, args in
            return DeepLinkHandler.handle(method: method, args: args)
        }

        // Platform Views channel
        register(channel: "drift/platform_views") { method, args in
            return PlatformViewHandler.handle(method: method, args: args)
        }
        PlatformViewHandler.registerBuiltIns(host: self)

        // Permissions channel and Drift's own permission kinds
        PermissionHandler.start(host: self)

        StorageHandler.start(host: self)

        // Accessibility channel
        register(channel: "drift/accessibility") { method, args in
            return AccessibilityHandler.handle(method: method, args: args)
        }

        // Preferences channel
        register(channel: "drift/preferences") { method, args in
            return PreferencesHandler.handle(method: method, args: args)
        }

        DatePickerHandler.start(host: self)
        TimePickerHandler.start(host: self)

        // URL Launcher channel
        register(channel: "drift/url_launcher") { method, args in
            return URLLauncherHandler.handle(method: method, args: args)
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
                return (nil, DriftCallError.invalidArguments("Missing text argument"))
            }
            UIPasteboard.general.string = text
            return (nil, nil)

        case "hasText":
            return (UIPasteboard.general.hasStrings, nil)

        case "clear":
            UIPasteboard.general.items = []
            return (nil, nil)

        default:
            return (nil, unknownMethod("drift/clipboard", method))
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
                return (nil, DriftCallError.invalidArguments("Missing style argument"))
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
            return (nil, unknownMethod("drift/haptics", method))
        }
    }
}

// MARK: - Share Handler

enum ShareHandler {
    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        guard method == "share" else {
            return (nil, unknownMethod("drift/share", method))
        }

        guard let dict = args as? [String: Any] else {
            return (nil, DriftCallError.invalidArguments("Invalid arguments"))
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

    static let `default` = SystemUIStyle(
        statusBarHidden: false,
        statusBarStyle: .default
    )
}

enum SystemUIHandler {
    static var currentStyle = SystemUIStyle.default

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        guard method == "setStyle" else {
            return (nil, unknownMethod("drift/system_ui", method))
        }

        guard let dict = args as? [String: Any] else {
            return (nil, DriftCallError.invalidArguments("Invalid arguments"))
        }

        let statusBarHidden = dict["statusBarHidden"] as? Bool ?? false
        let statusBarStyle = parseStatusBarStyle(dict["statusBarStyle"] as? String)

        let style = SystemUIStyle(
            statusBarHidden: statusBarHidden,
            statusBarStyle: statusBarStyle
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
            return .darkContent
        default:
            return .default
        }
    }
}

// MARK: - Notification Handler

/// The app's one UNUserNotificationCenter delegate. Every notification is
/// offered to plugins, which claim their own (drift.yaml order, on the main
/// thread, as for every plugin callback): local notifications to the
/// notifications plugin, push to the Firebase plugin. An unclaimed
/// notification is not shown in the foreground, as without a delegate.
final class NotificationHandler: NSObject, UNUserNotificationCenterDelegate {
    static let shared = NotificationHandler()
    private static var started = false

    static func start() {
        guard !started else { return }
        started = true
        UNUserNotificationCenter.current().delegate = shared
    }

    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        willPresent notification: UNNotification,
        withCompletionHandler completionHandler: @escaping (UNNotificationPresentationOptions) -> Void
    ) {
        DispatchQueue.main.async {
            completionHandler(DriftPlugins.shared.willPresentNotification(notification) ?? [])
        }
    }

    func userNotificationCenter(
        _ center: UNUserNotificationCenter,
        didReceive response: UNNotificationResponse,
        withCompletionHandler completionHandler: @escaping () -> Void
    ) {
        DispatchQueue.main.async {
            _ = DriftPlugins.shared.didReceiveNotificationResponse(response)
            completionHandler()
        }
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
            return (nil, unknownMethod("drift/deeplinks", method))
        }
    }

    static func handle(url: URL, source: String) {
        let urlString = url.absoluteString
        guard !urlString.isEmpty else { return }
        if lastLink == urlString {
            return
        }
        lastLink = urlString
        DriftLog.deeplink.log("Received deep link: \(urlString) (source=\(source))")
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
            return (nil, unknownMethod("drift/lifecycle", method))
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

// MARK: - URL Launcher Handler

enum URLLauncherHandler {
    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "openURL":
            guard let dict = args as? [String: Any],
                  let urlString = dict["url"] as? String,
                  let url = URL(string: urlString) else {
                return (nil, DriftCallError.invalidArguments("Missing or invalid url argument"))
            }
            DispatchQueue.main.async {
                UIApplication.shared.open(url)
            }
            return (nil, nil)

        case "canOpenURL":
            guard let dict = args as? [String: Any],
                  let urlString = dict["url"] as? String,
                  let url = URL(string: urlString) else {
                return (nil, DriftCallError.invalidArguments("Missing or invalid url argument"))
            }
            return (["canOpen": UIApplication.shared.canOpenURL(url)], nil)

        default:
            return (nil, unknownMethod("drift/url_launcher", method))
        }
    }
}

// MARK: - C Bridge Functions

/// The JSON error payload Go decodes into a ChannelError: a DriftCallError's
/// code, message and details; any other error as native_error, its NSError
/// domain and code in details. Details that are not JSON-like answer
/// not_encodable instead.
private func encodeErrorPayload(_ error: Error) -> String {
    var payload: [String: Any]
    if let callError = error as? DriftCallError {
        payload = ["code": callError.code, "message": callError.message]
        if let details = callError.details {
            payload["details"] = details
        }
    } else {
        let nsError = error as NSError
        payload = [
            "code": "native_error",
            "message": nsError.localizedDescription,
            "details": ["domain": nsError.domain, "code": nsError.code],
        ]
    }
    let codec = JsonCodec()
    switch codec.encode(payload) {
    case .success(let data):
        return String(decoding: data, as: UTF8.self)
    case .failure(let encodeError):
        let message = "drift: the details of error \(payload["code"] ?? "") are not JSON-like: \(encodeError)"
        DriftLog.platform.error("\(message)")
        guard case .success(let data) = codec.encode(["code": "not_encodable", "message": message]) else {
            preconditionFailure("drift: error payload of strings is not JSON-like")
        }
        return String(decoding: data, as: UTF8.self)
    }
}

/// The error a built-in channel answers for a method it does not have.
func unknownMethod(_ channel: String, _ method: String) -> DriftCallError {
    DriftCallError("method_not_found", "\(channel) has no method \(method)")
}
