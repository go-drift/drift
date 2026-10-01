/**
 * DriftPlugins.kt
 *
 * Owns the app's plugin instances and drives their lifecycle. MainActivity
 * calls in; the generated DriftPluginRegistrant only lists the plugins.
 * Drift's own handling of the same Activity events (deep links,
 * notifications, built-in permission and camera results) stays in
 * MainActivity.
 *
 * Main thread only.
 *
 * IMPORTANT: shipped verbatim by Drift's scaffold; must not depend on the
 * user's app package.
 */
package com.drift.runner

import android.app.Activity
import android.content.Intent
import android.os.Looper
import android.view.ViewGroup

object DriftPlugins {
    private var plugins: List<DriftPlugin>? = null
    private var binding: DriftActivityBinding? = null

    /**
     * Creates the plugins and registers each with host, once per process.
     * Later calls (every Activity creation) are no-ops, so channels and
     * observers are never registered twice.
     */
    fun register(host: DriftPluginHost) {
        checkMainThread()
        if (plugins != null) return
        val created = DriftPluginRegistrant.makePlugins()
        plugins = created
        created.forEach { it.onRegister(host) }
    }

    /** Runs each plugin's pre-super.onCreate hook for this Activity creation. */
    fun preActivityCreate(activity: Activity) {
        requirePlugins().forEach { it.onPreActivityCreate(activity) }
    }

    /**
     * Attaches every plugin to activity, detaching from a previous Activity
     * first. overlayView hosts plugin overlays (DriftActivityBinding).
     */
    fun attach(activity: Activity, overlayView: ViewGroup) {
        val all = requirePlugins()
        binding?.let { detachAll(it) }
        val b = DriftActivityBinding(activity, overlayView)
        binding = b
        all.forEach { it.onAttach(b) }
    }

    /** Detaches every plugin if they are attached to activity. */
    fun detach(activity: Activity) {
        checkMainThread()
        val b = binding ?: return
        if (b.activity !== activity) return
        detachAll(b)
    }

    /** Offers intent to attached plugins; true if one claimed it. */
    fun onNewIntent(intent: Intent): Boolean {
        checkMainThread()
        return binding?.dispatchNewIntent(intent) ?: false
    }

    /** Offers an activity result to attached plugins; true if one claimed it. */
    fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?): Boolean {
        checkMainThread()
        return binding?.dispatchActivityResult(requestCode, resultCode, data) ?: false
    }

    /** Offers a permissions result to attached plugins; true if one claimed it. */
    fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray): Boolean {
        checkMainThread()
        return binding?.dispatchRequestPermissionsResult(requestCode, permissions, grantResults) ?: false
    }

    private fun detachAll(b: DriftActivityBinding) {
        binding = null
        requirePlugins().asReversed().forEach { it.onDetach() }
        b.clear()
    }

    private fun requirePlugins(): List<DriftPlugin> {
        checkMainThread()
        return checkNotNull(plugins) {
            "drift: DriftPlugins.register must run first (MainActivity.onCreate)"
        }
    }

    private fun checkMainThread() {
        check(Looper.myLooper() == Looper.getMainLooper()) {
            "drift: DriftPlugins must be used on the main thread"
        }
    }
}
