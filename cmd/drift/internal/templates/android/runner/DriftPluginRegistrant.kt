/**
 * DriftPluginRegistrant.kt
 *
 * Initial placeholder. The Drift plugin pipeline overwrites this file at each
 * build with one host.registerChannel(...) (or plugin.register(host)) call
 * per configured plugin, and one preActivityCreate call per plugin that
 * registered a pre-activity hook. With zero plugins both bodies stay empty
 * so the runtime call sites in PlatformChannelManager and MainActivity
 * always resolve.
 */
package com.drift.runner

import android.app.Activity

object DriftPluginRegistrant {
    fun registerAll(host: DriftPluginHost) {
    }

    /**
     * Runs once per Activity creation, before `super.onCreate(...)`. Plugins
     * that need access to the Activity instance pre-super (e.g. to call
     * `androidx.core.splashscreen.installSplashScreen()`) register a symbol
     * via `ctx.Android.PreActivityRegistrant(...)` at build time and the
     * Drift CLI inserts the call here.
     *
     * Stays empty when no plugin contributes a pre-activity hook.
     */
    fun preActivityCreate(activity: Activity) {
    }
}
