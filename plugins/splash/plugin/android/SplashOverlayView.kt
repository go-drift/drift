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
import android.content.res.Configuration
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

    private var brandingView: ImageView? = null

    init {
        layoutParams = ViewGroup.LayoutParams(
            ViewGroup.LayoutParams.MATCH_PARENT,
            ViewGroup.LayoutParams.MATCH_PARENT,
        )
        applyAppearance()
        imageView.setImageResource(resources.getIdentifier(
            "drift_splash", "drawable", context.packageName,
        ))
        addView(imageView)
        installBrandingIfPresent()
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        applyAppearance()
    }

    private fun applyAppearance() {
        val dark = (resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK) ==
            Configuration.UI_MODE_NIGHT_YES
        val bg = if (dark && DriftSplashConfig.DARK_BACKGROUND_COLOR.isNotEmpty())
            DriftSplashConfig.DARK_BACKGROUND_COLOR
        else
            DriftSplashConfig.BACKGROUND_COLOR
        setBackgroundColor(parseHex(bg))
    }

    private fun installBrandingIfPresent() {
        val brandingId = resources.getIdentifier(
            "drift_splash_branding", "drawable", context.packageName,
        )
        if (brandingId == 0) return
        val view = ImageView(context).apply {
            scaleType = ImageView.ScaleType.FIT_CENTER
            setImageResource(brandingId)
        }
        val gravity = when (DriftSplashConfig.BRANDING_POSITION) {
            "bottom_left" -> Gravity.BOTTOM or Gravity.START
            "bottom_right" -> Gravity.BOTTOM or Gravity.END
            else -> Gravity.BOTTOM or Gravity.CENTER_HORIZONTAL
        }
        view.layoutParams = LayoutParams(
            ViewGroup.LayoutParams.WRAP_CONTENT,
            ViewGroup.LayoutParams.WRAP_CONTENT,
        ).apply {
            this.gravity = gravity
            bottomMargin = (resources.displayMetrics.density * 24).toInt()
            leftMargin = bottomMargin
            rightMargin = bottomMargin
        }
        addView(view)
        brandingView = view
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
