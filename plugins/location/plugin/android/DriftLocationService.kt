/**
 * DriftLocationService.kt
 *
 * The foreground service (type location) that keeps the app's process
 * running, with an ongoing notification, while a background stream of
 * location updates is on. DriftLocationPlugin holds the location request
 * and starts and stops this service around it; the service itself only
 * shows the notification. The build declares it, with its permissions
 * and notification strings, only for the always: config.
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
        startForeground(NOTIFICATION_ID, notification(), ServiceInfo.FOREGROUND_SERVICE_TYPE_LOCATION)
        // The stream (the plugin's request, Go's listeners) dies with the
        // process, so a service restarted after that would track nothing.
        return START_NOT_STICKY
    }

    private fun notification(): Notification {
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
            .setContentTitle(string(TITLE_STRING))
            .setContentText(string(TEXT_STRING))
            .setOngoing(true)
            .setContentIntent(tap)
            .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)
            .build()
    }

    /** A string resource the build wrote (plugin code cannot see R). */
    private fun string(name: String): String {
        val id = resources.getIdentifier(name, "string", packageName)
        check(id != 0) { "drift location: string resource $name missing; the build adds it for the always: config" }
        return resources.getString(id)
    }

    private companion object {
        const val NOTIFICATION_ID = 0x10c
        const val CHANNEL_ID = "drift_location"
        const val CHANNEL_NAME = "Background location"
        // Must match the names in plugin.go.
        const val TITLE_STRING = "drift_location_notification_title"
        const val TEXT_STRING = "drift_location_notification_text"
    }
}
