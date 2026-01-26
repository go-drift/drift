/**
 * TimePickerChannel.kt
 * Provides native time picker dialog for Drift.
 */
package {{.PackageName}}

import android.app.TimePickerDialog
import android.text.format.DateFormat
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

/**
 * Handles time picker channel methods from Go.
 */
object TimePickerHandler {
    fun handle(method: String, args: Any?): Pair<Any?, Exception?> {
        if (method != "show") {
            return Pair(null, IllegalArgumentException("Unknown method: $method"))
        }

        val params = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))

        // Parse initial time
        val hour = (params["hour"] as? Number)?.toInt() ?: 0
        val minute = (params["minute"] as? Number)?.toInt() ?: 0

        // Check if 24-hour format is specified, otherwise use system default
        val is24Hour = when (val is24Param = params["is24Hour"]) {
            is Boolean -> is24Param
            else -> null
        }

        val activity = PlatformChannelManager.currentActivity()
            ?: return Pair(null, IllegalStateException("No active activity"))

        // Show picker on main thread and wait for result
        var resultHour: Int? = null
        var resultMinute: Int? = null
        val latch = CountDownLatch(1)

        activity.runOnUiThread {
            val use24Hour = is24Hour ?: DateFormat.is24HourFormat(activity)

            val dialog = TimePickerDialog(
                activity,
                { _, selectedHour, selectedMinute ->
                    resultHour = selectedHour
                    resultMinute = selectedMinute
                    latch.countDown()
                },
                hour,
                minute,
                use24Hour
            )

            // Handle cancel
            dialog.setOnCancelListener {
                latch.countDown()
            }

            // Handle dismiss without selection
            dialog.setOnDismissListener {
                // Only countdown if not already done (selection or cancel)
                if (latch.count > 0) {
                    latch.countDown()
                }
            }

            dialog.show()
        }

        // Wait for user selection (with timeout)
        val completed = latch.await(300, TimeUnit.SECONDS)
        if (!completed) {
            return Pair(null, Exception("Picker timeout"))
        }

        // Return result (null means cancelled)
        return if (resultHour != null && resultMinute != null) {
            Pair(mapOf("hour" to resultHour, "minute" to resultMinute), null)
        } else {
            Pair(null, null)
        }
    }
}
