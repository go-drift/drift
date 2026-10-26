/**
 * DriftFirebasePlugin.kt
 *
 * The Firebase plugin's native half on Android, driven by
 * com.drift.runner.DriftPlugins on the main thread. Firebase initialises
 * itself from the resources the google-services Gradle plugin generates.
 *   1. onRegister: fetch the FCM token and send it to Go on
 *      `drift/firebase/messaging/token`. Once per process; refreshes come
 *      through DriftFirebaseMessagingService.
 *   2. Messages that reach the app while it runs (every data message, and
 *      notification messages in the foreground) arrive through
 *      DriftFirebaseMessagingService and go to Go as `message`.
 *   3. Notification messages in the background are shown by the system;
 *      tapping one starts MainActivity with the message in its extras,
 *      sent to Go as `opened`: from the launch intent when it is a new
 *      launch, from new intents after.
 *   4. `drift/firebase/messaging` methods: topic subscriptions, and
 *      deleteToken, which sends "" as the token before replying.
 * Mirrors the iOS plugin.
 */
package com.drift.plugin.firebase

import android.app.ActivityManager
import android.content.Intent
import android.os.Bundle
import android.util.Log
import com.drift.runner.DriftActivityBinding
import com.drift.runner.DriftCallError
import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost
import com.drift.runner.DriftResult
import com.google.android.gms.tasks.Task
import com.google.firebase.messaging.FirebaseMessaging
import com.google.firebase.messaging.RemoteMessage

class DriftFirebasePlugin : DriftPlugin {
    private lateinit var host: DriftPluginHost

    override fun onRegister(host: DriftPluginHost) {
        this.host = host
        registered = this
        host.registerChannel(METHOD_CHANNEL) {
            asyncMethod("subscribeToTopic") { args, result ->
                FirebaseMessaging.getInstance().subscribeToTopic(topic(args)).replyTo(result)
            }
            asyncMethod("unsubscribeFromTopic") { args, result ->
                FirebaseMessaging.getInstance().unsubscribeFromTopic(topic(args)).replyTo(result)
            }
            asyncMethod("deleteToken") { _, result ->
                // Completes on the main thread: Go's Token is "" before the
                // reply, and tokens issued after (also sent from main)
                // follow it.
                FirebaseMessaging.getInstance().deleteToken().addOnCompleteListener { task ->
                    if (task.isSuccessful) {
                        onNewToken("")
                        result.success(null)
                    } else {
                        result.error(task.failure())
                    }
                }
            }
        }
        FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
            if (task.isSuccessful) {
                onNewToken(task.result)
            } else {
                Log.e(TAG, "FCM token fetch failed", task.exception)
            }
        }
    }

    override fun onAttach(binding: DriftActivityBinding) {
        // A recreated Activity, or one relaunched from Recents, carries a
        // launch intent an earlier Activity already handled.
        if (binding.isNewLaunch) {
            binding.activity.intent?.let { opened(it) }
        }
        binding.addOnNewIntentListener { opened(it) }
    }

    internal fun onNewToken(token: String) {
        host.sendEvent(TOKEN_CHANNEL, mapOf("token" to token))
    }

    internal fun onMessage(message: RemoteMessage) {
        val id = message.messageId ?: return
        host.sendEvent(MESSAGE_CHANNEL, payload(
            id = id,
            title = message.notification?.title ?: "",
            body = message.notification?.body ?: "",
            data = message.data,
            foreground = isForeground(),
        ))
    }

    /** Sends a tapped notification's message to Go; true if intent had one. */
    private fun opened(intent: Intent): Boolean {
        val extras = intent.extras ?: return false
        val id = extras.getString("google.message_id") ?: return false
        host.sendEvent(OPENED_CHANNEL, payload(
            id = id,
            // The system shows title and body; the intent carries only data.
            title = "",
            body = "",
            data = senderData(extras),
            foreground = false,
        ))
        return true
    }

    private fun isForeground(): Boolean {
        val info = ActivityManager.RunningAppProcessInfo()
        ActivityManager.getMyMemoryState(info)
        return info.importance == ActivityManager.RunningAppProcessInfo.IMPORTANCE_FOREGROUND
    }

    companion object {
        private const val TAG = "DriftFirebase"
        private const val METHOD_CHANNEL = "drift/firebase/messaging"
        private const val TOKEN_CHANNEL = "drift/firebase/messaging/token"
        private const val MESSAGE_CHANNEL = "drift/firebase/messaging/message"
        private const val OPENED_CHANNEL = "drift/firebase/messaging/opened"

        /**
         * The registered plugin, for DriftFirebaseMessagingService. Plugins
         * register in Application.onCreate, before the service starts;
         * main thread only.
         */
        internal var registered: DriftFirebasePlugin? = null
            private set

        /** The call's topic; Go validated it. */
        private fun topic(args: Any?): String =
            (args as? Map<*, *>)?.get("topic") as? String
                ?: throw IllegalArgumentException("missing topic")

        /** Replies to result when the task completes. */
        private fun Task<Void>.replyTo(result: DriftResult) {
            addOnCompleteListener { task ->
                if (task.isSuccessful) result.success(null) else result.error(task.failure())
            }
        }

        private fun Task<*>.failure(): Exception =
            exception ?: DriftCallError("native_error", "Firebase task failed without an exception")

        /** The wire form of a message (see runtime/messaging). */
        private fun payload(
            id: String,
            title: String,
            body: String,
            data: Map<String, String>,
            foreground: Boolean,
        ): Map<String, Any> = mapOf(
            "id" to id,
            "title" to title,
            "body" to body,
            "data" to data,
            "foreground" to foreground,
        )

        /** The sender's data keys of a tapped notification's extras. */
        private fun senderData(extras: Bundle): Map<String, String> =
            extras.keySet()
                .filter { key ->
                    !key.startsWith("google.") && !key.startsWith("gcm.") &&
                        key != "from" && key != "collapse_key"
                }
                .mapNotNull { key -> extras.getString(key)?.let { key to it } }
                .toMap()
    }
}
