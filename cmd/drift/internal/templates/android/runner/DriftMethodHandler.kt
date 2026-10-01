/**
 * DriftMethodHandler.kt
 *
 * The plugin-side method channel API: a handler and the result it replies
 * through.
 *
 * IMPORTANT: shipped verbatim by Drift's scaffold; must not depend on the
 * user's app package.
 */
package com.drift.runner

import java.util.concurrent.atomic.AtomicBoolean

/**
 * Handles one call on a plugin channel, on the main thread. Reply through
 * [result] exactly once, either before returning or later from any thread
 * (e.g. an SDK callback). An exception thrown before replying becomes the
 * call's error.
 */
fun interface DriftMethodHandler {
    fun onMethodCall(method: String, args: Any?, result: DriftResult)
}

/**
 * The reply to one method call. Exactly one of [success] or [error] must be
 * called, once; a second reply throws. Until the reply arrives the Go caller
 * waits, so a handler must not drop its result.
 */
class DriftResult internal constructor(
    private val call: String,
    private val deliver: (value: Any?, error: Exception?) -> Unit,
) {
    private val replied = AtomicBoolean(false)

    /** Replies with a JSON-encodable value (or null). */
    fun success(value: Any?) {
        check(replied.compareAndSet(false, true)) { "drift: $call replied twice" }
        deliver(value, null)
    }

    /** Replies with an error; Go receives it as the call's error. */
    fun error(error: Exception) {
        check(replied.compareAndSet(false, true)) { "drift: $call replied twice" }
        deliver(null, error)
    }

    /** Replies with error unless a reply was already submitted. */
    internal fun errorIfPending(error: Exception) {
        if (replied.compareAndSet(false, true)) deliver(null, error)
    }
}
