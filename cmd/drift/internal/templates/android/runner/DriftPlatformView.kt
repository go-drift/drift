/**
 * DriftPlatformView.kt
 *
 * Native views embedded in Drift UI, provided by plugins (and by Drift for
 * its own text inputs, switches and web views). A plugin registers a view
 * type with DriftPluginHost.registerPlatformView; its Go runtime registers
 * a platform.PlatformViewFactory of the same type and embeds the view in a
 * widget. Drift positions, clips and shows the view each frame.
 *
 * Threading: Drift creates views and calls them on the main thread, in the
 * order Go made the calls.
 *
 * A view outlives the Activity that hosts it: when the Activity is
 * recreated (a configuration change) or replaced (the app reopened after
 * its Activity finished), Drift moves the view into the new one. Its
 * context follows: see [DriftPlatformViewFactory.create].
 *
 * IMPORTANT: shipped verbatim by Drift's scaffold; must not depend on the
 * user's app package.
 */
package com.drift.runner

import android.content.Context
import android.view.View

/** One native view instance. */
interface DriftPlatformView {
    /** The native view Drift positions over its content. */
    val view: View

    /**
     * Whether the view can be clipped by a path for partial occlusion:
     * true for TextureView-backed views, false for SurfaceView-backed ones.
     */
    val supportsRegionMask: Boolean get() = false

    /**
     * Handles a method Go called (PlatformViewRegistry.InvokeViewMethod);
     * [args] holds the call's arguments. Go does not wait for view methods:
     * a thrown exception (an unknown method, a bad argument) is logged.
     * Report results to Go as events.
     */
    fun handle(method: String, args: Map<String, Any?>)

    /** Releases the view's resources; Drift removes the view afterwards. */
    fun dispose()
}

/** Sends a view's events to its Go view (PlatformView.HandleEvent). */
class DriftPlatformViewEvents internal constructor(
    /** The view's ID, shared with Go. */
    val viewId: Int,
    private val deliver: (Map<String, Any?>) -> Unit,
) {
    /** Sends [method] with [data] to the Go view, from any thread. */
    fun send(method: String, data: Map<String, Any?> = emptyMap()) {
        deliver(data + mapOf("method" to method, "viewId" to viewId))
    }
}

/**
 * The observeEvent channel Drift posts on once at launch, on the main
 * thread, unless the app called engine.DisableViewWarmUp. A plugin whose
 * view loads a heavy library creates and drops a throwaway instance so the
 * first real view opens without a stall. Native only: Go never sees it.
 */
const val DRIFT_PLATFORM_VIEW_WARM_UP_EVENT = "drift/platform_views/warm_up"

/** Creates views of one type. */
fun interface DriftPlatformViewFactory {
    /**
     * Creates view [viewId] with the parameters Go created it with, and its
     * event sender. An exception it throws fails Go's
     * PlatformViewRegistry.Create (a [DriftCallError] with its code, an
     * IllegalArgumentException as `invalid_arguments`, anything else as
     * `native_error`), and no view exists.
     *
     * [context] is the view's own: it wraps the hosting Activity, and Drift
     * points it at the next Activity when it moves the view (and at the
     * Application in between), so the view never keeps a dead Activity.
     * Create the view with it; do not cast it to an Activity or keep the
     * Activity it wraps.
     */
    fun create(context: Context, viewId: Int, params: Map<String, Any?>, events: DriftPlatformViewEvents): DriftPlatformView
}
