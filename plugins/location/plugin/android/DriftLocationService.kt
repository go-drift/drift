/**
 * DriftLocationService.kt
 *
 * The foreground service (type location) that keeps the app's process
 * running, with an ongoing notification, while a background stream of
 * location updates is on. DriftLocationPlugin holds the location request
 * and starts and stops this service around it; the service itself only
 * shows the notification, whose text the plugin passes in the start
 * intent (from the always: config). The build declares it, with its
 * permissions, only for that config.
 */
package com.drift.plugin.location

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.IBinder
import androidx.core.app.NotificationCompat

class DriftLocationService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        val start = checkNotNull(intent) { "drift location: started without an intent" }
        val title = checkNotNull(start.getStringExtra(EXTRA_TITLE)) { "drift location: started without its notification title" }
        val text = checkNotNull(start.getStringExtra(EXTRA_TEXT)) { "drift location: started without its notification text" }
        startForeground(NOTIFICATION_ID, notification(title, text), ServiceInfo.FOREGROUND_SERVICE_TYPE_LOCATION)
        // The stream (the plugin's request, Go's listeners) dies with the
        // process, so a service restarted after that would track nothing.
        return START_NOT_STICKY
    }

    private fun notification(title: String, text: String): Notification {
        getSystemService(NotificationManager::class.java).createNotificationChannel(
            NotificationChannel(CHANNEL_ID, CHANNEL_NAME, NotificationManager.IMPORTANCE_LOW),
        )
        // The launcher Activity (MainActivity), which plugin code cannot
        // name directly; a tap brings the app back.
        val launch = checkNotNull(packageManager.getLaunchIntentForPackage(packageName)) {
            "drift location: the app has no launcher Activity"
        }
        val tap = PendingIntent.getActivity(this, 0, launch, PendingIntent.FLAG_IMMUTABLE)
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_menu_mylocation)
            .setContentTitle(title)
            .setContentText(text)
            .setOngoing(true)
            .setContentIntent(tap)
            .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)
            .build()
    }

    companion object {
        /** The start intent's notification title and text. */
        const val EXTRA_TITLE = "com.drift.plugin.location.NOTIFICATION_TITLE"
        const val EXTRA_TEXT = "com.drift.plugin.location.NOTIFICATION_TEXT"

        private const val NOTIFICATION_ID = 0x10c
        private const val CHANNEL_ID = "drift_location"
        private const val CHANNEL_NAME = "Background location"
    }
}
