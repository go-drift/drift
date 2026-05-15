/**
 * SplashState.kt
 *
 * Thread-safe shared state for the Drift splash plugin. The Go-side runtime
 * adjusts the preserve count via the `drift/splash` channel (single
 * `apply_delta` method); the first-frame event from
 * `drift/rendering/frame_events` flips the rendered flag. Both inputs feed
 * `canDismiss()` which the overlay (and the Android 12+ controller) polls
 * before tearing down.
 *
 * Object singleton — survives Activity recreation. On cold process start,
 * statics re-initialise so the count is fresh.
 */
package com.drift.plugin.splash

object DriftSplashState {
    private val lock = Any()
    private var preserveCount = 0
    private var firstFrameRenderedFlag = false

    /**
     * Applies a signed delta to the preserve count. Clamps at zero so a
     * `Remove()` before any `Preserve()` is end-to-end a no-op. This is the
     * single count-mutation entry point; the Go runtime always ships signed
     * deltas via `apply_delta`.
     */
    fun apply(delta: Int) {
        synchronized(lock) {
            preserveCount = maxOf(0, preserveCount + delta)
        }
    }

    fun markFirstFrame() {
        synchronized(lock) { firstFrameRenderedFlag = true }
    }

    fun canDismiss(): Boolean = synchronized(lock) {
        firstFrameRenderedFlag && preserveCount == 0
    }

    /** Snapshot accessor for the Android 12+ SplashScreen.setKeepOnScreenCondition. */
    fun keepOnScreen(): Boolean = synchronized(lock) {
        !firstFrameRenderedFlag || preserveCount > 0
    }
}
