/**
 * DriftOverlayHost.kt
 *
 * Capability interface adopted by `PlatformChannelManager` so plugins can
 * install full-screen overlay views (splash screens, debug HUDs, etc.)
 * without depending on host internals. Plugins down-cast the host they
 * receive from `DriftPluginRegistrant.registerAll(host)`:
 *
 *     val overlayHost = host as? DriftOverlayHost ?: return
 *     val rootView = overlayHost.driftRootView() ?: return
 *     (rootView as ViewGroup).addView(myOverlayView)
 *
 * Kept in a separate file from `DriftPluginHost` so the core channel
 * interface stays narrow. Plugins that don't need overlay access don't
 * see this surface.
 */
package com.drift.runner

import android.view.View

interface DriftOverlayHost {
    /**
     * Returns the active root view onto which plugins may install overlay
     * subviews (typically the activity's `window.decorView`). Returns null
     * if no activity is currently attached.
     *
     * Overlays installed via this hook should use `MATCH_PARENT` layout
     * params so they follow bounds across rotation and configuration
     * changes.
     */
    fun driftRootView(): View?
}
