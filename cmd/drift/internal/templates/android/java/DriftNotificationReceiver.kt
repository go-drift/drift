/**
 * DriftNotificationReceiver.kt
 * Receives scheduled notification alarms.
 */
package {{.PackageName}}

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class DriftNotificationReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        NotificationHandler.handleBroadcast(context, intent, source = "local")
    }
}
