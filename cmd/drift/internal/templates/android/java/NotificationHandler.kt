/**
 * NotificationHandler.kt
 * Provides local and remote notification support for Drift.
 */
package {{.PackageName}}

import android.Manifest
import android.app.AlarmManager
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.SharedPreferences
import android.content.pm.PackageManager
import android.os.Build
import android.util.Log
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import com.google.firebase.FirebaseApp
import com.google.firebase.messaging.FirebaseMessaging

object NotificationHandler {
    private const val defaultChannelId = "drift_default"
    private const val defaultChannelName = "Drift Notifications"
    private const val prefsName = "drift_notifications"
    private const val scheduledKey = "scheduled_ids"

    private const val extraId = "drift_notification_id"
    private const val extraTitle = "drift_notification_title"
    private const val extraBody = "drift_notification_body"
    private const val extraData = "drift_notification_data"
    private const val extraChannel = "drift_notification_channel"
    private const val extraSource = "drift_notification_source"
    private const val TAG = "DriftNotifications"
    private var currentPushToken: String? = null
    private var appContext: Context? = null

    private fun isFirebaseAvailable(): Boolean {
        val ctx = appContext ?: return false
        return try {
            FirebaseApp.getApps(ctx).isNotEmpty()
        } catch (e: Exception) {
            false
        }
    }

    private fun firebaseNotConfiguredError(): Pair<Any?, Exception?> {
        return Pair(null, IllegalStateException(
            "Firebase is not configured. Add a google-services.json file to your Android app module to enable push notifications."
        ))
    }

    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        if (appContext == null) appContext = context.applicationContext
        return when (method) {
            "getSettings" -> Pair(getSettings(context), null)
            "schedule" -> scheduleLocal(context, args)
            "cancel" -> cancelLocal(context, args)
            "cancelAll" -> cancelAll(context)
            "setBadge" -> Pair(null, null)
            "registerForPush" -> registerForPush()
            "getPushToken" -> getPushToken()
            "subscribeToTopic" -> subscribeToTopic(args)
            "unsubscribeFromTopic" -> unsubscribeFromTopic(args)
            "deletePushToken" -> deletePushToken()
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    fun handleNotificationOpen(intent: Intent?) {
        val payload = parsePayload(intent) ?: return
        sendOpened(payload, action = "tap")
    }

    fun handleBroadcast(context: Context, intent: Intent, source: String) {
        val payload = parsePayload(intent) ?: return
        val isForeground = PlatformChannelManager.isAppForeground()
        sendReceived(payload, isForeground = isForeground, source = source)
        if (!isForeground) {
            showNotification(context, payload, source)
        }
    }

    fun handleRemoteMessage(context: Context, title: String?, body: String?, data: Map<String, Any?>?) {
        val payload = NotificationPayload(
            id = data?.get("id") as? String ?: System.currentTimeMillis().toString(),
            title = title ?: "",
            body = body ?: "",
            data = data ?: emptyMap(),
            channelId = null
        )
        val isForeground = PlatformChannelManager.isAppForeground()
        sendReceived(payload, isForeground = isForeground, source = "remote")
        if (!isForeground) {
            showNotification(context, payload, "remote")
        }
    }

    fun handleNewToken(token: String, isRefresh: Boolean = true) {
        val payload = mapOf(
            "platform" to "android",
            "token" to token,
            "timestamp" to System.currentTimeMillis(),
            "isRefresh" to isRefresh
        )
        PlatformChannelManager.sendEvent("drift/notifications/token", payload)
    }

    // region Push notification methods

    private fun registerForPush(): Pair<Any?, Exception?> {
        if (!isFirebaseAvailable()) return firebaseNotConfiguredError()
        try {
            FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
                if (task.isSuccessful) {
                    val token = task.result
                    currentPushToken = token
                    Log.d(TAG, "FCM registration token: $token")
                    handleNewToken(token, isRefresh = false)
                } else {
                    Log.e(TAG, "Failed to get FCM token", task.exception)
                    sendPushError("registration_failed", task.exception?.message ?: "Unknown error")
                }
            }
            return Pair(null, null)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to register for push", e)
            return Pair(null, e)
        }
    }

    private fun getPushToken(): Pair<Any?, Exception?> {
        if (currentPushToken != null) {
            return Pair(mapOf("token" to currentPushToken), null)
        }
        if (!isFirebaseAvailable()) return firebaseNotConfiguredError()

        var token: String? = null
        var error: Exception? = null
        val latch = java.util.concurrent.CountDownLatch(1)

        try {
            FirebaseMessaging.getInstance().token.addOnCompleteListener { task ->
                if (task.isSuccessful) {
                    token = task.result
                    currentPushToken = token
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
        if (!isFirebaseAvailable()) return firebaseNotConfiguredError()
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
        if (!isFirebaseAvailable()) return firebaseNotConfiguredError()
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

    private fun deletePushToken(): Pair<Any?, Exception?> {
        if (!isFirebaseAvailable()) return firebaseNotConfiguredError()
        var error: Exception? = null
        val latch = java.util.concurrent.CountDownLatch(1)

        try {
            FirebaseMessaging.getInstance().deleteToken().addOnCompleteListener { task ->
                if (task.isSuccessful) {
                    currentPushToken = null
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

    private fun sendPushError(code: String, message: String) {
        PlatformChannelManager.sendEvent("drift/notifications/error", mapOf(
            "code" to code,
            "message" to message,
            "platform" to "android"
        ))
    }

    // endregion

    private fun getSettings(context: Context): Map<String, Any> {
        return mapOf(
            "status" to permissionStatus(context),
            "alertsEnabled" to notificationsEnabled(context),
            "soundsEnabled" to notificationsEnabled(context),
            "badgesEnabled" to notificationsEnabled(context)
        )
    }

    private fun scheduleLocal(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *> ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val id = argsMap["id"] as? String ?: return Pair(null, IllegalArgumentException("Missing id"))
        val title = argsMap["title"] as? String ?: ""
        val body = argsMap["body"] as? String ?: ""
        val data = argsMap["data"] as? Map<String, Any?>
        val channelId = argsMap["channelId"] as? String
        val atMillis = (argsMap["at"] as? Number)?.toLong()
        val intervalSeconds = (argsMap["intervalSeconds"] as? Number)?.toLong() ?: 0L
        val repeats = argsMap["repeats"] as? Boolean ?: false

        val payload = NotificationPayload(id = id, title = title, body = body, data = data ?: emptyMap(), channelId = channelId)
        val triggerAt = atMillis
            ?: if (intervalSeconds > 0) System.currentTimeMillis() + intervalSeconds * 1000
               else System.currentTimeMillis() + 1000

        scheduleAlarm(context, payload, triggerAt, repeats, intervalSeconds)
        return Pair(null, null)
    }

    private fun cancelLocal(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *> ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val id = argsMap["id"] as? String ?: return Pair(null, IllegalArgumentException("Missing id"))
        cancelAlarm(context, id)
        NotificationManagerCompat.from(context).cancel(id.hashCode())
        return Pair(null, null)
    }

    private fun cancelAll(context: Context): Pair<Any?, Exception?> {
        val prefs = prefs(context)
        val ids = prefs.getStringSet(scheduledKey, emptySet()) ?: emptySet()
        ids.forEach { cancelAlarm(context, it) }
        prefs.edit().remove(scheduledKey).apply()
        NotificationManagerCompat.from(context).cancelAll()
        return Pair(null, null)
    }

    private fun scheduleAlarm(context: Context, payload: NotificationPayload, triggerAt: Long, repeats: Boolean, intervalSeconds: Long) {
        val alarmManager = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
        val intent = Intent(context, DriftNotificationReceiver::class.java)
        putPayloadExtras(intent, payload, source = "local")
        val pendingIntent = PendingIntent.getBroadcast(
            context,
            payload.id.hashCode(),
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        if (repeats && intervalSeconds > 0) {
            alarmManager.setRepeating(AlarmManager.RTC_WAKEUP, triggerAt, intervalSeconds * 1000, pendingIntent)
        } else if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            try {
                alarmManager.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, triggerAt, pendingIntent)
            } catch (e: SecurityException) {
                alarmManager.set(AlarmManager.RTC_WAKEUP, triggerAt, pendingIntent)
            }
        } else {
            alarmManager.setExact(AlarmManager.RTC_WAKEUP, triggerAt, pendingIntent)
        }
        trackScheduled(context, payload.id)
    }

    private fun cancelAlarm(context: Context, id: String) {
        val alarmManager = context.getSystemService(Context.ALARM_SERVICE) as AlarmManager
        val intent = Intent(context, DriftNotificationReceiver::class.java)
        val pendingIntent = PendingIntent.getBroadcast(
            context,
            id.hashCode(),
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        alarmManager.cancel(pendingIntent)
        untrackScheduled(context, id)
    }

    private fun showNotification(context: Context, payload: NotificationPayload, source: String) {
        ensureChannel(context, payload.channelId)
        val intent = Intent(context, MainActivity::class.java).apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP)
            putPayloadExtras(this, payload, source)
        }
        val pendingIntent = PendingIntent.getActivity(
            context,
            payload.id.hashCode(),
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
        )
        val notification = NotificationCompat.Builder(context, payload.channelId ?: defaultChannelId)
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setContentTitle(payload.title)
            .setContentText(payload.body)
            .setAutoCancel(true)
            .setContentIntent(pendingIntent)
            .build()
        NotificationManagerCompat.from(context).notify(payload.id.hashCode(), notification)
    }

    private fun sendReceived(payload: NotificationPayload, isForeground: Boolean, source: String) {
        val event = mapOf(
            "id" to payload.id,
            "title" to payload.title,
            "body" to payload.body,
            "data" to payload.data,
            "timestamp" to System.currentTimeMillis(),
            "isForeground" to isForeground,
            "source" to source
        )
        PlatformChannelManager.sendEvent("drift/notifications/received", event)
    }

    private fun sendOpened(payload: NotificationPayload, action: String) {
        val event = mapOf(
            "id" to payload.id,
            "data" to payload.data,
            "action" to action,
            "source" to (payload.source ?: "local"),
            "timestamp" to System.currentTimeMillis()
        )
        PlatformChannelManager.sendEvent("drift/notifications/opened", event)
    }

    private fun ensureChannel(context: Context, channelId: String?) {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val id = channelId ?: defaultChannelId
        val manager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (manager.getNotificationChannel(id) == null) {
            val channel = NotificationChannel(id, defaultChannelName, NotificationManager.IMPORTANCE_DEFAULT)
            manager.createNotificationChannel(channel)
        }
    }

    private fun putPayloadExtras(intent: Intent, payload: NotificationPayload, source: String) {
        val encoder = JsonCodec
        intent.putExtra(extraId, payload.id)
        intent.putExtra(extraTitle, payload.title)
        intent.putExtra(extraBody, payload.body)
        intent.putExtra(extraChannel, payload.channelId)
        intent.putExtra(extraSource, source)
        intent.putExtra(extraData, encoder.encode(payload.data))
    }

    private fun parsePayload(intent: Intent?): NotificationPayload? {
        if (intent == null) return null
        val id = intent.getStringExtra(extraId) ?: return null
        val title = intent.getStringExtra(extraTitle) ?: ""
        val body = intent.getStringExtra(extraBody) ?: ""
        val channelId = intent.getStringExtra(extraChannel)
        val source = intent.getStringExtra(extraSource)
        val dataBytes = intent.getByteArrayExtra(extraData)
        val data = if (dataBytes != null && dataBytes.isNotEmpty()) {
            @Suppress("UNCHECKED_CAST")
            JsonCodec.decode(dataBytes) as? Map<String, Any?> ?: emptyMap()
        } else {
            emptyMap()
        }
        return NotificationPayload(id = id, title = title, body = body, data = data, channelId = channelId, source = source)
    }

    private fun permissionStatus(context: Context): String {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            val granted = ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
            return if (granted) "granted" else "denied"
        }
        return if (notificationsEnabled(context)) "granted" else "denied"
    }

    private fun notificationsEnabled(context: Context): Boolean {
        return NotificationManagerCompat.from(context).areNotificationsEnabled()
    }

    private fun prefs(context: Context): SharedPreferences {
        return context.getSharedPreferences(prefsName, Context.MODE_PRIVATE)
    }

    private fun trackScheduled(context: Context, id: String) {
        val prefs = prefs(context)
        val current = prefs.getStringSet(scheduledKey, emptySet())?.toMutableSet() ?: mutableSetOf()
        current.add(id)
        prefs.edit().putStringSet(scheduledKey, current).apply()
    }

    private fun untrackScheduled(context: Context, id: String) {
        val prefs = prefs(context)
        val current = prefs.getStringSet(scheduledKey, emptySet())?.toMutableSet() ?: mutableSetOf()
        if (current.remove(id)) {
            prefs.edit().putStringSet(scheduledKey, current).apply()
        }
    }
}

data class NotificationPayload(
    val id: String,
    val title: String,
    val body: String,
    val data: Map<String, Any?>,
    val channelId: String?,
    val source: String? = null
)
