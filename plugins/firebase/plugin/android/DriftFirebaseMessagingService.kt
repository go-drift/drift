/**
 * DriftFirebaseMessagingService.kt
 *
 * Firebase delivers new tokens and the messages that reach the app (every
 * data message, and notification messages in the foreground) to this
 * service, on a worker thread. It hands them to the registered
 * DriftFirebasePlugin on the main thread. With no Activity started in the
 * process yet there is no plugin and no running app to deliver to: the
 * event is dropped (the token is fetched again when the app starts).
 */
package com.drift.plugin.firebase

import android.os.Handler
import android.os.Looper
import android.util.Log
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage

class DriftFirebaseMessagingService : FirebaseMessagingService() {
    private val main = Handler(Looper.getMainLooper())

    override fun onNewToken(token: String) {
        main.post {
            DriftFirebasePlugin.registered?.onNewToken(token)
                ?: Log.i(TAG, "FCM token refreshed with the app not running; it is sent at the next start")
        }
    }

    override fun onMessageReceived(message: RemoteMessage) {
        main.post {
            DriftFirebasePlugin.registered?.onMessage(message)
                ?: Log.w(TAG, "FCM message ${message.messageId} arrived with the app not running; dropped")
        }
    }

    private companion object {
        const val TAG = "DriftFirebase"
    }
}
