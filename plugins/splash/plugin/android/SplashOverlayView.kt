/**
 * SplashOverlayView.kt
 *
 * Full-screen view drawing the launch theme's own `launch_background`
 * drawable, so it takes over from the launch window pixel for pixel
 * (resource qualifiers such as night mode resolve the same way). Swallows
 * touches while the splash is up. Main thread only.
 */
package com.drift.plugin.splash

import android.animation.Animator
import android.animation.AnimatorListenerAdapter
import android.content.Context
import android.view.View
import android.view.ViewGroup

class DriftSplashOverlayView(context: Context) : View(context) {

    init {
        layoutParams = ViewGroup.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT,
            ViewGroup.LayoutParams.MATCH_PARENT,
        )
        val id = resources.getIdentifier("launch_background", "drawable", context.packageName)
        check(id != 0) { "drift splash: launch_background drawable missing" }
        background = context.getDrawable(id)
        isClickable = true
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
}
