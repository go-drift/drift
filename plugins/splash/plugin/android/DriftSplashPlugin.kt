/**
 * DriftSplashPlugin.kt
 *
 * The splash plugin's native half, driven by com.drift.runner.DriftPlugins
 * on the main thread. The splash is the platform's splash screen (Android
 * 12+, Drift's minimum), styled by the launch theme the build half writes.
 * This class holds it on screen until the app is ready:
 *   1. onRegister: the `drift/splash` channel (the Go runtime's Preserve and
 *      Remove), the `drift/rendering/frame_events` observer (`first_frame`:
 *      the app has drawn content) and the max_duration_ms timer, after which
 *      Preserve no longer holds the splash. Once per process.
 *   2. onAttach: on the process's first Activity, hold its draws with a
 *      pre-draw listener (the platform keeps its splash until the Activity
 *      draws) and fade the splash out when it exits.
 *   3. onDetach: release the hold with the Activity.
 *
 * Dismissal is state, not an event: every input calls reconcile(), so an
 * input arriving while detached is never lost. Mirrors the iOS plugin.
 */
package com.drift.plugin.splash

import android.os.Handler
import android.os.Looper
import android.util.Log
import android.view.View
import android.view.ViewTreeObserver
import android.window.SplashScreenView
import com.drift.runner.DriftActivityBinding
import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost

class DriftSplashPlugin : DriftPlugin {
    /** Outstanding Preserve calls. */
    private var preserveCount = 0
    /** The app has drawn its first frame with content. */
    private var contentShown = false
    /** max_duration_ms has passed: Preserve no longer holds the splash. */
    private var timedOut = false
    /** The splash has gone for this process and never comes back. */
    private var dismissed = false
    /** Whether an Activity has attached in this process. */
    private var attachedBefore = false
    /** The attached Activity's content view and the listener holding its draws. */
    private var hold: Pair<View, ViewTreeObserver.OnPreDrawListener>? = null

    /**
     * The splash always waits for content (a slow App.OnInit keeps it up,
     * as Drift promises); the timeout only overrides a missed Remove.
     */
    private val dismissible get() = contentShown && (preserveCount == 0 || timedOut)

    override fun onRegister(host: DriftPluginHost) {
        host.registerChannel("drift/splash") { method, _, result ->
            when (method) {
                "preserve" -> if (dismissed) {
                    result.error(IllegalStateException(
                        "splash already dismissed; call Preserve before the first frame " +
                            "(App.OnInit or the root's InitState)"
                    ))
                } else {
                    preserveCount++
                    result.success(null)
                }
                "remove" -> {
                    preserveCount = maxOf(0, preserveCount - 1)
                    reconcile()
                    result.success(null)
                }
                else -> result.error(IllegalArgumentException("unknown splash method $method"))
            }
        }
        host.observeEvent("drift/rendering/frame_events") { data ->
            if ((data as? Map<*, *>)?.get("type") != "first_frame") return@observeEvent
            contentShown = true
            reconcile()
        }
        Handler(Looper.getMainLooper()).postDelayed({
            timedOut = true
            if (dismissed || preserveCount == 0) return@postDelayed
            Log.e(TAG, "splash held past max_duration_ms=${DriftSplashConfig.MAX_DURATION_MS} " +
                "by $preserveCount Preserve without Remove; ignoring them")
            reconcile()
        }, DriftSplashConfig.MAX_DURATION_MS.toLong())
    }

    override fun onAttach(binding: DriftActivityBinding) {
        val first = !attachedBefore
        attachedBefore = true
        if (dismissed) return
        if (!first) {
            // A recreated Activity (dark mode, locale) gets no platform
            // splash, so holding its draws would only show a blank window.
            dismissed = true
            return
        }
        val activity = binding.activity
        activity.splashScreen.setOnExitAnimationListener { fadeOut(it) }
        val content = activity.findViewById<View>(android.R.id.content)
        val listener = object : ViewTreeObserver.OnPreDrawListener {
            // Returning false cancels the draw and schedules another
            // traversal, so this runs every frame until the splash may go.
            override fun onPreDraw(): Boolean {
                if (!dismissed) return false
                content.viewTreeObserver.removeOnPreDrawListener(this)
                hold = null
                return true
            }
        }
        content.viewTreeObserver.addOnPreDrawListener(listener)
        hold = content to listener
    }

    override fun onDetach() {
        hold?.let { (content, listener) -> content.viewTreeObserver.removeOnPreDrawListener(listener) }
        hold = null
    }

    /**
     * Dismisses the splash once it may go: the held Activity draws on its
     * next frame and the platform runs the exit animation.
     */
    private fun reconcile() {
        if (dismissed || !dismissible) return
        dismissed = true
    }

    private fun fadeOut(view: SplashScreenView) {
        view.animate()
            .alpha(0f)
            .setDuration(DriftSplashConfig.FADE_DURATION_MS.toLong())
            .withEndAction { view.remove() }
            .start()
    }

    private companion object {
        const val TAG = "DriftSplash"
    }
}
