/**
 * DriftSplashPlugin.kt
 *
 * Entry point invoked from the generated DriftPluginRegistrant.registerAll.
 * Responsibilities (mirrors the iOS implementation):
 *   1. Install the overlay View on `host.driftRootView()` (the activity's
 *      decorView) synchronously so the visual hand-off from the legacy
 *      LaunchTheme drawable is seamless.
 *   2. Register the `drift/splash` channel handler. The Go runtime calls
 *      `preserve` / `remove`; both forward to `DriftSplashState.apply(±1)`.
 *   3. Subscribe natively to `drift/rendering/frame_events`. On `first_frame`,
 *      mark state and dismiss if there are no outstanding preserves.
 */
package com.drift.plugin.splash

import android.os.Handler
import android.os.Looper
import android.util.Log
import android.view.ViewGroup
import com.drift.runner.DriftOverlayHost
import com.drift.runner.DriftPluginHost
import com.drift.runner.DriftSubscription

object DriftSplashPlugin {

    private const val TAG = "DriftSplash"

    private var overlay: DriftSplashOverlayView? = null
    private var frameSubscription: DriftSubscription? = null
    private val mainHandler = Handler(Looper.getMainLooper())

    @JvmStatic
    fun register(host: DriftPluginHost) {
        installOverlay(host)
        // Channel handler: named methods matching the Go API.
        // DriftSplashState.apply(±1) is the single saturation site;
        // a Remove without a matching Preserve clamps to zero.
        host.registerChannel("drift/splash") { method, _ ->
            when (method) {
                "preserve" -> {
                    DriftSplashState.apply(1)
                    maybeDismiss()
                    Pair(null, null)
                }
                "remove" -> {
                    DriftSplashState.apply(-1)
                    maybeDismiss()
                    Pair(null, null)
                }
                else -> Pair(null, IllegalArgumentException("unknown splash method $method"))
            }
        }
        // SkiaHostView fires first_frame after the next frame commit; the
        // host's event router delivers it here. The subscription token is
        // retained so v2 cleanup (Activity-recreate teardown) has a handle.
        frameSubscription = host.observeEvent("drift/rendering/frame_events") { data ->
            val payload = data as? Map<*, *> ?: return@observeEvent
            if (payload["type"] != "first_frame") return@observeEvent
            DriftSplashState.markFirstFrame()
            maybeDismiss()
        }
    }

    private fun installOverlay(host: DriftPluginHost) {
        val overlayHost = host as? DriftOverlayHost
        if (overlayHost == null) {
            Log.w(TAG, "host does not implement DriftOverlayHost; runtime overlay disabled " +
                "(likely framework/plugin version mismatch)")
            return
        }
        val rootView = overlayHost.driftRootView() as? ViewGroup
        if (rootView == null) {
            Log.w(TAG, "DriftOverlayHost.driftRootView() returned null or non-ViewGroup; " +
                "runtime overlay disabled (no active activity?)")
            return
        }
        val view = DriftSplashOverlayView(rootView.context)
        rootView.addView(view)
        overlay = view
    }

    private fun maybeDismiss() {
        if (!DriftSplashState.canDismiss()) return
        mainHandler.post {
            val current = overlay ?: return@post
            current.fadeOut(DriftSplashConfig.FADE_DURATION_MS) { overlay = null }
        }
    }
}
