/**
 * DriftPluginHost.kt
 *
 * Stable host API consumed by Drift plugin Kotlin sources. The drift runtime's
 * PlatformChannelManager adopts this interface; plugin Kotlin sources
 * reference only com.drift.runner.* types, never the user's app package,
 * so the same source compiles in every project.
 *
 * IMPORTANT: This file is shipped verbatim by Drift's scaffold and must NOT
 * depend on the user's app package. The package declaration is fixed.
 */
package com.drift.runner

import android.content.Context

/**
 * Token returned from `DriftPluginHost.observeEvent`. Call `cancel()` to
 * stop receiving callbacks. Idempotent; subsequent calls are no-ops.
 *
 * Hold the token in plugin state if the subscription needs to outlive the
 * call site; drop it if "subscribe until process death" is acceptable.
 */
class DriftSubscription internal constructor(private val cancelFn: () -> Unit) {
    private val lock = Any()
    private var canceled = false

    fun cancel() {
        synchronized(lock) {
            if (canceled) return
            canceled = true
            cancelFn()
        }
    }
}

interface DriftPluginHost {
    val context: Context
    fun registerChannel(name: String, handler: MethodHandler)
    fun sendEvent(channel: String, data: Any?)
    fun sendEventError(channel: String, code: String, message: String)
    fun sendEventDone(channel: String)

    /**
     * Subscribes to events posted on [channel] via the host's `sendEvent`
     * path. Native producers (e.g. the engine emitting `first_frame` on
     * `drift/rendering/frame_events`) fan out to all observers synchronously
     * on the producer's thread. Handlers that need main-thread access must
     * post to a Handler themselves.
     *
     * Fan-out covers every `sendEvent` invocation regardless of caller:
     * events originating in native modules are delivered to native
     * observers, AND events whose ultimate consumer is Go-side
     * `EventChannel.Listen` are also fanned out here in-process. A native
     * observer never has to know which side produced the event.
     *
     * The returned token's [DriftSubscription.cancel] unsubscribes. Plugins
     * that observe for the life of the process can discard the token.
     */
    fun observeEvent(channel: String, handler: (Any?) -> Unit): DriftSubscription
}
