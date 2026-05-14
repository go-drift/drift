/**
 * MethodHandler.kt
 *
 * Functional interface for platform channel method handlers. Plugins and the
 * core runtime both implement this, either via a lambda (SAM conversion) or
 * an explicit object. operator fun invoke keeps the call-site sugar
 * (`handler(method, args)`) intact.
 */
package com.drift.runner

fun interface MethodHandler {
    operator fun invoke(method: String, args: Any?): Pair<Any?, Exception?>
}
