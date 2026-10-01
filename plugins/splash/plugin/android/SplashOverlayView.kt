/**
 * SplashOverlayView.kt
 *
 * Full-screen FrameLayout that mirrors the LaunchTheme's launch_background
 * drawable so the runtime overlay can attach with no visual seam after the
 * system launch theme transitions to AppTheme. MATCH_PARENT layout params
 * keep the overlay glued to the activity's content area across rotation.
 *
 * `fadeOut(durationMs, onEnd)` runs on the main thread; callers must post
 * to a Handler before invoking.
 */
package com.drift.plugin.splash

import android.animation.Animator
import android.animation.AnimatorListenerAdapter
import android.content.Context
import android.graphics.Color
import android.view.Gravity
import android.view.ViewGroup
import android.widget.FrameLayout
import android.widget.ImageView

class DriftSplashOverlayView(context: Context) : FrameLayout(context) {

    private val imageView = ImageView(context).apply {
        scaleType = ImageView.ScaleType.FIT_CENTER
        val params = LayoutParams(
            ViewGroup.LayoutParams.WRAP_CONTENT,
            ViewGroup.LayoutParams.WRAP_CONTENT,
        ).apply { gravity = Gravity.CENTER }
        layoutParams = params
    }

    init {
        layoutParams = ViewGroup.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT,
            ViewGroup.LayoutParams.MATCH_PARENT,
        )
        setBackgroundColor(parseHex(DriftSplashConfig.BACKGROUND_COLOR))
        imageView.setImageResource(resources.getIdentifier(
            "drift_splash", "drawable", context.packageName,
        ))
        addView(imageView)
    }

    fun fadeOut(durationMs: Int, onEnd: () -> Unit) {
        animate()
            .alpha(0f)
            .setDuration(durationMs.toLong())
            .setListener(object : AnimatorListenerAdapter() {
                override fun onAnimationEnd(animation: Animator) {
                    (parent as? ViewGroup)?.removeView(this@DriftSplashOverlayView)
                    onEnd()
                }
            })
            .start()
    }

    private fun parseHex(s: String): Int {
        // Accepts #RRGGBB and #RRGGBBAA. Validated at build time.
        return try {
            Color.parseColor(s)
        } catch (_: IllegalArgumentException) {
            Color.WHITE
        }
    }
}
