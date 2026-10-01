/**
 * Android12SplashController.kt
 *
 * Invoked from `DriftPluginRegistrant.preActivityCreate(activity)` before
 * MainActivity calls `super.onCreate(...)`. Only meaningful on API 31+; on
 * older devices the legacy LaunchTheme drawable handles the splash.
 *
 * installSplashScreen() must run before super.onCreate. Once installed, the
 * SplashScreen API:
 *   - Reads the values-v31/styles.xml LaunchTheme variant to size and
 *     colour the system splash.
 *   - Calls our setKeepOnScreenCondition callback every vsync, so the
 *     system splash stays up until both `firstFrameRendered=true` and
 *     `preserveCount=0`.
 *   - Calls our setOnExitAnimationListener once the keep-on-screen returns
 *     false, where we run the cross-fade to the Drift content.
 */
package com.drift.plugin.splash

import android.app.Activity
import android.os.Build
import android.util.Log
import androidx.appcompat.app.AppCompatActivity
import androidx.core.splashscreen.SplashScreen.Companion.installSplashScreen
import androidx.core.splashscreen.SplashScreenViewProvider

object Android12SplashController {

    private const val TAG = "DriftSplash"

    @JvmStatic
    fun install(activity: Activity) {
        // SplashScreen API requires AppCompatActivity (or any ComponentActivity)
        // to satisfy its lifecycle expectations. MainActivity is AppCompatActivity
        // in the Drift template; the warn log covers the case where a user
        // customised MainActivity to extend a different base, in which case the
        // Android 12+ system splash silently falls back to the legacy path.
        val app = activity as? AppCompatActivity
        if (app == null) {
            Log.w(TAG, "activity is not AppCompatActivity; SplashScreen API skipped " +
                "(custom MainActivity base class?)")
            return
        }
        val splashScreen = app.installSplashScreen()
        splashScreen.setKeepOnScreenCondition { DriftSplashState.keepOnScreen() }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            splashScreen.setOnExitAnimationListener { provider -> fadeAndExit(provider) }
        }
    }

    private fun fadeAndExit(provider: SplashScreenViewProvider) {
        provider.view.animate()
            .alpha(0f)
            .setDuration(DriftSplashConfig.FADE_DURATION_MS.toLong())
            .withEndAction { provider.remove() }
            .start()
    }
}
