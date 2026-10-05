/**
 * TimePickerChannel.kt
 * The drift/time_picker channel: `show` replies when the user picks a time
 * ({"hour", "minute"}) or cancels (null).
 */
package {{.PackageName}}

import android.app.TimePickerDialog
import android.text.format.DateFormat
import com.drift.runner.DriftCallError
import com.drift.runner.DriftPluginHost

object TimePickerHandler {
    fun start(host: DriftPluginHost) {
        host.registerChannel("drift/time_picker") {
            asyncMethod("show") { args, result ->
                val params = args as? Map<*, *> ?: emptyMap<String, Any?>()
                val activity = PlatformChannelManager.currentActivity()
                    ?: throw DriftCallError("platform_unavailable", "no screen to show the time picker over")
                // null: the system's 12/24-hour preference
                val use24Hour = params["is24Hour"] as? Boolean ?: DateFormat.is24HourFormat(activity)
                var replied = false
                val dialog = TimePickerDialog(
                    activity,
                    { _, hour, minute ->
                        replied = true
                        result.success(mapOf("hour" to hour, "minute" to minute))
                    },
                    (params["hour"] as? Number)?.toInt() ?: 0,
                    (params["minute"] as? Number)?.toInt() ?: 0,
                    use24Hour,
                )
                // Dismissal follows a pick too; anything else is a cancel.
                dialog.setOnDismissListener {
                    if (!replied) {
                        replied = true
                        result.success(null)
                    }
                }
                dialog.show()
            }
        }
    }
}
