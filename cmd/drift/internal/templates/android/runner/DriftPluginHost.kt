/**
 * DriftPluginHost.kt
 *
 * Stable host API consumed by Drift plugin Kotlin sources. The drift runtime's
 * PlatformChannelManager adopts this interface; plugin Kotlin sources
 * reference only com.drift.runner.* types, never the user's app package,
 * so the same source compiles in every project.
 *
 * Threading: every plugin callback (method handlers, event observers and
 * the DriftPlugin lifecycle) runs on the main thread.
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

    /**
     * The plugin's config, as its build half recorded it with
     * ctx.NativeConfig: JSON-like values (whole numbers are Int or Long,
     * others Double), empty when it recorded none. Fixed for the life of
     * the process.
     */
    val config: Map<String, Any?>

    /**
     * Registers the channel [name] and its methods, declared in [declare]:
     *
     * ```
     * host.registerChannel("acme/camera") {
     *     method("isAvailable") { _ -> hasCamera() }
     *     asyncMethod("takePicture") { args, result -> capture(args, result) }
     * }
     * ```
     *
     * See [DriftChannel] for how each kind of method replies. Declare every
     * method inside the block; the channel is fixed once it returns. Name
     * it `<vendor>/<feature>`; `drift/` is reserved for Drift and its
     * first-party plugins, and registering a name there, or a name twice,
     * crashes.
     */
    fun registerChannel(name: String, declare: DriftChannel.() -> Unit)

    /**
     * Sends [data] (JSON-like, or null) to Go's listeners on [channel],
     * whether or not Go is listening. A value that is not JSON-like is
     * logged and reaches Go as a `not_encodable` stream error instead.
     */
    fun sendEvent(channel: String, data: Any?)
    /**
     * Fails Go's stream on [channel] with [error]: its subscribers' OnError
     * receive it, a [DriftCallError] with its code, message and details, any
     * other exception as for a failed call. The stream continues; send later
     * events as usual.
     */
    fun sendEventError(channel: String, error: Exception)

    /** Ends Go's stream on [channel]: its subscriptions end. */
    fun sendEventDone(channel: String)

    /**
     * Subscribes to events posted on [channel] via the host's `sendEvent`
     * path. Native producers (e.g. the engine emitting `first_frame` on
     * `drift/rendering/frame_events`) fan out to all observers, which run
     * asynchronously on the main thread, in the order events were sent.
     *
     * Fan-out covers every `sendEvent` invocation regardless of caller:
     * events originating in native modules are delivered to native
     * observers, AND events whose ultimate consumer is Go-side
     * `EventChannel.Listen` are also fanned out here in-process. A native
     * observer never has to know which side produced the event.
     *
     * The returned token's [DriftSubscription.cancel] unsubscribes; no
     * callback runs after it returns on the main thread. Plugins that
     * observe for the life of the process can discard the token.
     */
    fun observeEvent(channel: String, handler: (Any?) -> Unit): DriftSubscription

    /**
     * Registers the permission [kind] (Go: `platform.NewPermission(kind)`).
     * Register each kind once, from onRegister, and declare its manifest
     * permissions in the plugin's Build; registering a kind twice crashes.
     */
    fun registerPermission(kind: String, permission: DriftPermission)

    /**
     * Registers the platform view type [type], created by [factory]. Go
     * creates views of it through a platform.PlatformViewFactory with the
     * same type. Register each type once, from onRegister; registering a
     * type twice crashes.
     */
    fun registerPlatformView(type: String, factory: DriftPlatformViewFactory)
}
