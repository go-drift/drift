/**
 * DriftActivityBinding.kt
 *
 * What a plugin gets when it attaches to MainActivity: the Activity, the
 * root view for overlays, and the intents Android delivers to the running
 * Activity. Valid from
 * DriftPlugin.onAttach until onDetach; listeners are dropped on detach, and
 * a plugin must not keep the binding past it.
 *
 * The Intent that launched the Activity is `activity.intent`; the listener
 * below covers later deliveries.
 *
 * IMPORTANT: shipped verbatim by Drift's scaffold; must not depend on the
 * user's app package.
 */
package com.drift.runner

import android.app.Activity
import android.content.Intent
import android.view.ViewGroup

class DriftActivityBinding(
    val activity: Activity,
    /**
     * Host for plugin overlays (a splash, say): the window's root view, so
     * overlays cover the whole window, above Drift's content and its
     * platform views.
     */
    val overlayView: ViewGroup,
) {
    private val newIntentListeners = mutableListOf<(Intent) -> Boolean>()

    /**
     * Receives intents delivered to the running Activity (onNewIntent).
     * Return true to claim the intent: later listeners and Drift's own
     * deep-link handling do not see it.
     */
    fun addOnNewIntentListener(listener: (intent: Intent) -> Boolean) {
        newIntentListeners += listener
    }

    internal fun dispatchNewIntent(intent: Intent): Boolean =
        newIntentListeners.any { it(intent) }

    internal fun clear() {
        newIntentListeners.clear()
    }
}
