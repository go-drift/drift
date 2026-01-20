/**
 * PushHandler.kt
 * Handles push notification registration and token management for the Drift platform channel.
 */
package {{.PackageName}}

import android.content.Context
import android.util.Log
import com.google.firebase.messaging.FirebaseMessaging

object PushHandler {
    private const val TAG = "DriftPush"
    private var currentToken: String? = null

    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        return when (method) {
            "register" -> register()
            "getToken" -> getToken()
            "subscribeToTopic" -> subscribeToTopic(args)
            "unsubscribeFromTopic" -> unsubscribeFromTopic(args)
            "deleteToken" -> deleteToken()
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun register(): Pair<Any?, Exception?> {
        try {
            FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
                if (task.isSuccessful) {
                    val token = task.result
                    currentToken = token
                    Log.d(TAG, "FCM registration token: $token")
                    sendTokenUpdate(token)
                } else {
                    Log.e(TAG, "Failed to get FCM token", task.exception)
                    sendError("registration_failed", task.exception?.message ?: "Unknown error")
                }
            }
            return Pair(null, null)
        } catch (e: Exception) {
            Log.e(TAG, "Firebase not configured", e)
            return Pair(null, e)
        }
    }

    private fun getToken(): Pair<Any?, Exception?> {
        if (currentToken != null) {
            return Pair(mapOf("token" to currentToken), null)
        }

        var token: String? = null
        var error: Exception? = null
        val latch = java.util.concurrent.CountDownLatch(1)

        try {
            FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
                if (task.isSuccessful) {
                    token = task.result
                    currentToken = token
                } else {
                    error = task.exception
                }
                latch.countDown()
            }

            latch.await(10, java.util.concurrent.TimeUnit.SECONDS)
        } catch (e: Exception) {
            error = e
        }

        return if (error != null) {
            Pair(null, error)
        } else {
            Pair(mapOf("token" to token), null)
        }
    }

    private fun subscribeToTopic(args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val topic = argsMap["topic"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing topic"))

        var error: Exception? = null
        val latch = java.util.concurrent.CountDownLatch(1)

        try {
            FirebaseMessaging.getInstance().subscribeToTopic(topic).addOnCompleteListener { task ->
                if (!task.isSuccessful) {
                    error = task.exception
                }
                latch.countDown()
            }

            latch.await(10, java.util.concurrent.TimeUnit.SECONDS)
        } catch (e: Exception) {
            error = e
        }

        return if (error != null) {
            Pair(null, error)
        } else {
            Pair(null, null)
        }
    }

    private fun unsubscribeFromTopic(args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val topic = argsMap["topic"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing topic"))

        var error: Exception? = null
        val latch = java.util.concurrent.CountDownLatch(1)

        try {
            FirebaseMessaging.getInstance().unsubscribeFromTopic(topic).addOnCompleteListener { task ->
                if (!task.isSuccessful) {
                    error = task.exception
                }
                latch.countDown()
            }

            latch.await(10, java.util.concurrent.TimeUnit.SECONDS)
        } catch (e: Exception) {
            error = e
        }

        return if (error != null) {
            Pair(null, error)
        } else {
            Pair(null, null)
        }
    }

    private fun deleteToken(): Pair<Any?, Exception?> {
        var error: Exception? = null
        val latch = java.util.concurrent.CountDownLatch(1)

        try {
            FirebaseMessaging.getInstance().deleteToken().addOnCompleteListener { task ->
                if (task.isSuccessful) {
                    currentToken = null
                } else {
                    error = task.exception
                }
                latch.countDown()
            }

            latch.await(10, java.util.concurrent.TimeUnit.SECONDS)
        } catch (e: Exception) {
            error = e
        }

        return if (error != null) {
            Pair(null, error)
        } else {
            Pair(null, null)
        }
    }

    fun handleNewToken(token: String) {
        currentToken = token
        Log.d(TAG, "New FCM token: $token")
        sendTokenUpdate(token)
    }

    private fun sendTokenUpdate(token: String) {
        PlatformChannelManager.sendEvent("drift/push/token", mapOf(
            "platform" to "android",
            "token" to token,
            "timestamp" to System.currentTimeMillis()
        ))
    }

    private fun sendError(code: String, message: String) {
        PlatformChannelManager.sendEvent("drift/push/error", mapOf(
            "code" to code,
            "message" to message
        ))
    }
}
