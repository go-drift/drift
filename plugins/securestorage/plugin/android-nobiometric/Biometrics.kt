/**
 * Biometrics.kt (biometrics: not configured)
 *
 * The app ships no biometric library; values cannot require biometrics.
 */
package com.drift.plugin.securestorage

import android.content.Context
import androidx.fragment.app.FragmentActivity
import com.drift.runner.DriftCallError

internal object Biometrics {
    private fun notConfigured() =
        DriftCallError("biometrics_not_configured", "add biometrics: to the securestorage plugin config")

    fun requireConfigured() {
        throw notConfigured()
    }

    fun available(context: Context): String = "none"

    fun authenticate(activity: FragmentActivity, message: String, done: (DriftCallError?) -> Unit) {
        done(notConfigured())
    }
}
