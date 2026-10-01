/**
 * DriftActivityBinding.kt
 *
 * What a plugin gets when it attaches to MainActivity: the Activity, the
 * root view for overlays, and registration for Activity callbacks that
 * Android only delivers to the Activity itself. Valid from
 * DriftPlugin.onAttach until onDetach; listeners are dropped on detach, and
 * a plugin must not keep the binding past it.
 *
 * The Intent that launched the Activity is `activity.intent`; the listeners
 * below cover later deliveries.
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
    private val activityResultListeners = mutableListOf<(Int, Int, Intent?) -> Boolean>()
    private val permissionsResultListeners = mutableListOf<(Int, Array<out String>, IntArray) -> Boolean>()

    /**
     * Receives intents delivered to the running Activity (onNewIntent).
     * Return true to claim the intent: later listeners and Drift's own
     * deep-link handling do not see it.
     */
    fun addOnNewIntentListener(listener: (intent: Intent) -> Boolean) {
        newIntentListeners += listener
    }

    /** Receives onActivityResult. Return true if the request code was yours. */
    fun addActivityResultListener(listener: (requestCode: Int, resultCode: Int, data: Intent?) -> Boolean) {
        activityResultListeners += listener
    }

    /** Receives onRequestPermissionsResult. Return true if the request code was yours. */
    fun addRequestPermissionsResultListener(
        listener: (requestCode: Int, permissions: Array<out String>, grantResults: IntArray) -> Boolean,
    ) {
        permissionsResultListeners += listener
    }

    internal fun dispatchNewIntent(intent: Intent): Boolean =
        newIntentListeners.any { it(intent) }

    internal fun dispatchActivityResult(requestCode: Int, resultCode: Int, data: Intent?): Boolean =
        activityResultListeners.any { it(requestCode, resultCode, data) }

    internal fun dispatchRequestPermissionsResult(
        requestCode: Int,
        permissions: Array<out String>,
        grantResults: IntArray,
    ): Boolean = permissionsResultListeners.any { it(requestCode, permissions, grantResults) }

    internal fun clear() {
        newIntentListeners.clear()
        activityResultListeners.clear()
        permissionsResultListeners.clear()
    }
}
