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

/// Handles one call on a plugin channel, on the main thread. Reply through
/// `result` exactly once, either before returning or later from any thread
/// (e.g. an SDK completion handler).
public typealias DriftMethodHandler = (_ method: String, _ args: Any?, _ result: DriftResult) -> Void

/// The reply to one method call. Exactly one of success(_:) or error(_:)
/// must be called, once; a second reply is a programming error and traps.
/// Until the reply arrives the Go caller waits, so a handler must not drop
/// its result.
public final class DriftResult {
    private let lock = NSLock()
    private var replied = false
    private let call: String
    private let deliver: (Result<Any?, Error>) -> Void

    @_spi(DriftHost) public init(call: String, deliver: @escaping (Result<Any?, Error>) -> Void) {
        self.call = call
        self.deliver = deliver
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
    func registerChannel(_ name: String, handler: @escaping DriftMethodHandler)
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
