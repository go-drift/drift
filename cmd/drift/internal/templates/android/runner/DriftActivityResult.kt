/**
 * DriftActivityResult.kt
 *
 * Starting an Activity for its result without overriding
 * MainActivity.onActivityResult, for Drift's own handlers and plugins.
 *
 * IMPORTANT: shipped verbatim by Drift's scaffold; must not depend on the
 * user's app package.
 */
package com.drift.runner

import androidx.activity.result.ActivityResultLauncher
import androidx.activity.result.contract.ActivityResultContract
import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import java.util.concurrent.atomic.AtomicLong

private val launches = AtomicLong()

/**
 * Starts [contract] with [input] through [activity]'s activityResultRegistry
 * and calls [onResult] once, on the main thread: with the output, or with
 * null if the Activity is destroyed first (recreated behind the launched
 * one), so a caller waiting on the result is never left waiting.
 *
 * Call on the main thread. If launching throws (no app handles the
 * intent), the exception propagates and [onResult] is never called.
 */
fun <I, O> launchForResult(
    activity: FragmentActivity,
    contract: ActivityResultContract<I, O>,
    input: I,
    onResult: (O?) -> Unit,
) {
    var done = false
    lateinit var launcher: ActivityResultLauncher<I>
    lateinit var observer: LifecycleEventObserver
    fun release() {
        done = true
        launcher.unregister()
        activity.lifecycle.removeObserver(observer)
    }
    fun finish(output: O?) {
        if (done) return
        release()
        onResult(output)
    }
    observer = LifecycleEventObserver { _, event ->
        if (event == Lifecycle.Event.ON_DESTROY) finish(null)
    }
    launcher = activity.activityResultRegistry.register(
        "drift.result.${launches.incrementAndGet()}",
        contract,
    ) { finish(it) }
    activity.lifecycle.addObserver(observer)
    try {
        launcher.launch(input)
    } catch (e: Exception) {
        release()
        throw e
    }
}
