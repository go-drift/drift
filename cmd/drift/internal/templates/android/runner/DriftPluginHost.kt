/**
 * DriftPluginHost.kt
 *
 * Stable host API consumed by Drift plugin Kotlin sources. The drift runtime's
 * PlatformChannelManager adopts this interface; plugin Kotlin sources
 * reference only com.drift.runner.* types, never the user's app package,
 * so the same source compiles in every project.
 *
 * IMPORTANT: This file is shipped verbatim by Drift's scaffold and must NOT
 * depend on the user's app package. The package declaration is fixed.
 */
package com.drift.runner

import android.content.Context

interface DriftPluginHost {
    val context: Context
    fun registerChannel(name: String, handler: MethodHandler)
    fun sendEvent(channel: String, data: Any?)
    fun sendEventError(channel: String, code: String, message: String)
    fun sendEventDone(channel: String)
}
