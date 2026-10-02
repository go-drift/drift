/**
 * DriftMethodHandler.kt
 *
 * The plugin-side method channel API: a channel's declared methods and the
 * result an asynchronous method replies through.
 *
 * IMPORTANT: shipped verbatim by Drift's scaffold; must not depend on the
 * user's app package.
 */
package com.drift.runner

import java.lang.ref.PhantomReference
import java.lang.ref.ReferenceQueue
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.concurrent.thread

/**
 * A plugin channel's methods, declared inside
 * [DriftPluginHost.registerChannel]. Every method runs on the main thread.
 *
 * A method replies in one of two declared ways:
 * - [method] replies by returning. Go may call it from any goroutine,
 *   including the UI thread.
 * - [asyncMethod] replies later, through a [DriftResult], from any thread
 *   (an SDK callback, a dialog). Go must call it from a goroutine: a call
 *   from the UI thread fails with `blocks_ui_thread` every time, before the
 *   handler runs, since the UI thread cannot wait for a reply that needs it.
 */
class DriftChannel internal constructor(private val name: String) {
    internal sealed interface Method {
        class Sync(val handler: (args: Any?) -> Any?) : Method
        class Async(val handler: (args: Any?, result: DriftResult) -> Unit) : Method
    }

    private val methods = HashMap<String, Method>()

    // Set by the host once the declare block returns; the method map is
    // then read from Go threads and must not change.
    @Volatile
    private var sealed = false

    /**
     * Declares [method] as replying by returning: the return value (JSON-like,
     * or null) is the reply, and a thrown exception fails the call.
     */
    fun method(method: String, handler: (args: Any?) -> Any?) {
        add(method, Method.Sync(handler))
    }

    /**
     * Declares [method] as replying later through its [DriftResult], exactly
     * once, from any thread. An exception thrown before replying fails the
     * call; a result dropped without a reply fails it with `reply_dropped`.
     */
    fun asyncMethod(method: String, handler: (args: Any?, result: DriftResult) -> Unit) {
        add(method, Method.Async(handler))
    }

    private fun add(method: String, m: Method) {
        if (sealed) {
            driftCrash("drift: $name.$method declared after registerChannel returned; declare methods inside its block")
        }
        if (methods.put(method, m) != null) {
            driftCrash("drift: $name.$method declared twice")
        }
    }

    internal fun seal() {
        sealed = true
    }

    internal fun lookup(method: String): Method? = methods[method]
}

/**
 * The reply to one asynchronous method call. Exactly one of [success] or
 * [error] must be called, once, from any thread; a second reply is a
 * programming error and crashes. Until the reply arrives the Go caller
 * waits; a result dropped without a reply fails the call.
 */
class DriftResult internal constructor(
    call: String,
    deliver: (value: Any?, error: Exception?) -> Unit,
) {
    private val reply = Reply(call, deliver)

    init {
        DroppedResults.watch(this, reply)
    }

    /** Replies with a JSON-encodable value (or null). */
    fun success(value: Any?) = reply.submit(value, null)

    /** Replies with an error; Go receives it as the call's error. */
    fun error(error: Exception) = reply.submit(null, error)

    /** Replies with error unless a reply was already submitted. */
    internal fun errorIfPending(error: Exception) {
        reply.submitIfPending(null, error)
    }

    /**
     * The reply state, kept apart from the DriftResult so [DroppedResults]
     * can still reply after the result itself is collected.
     */
    private class Reply(val call: String, val deliver: (Any?, Exception?) -> Unit) {
        private val replied = AtomicBoolean(false)

        fun submit(value: Any?, error: Exception?) {
            if (!submitIfPending(value, error)) driftCrash("drift: $call replied twice")
        }

        fun submitIfPending(value: Any?, error: Exception?): Boolean {
            if (!replied.compareAndSet(false, true)) return false
            deliver(value, error)
            return true
        }
    }

    /**
     * Fails the calls whose DriftResult was garbage-collected without a
     * reply, so Go gets an error instead of waiting forever. (minSdk 31
     * predates java.lang.ref.Cleaner.)
     */
    private object DroppedResults {
        private val queue = ReferenceQueue<DriftResult>()

        // Keeps each reference reachable until its result is collected.
        private val watched = ConcurrentHashMap.newKeySet<Ref>()

        private class Ref(result: DriftResult, val reply: Reply) :
            PhantomReference<DriftResult>(result, queue)

        init {
            thread(isDaemon = true, name = "DriftDroppedResults") {
                while (true) {
                    val ref = queue.remove() as Ref
                    watched.remove(ref)
                    ref.reply.submitIfPending(
                        null,
                        DriftCallError("reply_dropped", "drift: ${ref.reply.call} dropped its DriftResult without replying"),
                    )
                }
            }
        }

        fun watch(result: DriftResult, reply: Reply) {
            watched.add(Ref(result, reply))
        }
    }
}

/** A call failure Go sees with a specific error code. */
internal class DriftCallError(val code: String, message: String) : Exception(message)

/**
 * Fails fast on a programming error: reports to the default uncaught
 * exception handler, which ends the process, then throws in case none is
 * installed. A plain throw could be caught and swallowed on its way back.
 */
internal fun driftCrash(message: String): Nothing {
    val e = IllegalStateException(message)
    Thread.getDefaultUncaughtExceptionHandler()?.uncaughtException(Thread.currentThread(), e)
    throw e
}
