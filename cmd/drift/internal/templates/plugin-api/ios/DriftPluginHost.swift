/// DriftPluginHost.swift
///
/// Stable host API consumed by Drift plugin Swift sources. Plugin sources
/// reference this protocol; the templated PlatformChannelManager adopts it.
///
/// The DriftPluginAPI module (this file and DriftPlugin.swift, in the
/// generated Drift/Plugins package) is the whole surface plugin sources
/// compile against; it depends only on Foundation and UIKit. The app's
/// host side constructs the types below through the DriftHost SPI.
///
/// Threading: every plugin callback (method handlers, event observers and
/// the DriftPlugin lifecycle) runs on the main thread.

import Foundation

/// A plugin channel's methods, declared inside
/// `DriftPluginHost.registerChannel`. Every method runs on the main thread.
///
/// A method replies in one of two declared ways:
/// - `method(_:_:)` replies by returning. Go may call it from any goroutine,
///   including the UI thread.
/// - `asyncMethod(_:_:)` replies later, through a `DriftResult`, from any
///   thread (an SDK completion handler, a dialog). Go must call it from a
///   goroutine: a call from the UI thread fails with `blocks_ui_thread`
///   every time, before the handler runs, since the UI thread cannot wait
///   for a reply that needs it.
///
/// The channel's events (`DriftPluginHost.sendEvent` with its name) can
/// start and stop with Go's interest: see `stream(onListen:onCancel:)`.
public final class DriftChannel {
    @_spi(DriftHost) public enum Method {
        case sync((_ args: Any?) throws -> Any?)
        case async((_ args: Any?, _ result: DriftResult) -> Void)
    }

    /// The handlers `stream(onListen:onCancel:)` declared.
    @_spi(DriftHost) public struct Stream {
        public let onListen: () throws -> Void
        public let onCancel: () -> Void
    }

    private let name: String
    private var methods: [String: Method] = [:]
    @_spi(DriftHost) public private(set) var declaredStream: Stream?
    // Set by the host once the declare closure returns; the methods are
    // then read from Go threads and must not change.
    private var sealed = false

    @_spi(DriftHost) public init(name: String) {
        self.name = name
    }

    /// Declares `method` as replying by returning: the return value
    /// (JSON-like, or nil) is the reply, and a thrown error fails the call.
    public func method(_ method: String, _ handler: @escaping (_ args: Any?) throws -> Any?) {
        add(method, .sync(handler))
    }

    /// Declares `method` as replying later through its `DriftResult`,
    /// exactly once, from any thread. A result dropped without a reply
    /// fails the call with `reply_dropped`.
    public func asyncMethod(_ method: String, _ handler: @escaping (_ args: Any?, _ result: DriftResult) -> Void) {
        add(method, .async(handler))
    }

    /// Declares the channel's event stream: `onListen` runs when Go's event
    /// channel of this name gains its first subscriber, `onCancel` when it
    /// loses its last, each on the main thread, alternating, starting with
    /// `onListen`. Start the native work (location updates, a sensor) in
    /// `onListen` and send its events with `sendEvent`; stop it in
    /// `onCancel`.
    ///
    /// An error `onListen` throws reaches Go's subscribers as a stream error
    /// (as `sendEventError` sends), and `onCancel` then does not run: the
    /// stream did not start. A stream Go subscribed to before the app
    /// started starts once the channel is registered. `sendEventDone` ends
    /// the stream without `onCancel`; Go's next subscriber starts it again.
    public func stream(onListen: @escaping () throws -> Void, onCancel: @escaping () -> Void) {
        if sealed {
            fatalError("drift: \(name) stream declared after registerChannel returned; declare it inside its closure")
        }
        if declaredStream != nil {
            fatalError("drift: \(name) stream declared twice")
        }
        declaredStream = Stream(onListen: onListen, onCancel: onCancel)
    }

    private func add(_ method: String, _ m: Method) {
        if sealed {
            fatalError("drift: \(name).\(method) declared after registerChannel returned; declare methods inside its closure")
        }
        if methods.updateValue(m, forKey: method) != nil {
            fatalError("drift: \(name).\(method) declared twice")
        }
    }

    @_spi(DriftHost) public func seal() {
        sealed = true
    }

    @_spi(DriftHost) public func lookup(_ method: String) -> Method? {
        methods[method]
    }
}

/// The reply to one asynchronous method call. Exactly one of success(_:) or
/// error(_:) must be called, once, from any thread; a second reply is a
/// programming error and traps. Until the reply arrives the Go caller
/// waits; a result released without a reply fails the call.
public final class DriftResult {
    private let lock = NSLock()
    private var replied = false
    private let call: String
    private let deliver: (Result<Any?, Error>) -> Void

    @_spi(DriftHost) public init(call: String, deliver: @escaping (Result<Any?, Error>) -> Void) {
        self.call = call
        self.deliver = deliver
    }

    deinit {
        if !replied {
            deliver(.failure(DriftCallError("reply_dropped", "drift: \(call) released its DriftResult without replying")))
        }
    }

    /// Replies with a JSON-like value (or nil). Any other value fails the
    /// call with `not_encodable`.
    public func success(_ value: Any?) {
        submit(.success(value))
    }

    /// Replies with an error; Go receives it as the call's error.
    public func error(_ error: Error) {
        submit(.failure(error))
    }

    private func submit(_ outcome: Result<Any?, Error>) {
        lock.lock()
        let first = !replied
        replied = true
        lock.unlock()
        guard first else {
            fatalError("drift: \(call) replied twice")
        }
        deliver(outcome)
    }
}

/// Token returned from `DriftPluginHost.observeEvent`. Call `cancel()` to
/// stop receiving callbacks. Idempotent; subsequent calls are no-ops.
///
/// Hold the token in the plugin's state if the subscription needs to outlive
/// the call site; drop it if "subscribe until process death" is acceptable.
public final class DriftSubscription {
    private let cancelClosure: () -> Void
    private let lock = NSLock()
    private var canceled = false

    @_spi(DriftHost) public init(cancel: @escaping () -> Void) {
        self.cancelClosure = cancel
    }

    public func cancel() {
        lock.lock()
        defer { lock.unlock() }
        if canceled { return }
        canceled = true
        cancelClosure()
    }
}

public protocol DriftPluginHost: AnyObject {
    /// The plugin's config, as its build half recorded it with
    /// `ctx.NativeConfig`: JSON-like values (numbers are NSNumbers), empty
    /// when it recorded none. Fixed for the life of the process.
    var config: [String: Any] { get }

    /// Registers the channel `name` and its methods, declared in `declare`:
    ///
    ///     host.registerChannel("acme/camera") { channel in
    ///         channel.method("isAvailable") { _ in hasCamera() }
    ///         channel.asyncMethod("takePicture") { args, result in capture(args, result) }
    ///     }
    ///
    /// See `DriftChannel` for how each kind of method replies. Declare every
    /// method inside the closure; the channel is fixed once it returns.
    /// Name it `<vendor>/<feature>`; `drift/` is reserved for Drift and its
    /// first-party plugins, and registering a name there, or a name twice,
    /// traps.
    func registerChannel(_ name: String, _ declare: (DriftChannel) -> Void)
    /// Sends `data` (JSON-like, or nil) to Go's listeners on `channel`,
    /// whether or not Go is listening. A value that is not JSON-like is
    /// logged and reaches Go as a `not_encodable` stream error instead.
    func sendEvent(_ channel: String, data: Any?)
    /// Fails Go's stream on `channel` with `error`: its subscribers'
    /// `OnError` receive it, a `DriftCallError` with its code, message and
    /// details, any other error as `native_error`, as for a failed call.
    /// The stream continues; send later events as usual.
    func sendEventError(_ channel: String, _ error: Error)
    /// Ends Go's stream on `channel`: its subscriptions end.
    func sendEventDone(_ channel: String)

    /// Subscribes to events posted on `channel` via the host's `sendEvent`
    /// path. Native producers (e.g. the engine emitting `first_frame` on
    /// `drift/rendering/frame_events`) fan out to all observers, which run
    /// asynchronously on the main thread, in the order events were sent.
    ///
    /// Fan-out covers every `sendEvent` invocation regardless of caller:
    /// events originating in native modules are delivered to native
    /// observers, AND events whose ultimate consumer is Go-side
    /// `EventChannel.Listen` are also fanned out here in-process. A native
    /// observer never has to know which side produced the event.
    ///
    /// The returned token's `cancel()` unsubscribes; no callback runs after
    /// it returns on the main thread. Plugins that observe for the life of
    /// the process can discard the token.
    func observeEvent(_ channel: String, handler: @escaping (Any?) -> Void) -> DriftSubscription

    /// Registers the permission `kind` (Go: `platform.NewPermission(kind)`).
    /// Register each kind once, from `register(host:)`, together with the
    /// Info.plist usage string it needs; registering a kind twice traps.
    func registerPermission(_ kind: String, _ permission: DriftPermission)

    /// Registers the platform view type `type`, created by `factory`. Go
    /// creates views of it through a `platform.PlatformViewFactory` with
    /// the same type. Register each type once, from `register(host:)`;
    /// registering a type twice traps.
    func registerPlatformView(_ type: String, _ factory: @escaping DriftPlatformViewFactory)
}
