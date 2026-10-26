/**
 * SplashConfig.kt
 *
 * The splash config the build half records (codegen.go nativeConfig), read
 * from host.config. Android uses the durations; the colours and icon reach
 * the platform splash screen through the build's theme.
 */
package com.drift.plugin.splash

internal class SplashConfig(config: Map<String, Any?>) {
    val fadeDurationMs = config.millis("fadeDurationMs")
    val maxDurationMs = config.millis("maxDurationMs")
    val ceilingMs = config.millis("ceilingMs")

    private companion object {
        /** A duration the build half wrote; a missing one is a bug. */
        fun Map<String, Any?>.millis(key: String): Long =
            (this[key] as? Number)?.toLong() ?: error("drift splash: config has no $key: $this")
    }
}
