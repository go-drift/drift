/**
 * DriftActivityBinding.kt
 *
 * What a plugin gets when it attaches to MainActivity: the Activity, the
 * root view for overlays, and the intents the Activity receives. Valid
 * from DriftPlugin.onAttach until onDetach; listeners are dropped on
 * detach, and a plugin must not keep the binding past it.
 *
 * IMPORTANT: shipped verbatim by Drift's scaffold; must not depend on the
 * user's app package.
 */
package com.drift.runner

import android.content.Intent
import android.view.ViewGroup
import androidx.fragment.app.FragmentActivity

class DriftActivityBinding(
    /**
     * MainActivity, a FragmentActivity: plugins can start activities for a
     * result through its activityResultRegistry and show BiometricPrompt.
     */
    val activity: FragmentActivity,
    /**
     * Host for plugin overlays (a splash, say): the window's root view, so
     * overlays cover the whole window, above Drift's content and its
     * platform views.
     */
    val overlayView: ViewGroup,
) {
    private val intentListeners = mutableListOf<(Intent) -> Boolean>()

    /**
     * Receives the intents the app is opened with (a notification tap, a
     * deep link), as iOS offers openURL: the intent that launched the
     * Activity, right after every plugin's onAttach, and each intent
     * delivered to the running Activity later (onNewIntent). Each arrives
     * once: a recreated Activity, or one relaunched from Recents, carries an
     * intent an earlier Activity already offered, and is not offered again.
     * Offered to plugins in drift.yaml order; return true to claim the
     * intent: later listeners and Drift's own deep-link handling do not see
     * it.
     */
    fun addIntentListener(listener: (intent: Intent) -> Boolean) {
        intentListeners += listener
    }

    internal fun dispatchIntent(intent: Intent): Boolean =
        intentListeners.any { it(intent) }

    internal fun clear() {
        intentListeners.clear()
    }
}
