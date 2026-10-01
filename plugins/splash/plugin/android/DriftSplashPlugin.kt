/**
 * DriftSplashPlugin.kt
 *
 * The splash plugin's native half, driven by com.drift.runner.DriftPlugins
 * (mirrors the iOS implementation):
 *   1. onRegister: the `drift/splash` channel (the Go runtime calls
 *      `preserve` / `remove`, both forwarding to DriftSplashState.apply(±1))
 *      and the `drift/rendering/frame_events` observer (on `first_frame`,
 *      mark state and dismiss if nothing preserves the splash). Once per
 *      process.
 *   2. onPreActivityCreate: the Android 12+ SplashScreen, when configured.
 *   3. onAttach: install the overlay on the window's root view, taking over
 *      from the launch theme drawable. onDetach drops it with the Activity.
 */
package com.drift.plugin.splash

import android.app.Activity
import android.view.ViewGroup
import com.drift.runner.DriftActivityBinding
import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost

class DriftSplashPlugin : DriftPlugin {
    private var overlay: DriftSplashOverlayView? = null
    // Set once the overlay has faded out; a recreated Activity does not
    // bring the splash back.
    private var dismissed = false

    override fun onRegister(host: DriftPluginHost) {
        host.registerChannel("drift/splash") { method, _, result ->
            when (method) {
                "preserve" -> {
                    DriftSplashState.apply(1)
                    maybeDismiss()
                    result.success(null)
                }
                "remove" -> {
                    DriftSplashState.apply(-1)
                    maybeDismiss()
                    result.success(null)
                }
                else -> result.error(IllegalArgumentException("unknown splash method $method"))
            }
        }
        host.observeEvent("drift/rendering/frame_events") { data ->
            val payload = data as? Map<*, *> ?: return@observeEvent
            if (payload["type"] != "first_frame") return@observeEvent
            DriftSplashState.markFirstFrame()
            maybeDismiss()
        }
    }

    override fun onPreActivityCreate(activity: Activity) {
        DriftSplashConfig.preActivityCreate(activity)
    }

    override fun onAttach(binding: DriftActivityBinding) {
        if (dismissed) return
        val view = DriftSplashOverlayView(binding.activity)
        binding.rootView.addView(view)
        overlay = view
    }

    override fun onDetach() {
        overlay?.let { (it.parent as? ViewGroup)?.removeView(it) }
        overlay = null
    }

    // Called on the main thread, where the host runs plugin handlers and
    // observers.
    private fun maybeDismiss() {
        if (dismissed || !DriftSplashState.canDismiss()) return
        val current = overlay ?: return
        dismissed = true
        current.fadeOut(DriftSplashConfig.FADE_DURATION_MS) {
            if (overlay === current) overlay = null
        }
    }
}
