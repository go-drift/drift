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
 *      sent to Go as `opened`: from the launch intent on the process's
 *      first attach, from new intents after.
 * Mirrors the iOS plugin.
 */
package com.drift.plugin.firebase

import android.app.ActivityManager
import android.content.Intent
import android.os.Bundle
import android.util.Log
import com.drift.runner.DriftActivityBinding
import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost
import com.google.firebase.messaging.FirebaseMessaging
import com.google.firebase.messaging.RemoteMessage

class DriftFirebasePlugin : DriftPlugin {
    private lateinit var host: DriftPluginHost
    /** Whether an Activity has attached in this process. */
    private var attachedBefore = false

    override fun onRegister(host: DriftPluginHost) {
        this.host = host
        registered = this
        FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
            if (task.isSuccessful) {
                onNewToken(task.result)
            } else {
                Log.e(TAG, "FCM token fetch failed", task.exception)
            }
        }
    }

    override fun onAttach(binding: DriftActivityBinding) {
        val first = !attachedBefore
        attachedBefore = true
        val launch = binding.activity.intent
        // A recreated Activity, or one relaunched from Recents, carries the
        // same launch intent again; only the process's first launch counts.
        if (first && launch != null && launch.flags and Intent.FLAG_ACTIVITY_LAUNCHED_FROM_HISTORY == 0) {
            opened(launch)
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
        private const val TOKEN_CHANNEL = "drift/firebase/messaging/token"
        private const val MESSAGE_CHANNEL = "drift/firebase/messaging/message"
        private const val OPENED_CHANNEL = "drift/firebase/messaging/opened"

        /**
         * The registered plugin, for DriftFirebaseMessagingService. Null
         * while no Activity has started in this process (the service can
         * run without one); main thread only.
         */
        internal var registered: DriftFirebasePlugin? = null
            private set

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
