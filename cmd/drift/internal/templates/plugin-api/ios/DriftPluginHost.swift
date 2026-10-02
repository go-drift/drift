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
public final class DriftChannel {
    @_spi(DriftHost) public enum Method {
        case sync((_ args: Any?) throws -> Any?)
        case async((_ args: Any?, _ result: DriftResult) -> Void)
    }

    private let name: String
    private var methods: [String: Method] = [:]
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
            deliver(.failure(NSError(domain: "reply_dropped", code: 0, userInfo: [
                NSLocalizedDescriptionKey: "drift: \(call) released its DriftResult without replying",
            ])))
        }
    }

    /// Replies with a JSON-encodable value (or nil).
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
    /// Registers the channel `name` and its methods, declared in `declare`:
    ///
    ///     host.registerChannel("acme/camera") { channel in
    ///         channel.method("isAvailable") { _ in hasCamera() }
    ///         channel.asyncMethod("takePicture") { args, result in capture(args, result) }
    ///     }
    ///
    /// See `DriftChannel` for how each kind of method replies. Declare every
    /// method inside the closure; the channel is fixed once it returns.
    func registerChannel(_ name: String, _ declare: (DriftChannel) -> Void)
    func sendEvent(_ channel: String, data: Any?)
    func sendEventError(_ channel: String, code: String, message: String)
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
}
