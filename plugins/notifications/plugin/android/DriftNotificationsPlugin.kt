/**
 * DriftNotificationsPlugin.kt
 *
 * The notifications plugin's native half on Android, on the main thread.
 * Schedules local notifications with AlarmManager: when one is due,
 * DriftNotificationReceiver shows it and reports it to Go (`received`). A
 * tap launches the app with the notification in the intent's extras,
 * reported to Go as `opened`: from the launch intent when it is a new
 * launch, from new intents after. Only intents carrying `EXTRA_ID` are
 * this plugin's; push notifications belong to the Firebase plugin.
 */
package com.drift.plugin.notifications

import android.app.ActivityManager
import android.app.AlarmManager
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.service.notification.StatusBarNotification
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import com.drift.runner.DriftActivityBinding
import com.drift.runner.DriftCallError
import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost
import org.json.JSONArray
import org.json.JSONObject

class DriftNotificationsPlugin : DriftPlugin {
    private lateinit var host: DriftPluginHost

    override fun onRegister(host: DriftPluginHost) {
        this.host = host
        registered = this
        val context = host.context
        host.registerChannel(CHANNEL) {
            // Async to match iOS, where scheduling replies from a
            // completion handler: Go calls it from a goroutine on both.
            asyncMethod("schedule") { args, result ->
                schedule(context, Notification.fromArgs(args))
                result.success(null)
            }
            method("cancel") { args ->
                val id = (args as? Map<*, *>)?.get("id") as? String
                    ?: throw IllegalArgumentException("cancel needs an id")
                cancel(context, id)
                null
            }
            method("cancelAll") { _ ->
                // Shown notifications that will not fire again have left
                // Scheduled; the shade still holds them.
                val shown = context.getSystemService(NotificationManager::class.java)
                    .activeNotifications.mapNotNull { shownID(it) }
                (Scheduled.all(context) + shown).forEach { cancel(context, it) }
                null
            }
            // Android has no app badge count; launchers show their own.
            method("setBadge") { _ -> null }
            asyncMethod("getSettings") { _, result ->
                val enabled = NotificationManagerCompat.from(context).areNotificationsEnabled()
                result.success(mapOf(
                    "alertsEnabled" to enabled,
                    "soundsEnabled" to enabled,
                    "badgesEnabled" to enabled,
                ))
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

    /** Reports a tap on one of this plugin's notifications; true if it was one. */
    private fun opened(intent: Intent): Boolean {
        val notification = Notification.fromIntent(intent) ?: return false
        host.sendEvent(OPENED_CHANNEL, mapOf("id" to notification.id, "data" to notification.data))
        return true
    }

    /** Shows a due notification and reports it to Go (from the receiver). */
    internal fun deliver(context: Context, notification: Notification) {
        show(context, notification)
        host.sendEvent(RECEIVED_CHANNEL, mapOf(
            "id" to notification.id,
            "title" to notification.title,
            "body" to notification.body,
            "data" to notification.data,
            "foreground" to isForeground(),
        ))
        if (!notification.repeats) Scheduled.remove(context, notification.id)
    }

    private fun schedule(context: Context, n: Notification) {
        val alarms = context.getSystemService(AlarmManager::class.java)
        val at = n.atMillis
            ?: (System.currentTimeMillis() + maxOf(n.intervalSeconds, 1L) * 1000)
        val alarm = alarmIntent(context, n.id, n, PendingIntent.FLAG_UPDATE_CURRENT)!!
        when {
            // Go checked Repeats: no At, an interval of at least a minute.
            n.repeats -> alarms.setRepeating(AlarmManager.RTC_WAKEUP, at, n.intervalSeconds * 1000, alarm)
            alarms.canScheduleExactAlarms() -> alarms.setExactAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, alarm)
            else -> alarms.setAndAllowWhileIdle(AlarmManager.RTC_WAKEUP, at, alarm)
        }
        Scheduled.add(context, n.id)
    }

    private fun cancel(context: Context, id: String) {
        alarmIntent(context, id, null, PendingIntent.FLAG_NO_CREATE)?.let {
            context.getSystemService(AlarmManager::class.java).cancel(it)
            it.cancel()
        }
        NotificationManagerCompat.from(context).cancel(TAG_PREFIX + id, NOTIFICATION_TAG_ID)
        Scheduled.remove(context, id)
    }

    /**
     * The alarm's broadcast, one per notification ID (its action), carrying
     * n when given. flags: FLAG_UPDATE_CURRENT to create or update it,
     * FLAG_NO_CREATE to look up an existing one (null if none).
     */
    private fun alarmIntent(context: Context, id: String, n: Notification?, flags: Int): PendingIntent? {
        val intent = Intent(context, DriftNotificationReceiver::class.java)
            .setAction(ACTION_DELIVER + id)
        n?.putInto(intent)
        return PendingIntent.getBroadcast(context, 0, intent, flags or PendingIntent.FLAG_IMMUTABLE)
    }

    private fun show(context: Context, n: Notification) {
        val channelId = n.channelId ?: DEFAULT_CHANNEL
        val manager = context.getSystemService(NotificationManager::class.java)
        if (manager.getNotificationChannel(channelId) == null) {
            val name = if (channelId == DEFAULT_CHANNEL) DEFAULT_CHANNEL_NAME else channelId
            manager.createNotificationChannel(NotificationChannel(channelId, name, NotificationManager.IMPORTANCE_DEFAULT))
        }
        // The launcher Activity (MainActivity), which plugin code cannot
        // name directly.
        val launch = checkNotNull(context.packageManager.getLaunchIntentForPackage(context.packageName)) {
            "drift notifications: the app has no launcher Activity"
        }.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP)
            .setAction(ACTION_OPEN + n.id)
        n.putInto(launch)
        val tap = PendingIntent.getActivity(context, 0, launch,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
        val notification = NotificationCompat.Builder(context, channelId)
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setContentTitle(n.title)
            .setContentText(n.body)
            .setAutoCancel(true)
            .setContentIntent(tap)
            .build()
        // Posting needs POST_NOTIFICATIONS, which the app may not have; the
        // system then drops it, as it does for any app.
        try {
            NotificationManagerCompat.from(context).notify(TAG_PREFIX + n.id, NOTIFICATION_TAG_ID, notification)
        } catch (_: SecurityException) {
        }
    }

    /** The ID of a shown notification if this plugin posted it, else null. */
    private fun shownID(shown: StatusBarNotification): String? {
        if (shown.id != NOTIFICATION_TAG_ID) return null
        return shown.tag?.takeIf { it.startsWith(TAG_PREFIX) }?.removePrefix(TAG_PREFIX)
    }

    private fun isForeground(): Boolean {
        val info = ActivityManager.RunningAppProcessInfo()
        ActivityManager.getMyMemoryState(info)
        return info.importance == ActivityManager.RunningAppProcessInfo.IMPORTANCE_FOREGROUND
    }

    companion object {
        private const val CHANNEL = "drift/notifications"
        private const val RECEIVED_CHANNEL = "drift/notifications/received"
        private const val OPENED_CHANNEL = "drift/notifications/opened"
        private const val DEFAULT_CHANNEL = "drift_notifications"
        private const val DEFAULT_CHANNEL_NAME = "Notifications"
        // Notifications are posted with this id and TAG_PREFIX plus their
        // ID as the tag. The prefix tells them from the app's other posts:
        // Firebase posts push notifications with id 0 and its own tags.
        private const val NOTIFICATION_TAG_ID = 0
        private const val TAG_PREFIX = "com.drift.plugin.notifications/"
        private const val ACTION_DELIVER = "com.drift.plugin.notifications.DELIVER/"
        private const val ACTION_OPEN = "com.drift.plugin.notifications.OPEN/"

        /**
         * The registered plugin, for DriftNotificationReceiver. Plugins
         * register in Application.onCreate, before the receiver runs; main
         * thread only.
         */
        internal var registered: DriftNotificationsPlugin? = null
            private set
    }
}

/** A notification as scheduled from Go and carried in intent extras. */
internal data class Notification(
    val id: String,
    val title: String,
    val body: String,
    val data: Map<String, Any?>,
    val atMillis: Long?,
    val intervalSeconds: Long,
    val repeats: Boolean,
    val channelId: String?,
) {
    fun putInto(intent: Intent) {
        intent.putExtra(EXTRA_ID, id)
            .putExtra(EXTRA_TITLE, title)
            .putExtra(EXTRA_BODY, body)
            .putExtra(EXTRA_DATA, JSONObject(data).toString())
            .putExtra(EXTRA_REPEATS, repeats)
            .putExtra(EXTRA_CHANNEL, channelId)
    }

    companion object {
        const val EXTRA_ID = "drift.notification.id"
        private const val EXTRA_TITLE = "drift.notification.title"
        private const val EXTRA_BODY = "drift.notification.body"
        private const val EXTRA_DATA = "drift.notification.data"
        private const val EXTRA_REPEATS = "drift.notification.repeats"
        private const val EXTRA_CHANNEL = "drift.notification.channel"

        fun fromArgs(args: Any?): Notification {
            val m = args as? Map<*, *> ?: throw IllegalArgumentException("schedule needs arguments")
            val id = m["id"] as? String
            if (id.isNullOrEmpty()) throw DriftCallError("invalid_arguments", "schedule needs an id")
            @Suppress("UNCHECKED_CAST")
            return Notification(
                id = id,
                title = m["title"] as? String ?: "",
                body = m["body"] as? String ?: "",
                data = m["data"] as? Map<String, Any?> ?: emptyMap(),
                atMillis = (m["at"] as? Number)?.toLong(),
                intervalSeconds = (m["intervalSeconds"] as? Number)?.toLong() ?: 0L,
                repeats = m["repeats"] as? Boolean ?: false,
                channelId = (m["channelId"] as? String)?.takeIf { it.isNotEmpty() },
            )
        }

        /** The notification an intent carries, or null if it carries none. */
        fun fromIntent(intent: Intent): Notification? {
            val id = intent.getStringExtra(EXTRA_ID) ?: return null
            return Notification(
                id = id,
                title = intent.getStringExtra(EXTRA_TITLE) ?: "",
                body = intent.getStringExtra(EXTRA_BODY) ?: "",
                data = intent.getStringExtra(EXTRA_DATA)?.let { toMap(JSONObject(it)) } ?: emptyMap(),
                atMillis = null,
                intervalSeconds = 0,
                repeats = intent.getBooleanExtra(EXTRA_REPEATS, false),
                channelId = intent.getStringExtra(EXTRA_CHANNEL),
            )
        }

        private fun toMap(json: JSONObject): Map<String, Any?> =
            json.keys().asSequence().associateWith { fromJson(json.get(it)) }

        private fun fromJson(value: Any?): Any? = when (value) {
            JSONObject.NULL -> null
            is JSONObject -> toMap(value)
            is JSONArray -> (0 until value.length()).map { fromJson(value.get(it)) }
            else -> value
        }
    }
}

/** The IDs of scheduled notifications, for cancelAll. */
private object Scheduled {
    private const val PREFS = "drift_notifications"
    private const val KEY = "scheduled"

    fun all(context: Context): Set<String> = prefs(context).getStringSet(KEY, emptySet()).orEmpty().toSet()

    fun add(context: Context, id: String) = update(context) { it + id }

    fun remove(context: Context, id: String) = update(context) { it - id }

    private fun update(context: Context, change: (Set<String>) -> Set<String>) {
        prefs(context).edit().putStringSet(KEY, change(all(context))).apply()
    }

    private fun prefs(context: Context) = context.getSharedPreferences(PREFS, Context.MODE_PRIVATE)
}
