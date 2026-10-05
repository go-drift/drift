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

/** Creates views of one type. */
fun interface DriftPlatformViewFactory {
    /**
     * Creates view [viewId] with the parameters Go created it with, and its
     * event sender. [context] is the Activity's.
     */
    fun create(context: Context, viewId: Int, params: Map<String, Any?>, events: DriftPlatformViewEvents): DriftPlatformView
}
