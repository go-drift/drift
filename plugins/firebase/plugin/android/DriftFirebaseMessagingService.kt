/**
 * DriftFirebaseMessagingService.kt
 *
 * Firebase delivers new tokens and the messages that reach the app (every
 * data message, and notification messages in the foreground) to this
 * service, on a worker thread. It hands them to the registered
 * DriftFirebasePlugin on the main thread, which sends them to Go. Plugins
 * register when the process starts, so this works with no Activity: Go
 * keeps messages queued until the app listens.
 */
package com.drift.plugin.firebase

import android.os.Handler
import android.os.Looper
import com.google.firebase.messaging.FirebaseMessagingService
import com.google.firebase.messaging.RemoteMessage

class DriftFirebaseMessagingService : FirebaseMessagingService() {
    private val main = Handler(Looper.getMainLooper())

    override fun onNewToken(token: String) {
        main.post {
            plugin().onNewToken(token)
        }
    }

    override fun onMessageReceived(message: RemoteMessage) {
        main.post {
            plugin().onMessage(message)
        }
    }

    private fun plugin(): DriftFirebasePlugin = checkNotNull(DriftFirebasePlugin.registered) {
        "drift: DriftFirebasePlugin not registered; DriftApplication must register plugins at process start"
    }
}
