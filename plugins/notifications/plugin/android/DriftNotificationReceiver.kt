/**
 * DriftNotificationReceiver.kt
 *
 * Receives the alarms DriftNotificationsPlugin schedules, with or without
 * an Activity: plugins register when the process starts, so the plugin is
 * there to show the notification and report it to Go.
 */
package com.drift.plugin.notifications

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class DriftNotificationReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val notification = Notification.fromIntent(intent) ?: return
        val plugin = checkNotNull(DriftNotificationsPlugin.registered) {
            "drift: DriftNotificationsPlugin not registered; DriftApplication must register plugins at process start"
        }
        plugin.deliver(context, notification)
    }
}
