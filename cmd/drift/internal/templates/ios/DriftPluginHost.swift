/// DriftPluginHost.swift
///
/// Stable host API consumed by Drift plugin Swift sources. Plugin sources
/// reference this protocol; the templated PlatformChannelManager adopts it.
/// MethodHandler is declared here so both the protocol and concrete plugins
/// can refer to it without depending on PlatformChannel.swift internals.

import Foundation

typealias MethodHandler = (String, Any?) -> (Any?, Error?)

/// Token returned from `DriftPluginHost.observeEvent`. Call `cancel()` to
/// stop receiving callbacks. Idempotent; subsequent calls are no-ops.
///
/// Hold the token in the plugin's state if the subscription needs to outlive
/// the call site; drop it if "subscribe until process death" is acceptable.
final class DriftSubscription {
    private let cancelClosure: () -> Void
    private let lock = NSLock()
    private var canceled = false

    init(cancel: @escaping () -> Void) {
        self.cancelClosure = cancel
    }

    func cancel() {
        lock.lock()
        defer { lock.unlock() }
        if canceled { return }
        canceled = true
        cancelClosure()
    }
}

protocol DriftPluginHost: AnyObject {
    func registerChannel(_ name: String, handler: @escaping MethodHandler)
    func sendEvent(_ channel: String, data: Any?)
    func sendEventError(_ channel: String, code: String, message: String)
    func sendEventDone(_ channel: String)

    /// Subscribes to events posted on `channel` via the host's `sendEvent`
    /// path. Native producers (e.g. the engine emitting `first_frame` on
    /// `drift/rendering/frame_events`) fan out to all observers
    /// synchronously on the producer's thread. Handlers that need
    /// main-thread access must dispatch themselves.
    ///
    /// Fan-out covers every `sendEvent` invocation regardless of caller:
    /// events originating in native modules are delivered to native
    /// observers, AND events whose ultimate consumer is Go-side
    /// `EventChannel.Listen` are also fanned out here in-process. A native
    /// observer never has to know which side produced the event.
    ///
    /// The returned token's `cancel()` unsubscribes. Plugins that observe
    /// for the life of the process can discard the token.
    func observeEvent(_ channel: String, handler: @escaping (Any?) -> Void) -> DriftSubscription
}
