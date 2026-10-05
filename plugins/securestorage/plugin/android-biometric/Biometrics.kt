/**
 * Biometrics.kt (biometrics: configured)
 *
 * Strong biometric authentication through androidx.biometric's
 * BiometricPrompt, shown over the app's FragmentActivity.
 */
package com.drift.plugin.securestorage

import android.content.Context
import android.content.pm.PackageManager
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricPrompt
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity
import com.drift.runner.DriftCallError

internal object Biometrics {
    private const val STRONG = BiometricManager.Authenticators.BIOMETRIC_STRONG

    /** The plugin's config enables biometrics. */
    fun requireConfigured() {}

    /** "fingerprint", "face", or "none" when unavailable or not enrolled. */
    fun available(context: Context): String {
        if (BiometricManager.from(context).canAuthenticate(STRONG) != BiometricManager.BIOMETRIC_SUCCESS) {
            return "none"
        }
        return if (context.packageManager.hasSystemFeature(PackageManager.FEATURE_FACE)) "face" else "fingerprint"
    }

    /**
     * Prompts on the main thread, then calls done once: with null on
     * success, otherwise with the error to fail the call with.
     */
    fun authenticate(activity: FragmentActivity, message: String, done: (DriftCallError?) -> Unit) {
        val callback = object : BiometricPrompt.AuthenticationCallback() {
            override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) {
                done(null)
            }

            override fun onAuthenticationError(errorCode: Int, errString: CharSequence) {
                val code = when (errorCode) {
                    BiometricPrompt.ERROR_USER_CANCELED,
                    BiometricPrompt.ERROR_NEGATIVE_BUTTON,
                    BiometricPrompt.ERROR_CANCELED -> "auth_canceled"
                    BiometricPrompt.ERROR_NO_BIOMETRICS,
                    BiometricPrompt.ERROR_HW_NOT_PRESENT,
                    BiometricPrompt.ERROR_HW_UNAVAILABLE -> "biometrics_unavailable"
                    else -> "auth_failed"
                }
                done(DriftCallError(code, errString.toString()))
            }
            // onAuthenticationFailed: one rejected attempt; the prompt stays
            // up and ends in success or an error.
        }
        val info = BiometricPrompt.PromptInfo.Builder()
            .setTitle(message)
            .setNegativeButtonText(activity.getString(android.R.string.cancel))
            .setAllowedAuthenticators(STRONG)
            .build()
        BiometricPrompt(activity, ContextCompat.getMainExecutor(activity), callback).authenticate(info)
    }
}
