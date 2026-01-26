/**
 * DatePickerChannel.kt
 * Provides native date picker dialog for Drift.
 */
package {{.PackageName}}

import android.app.DatePickerDialog
import java.util.Calendar
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

/**
 * Handles date picker channel methods from Go.
 */
object DatePickerHandler {
    fun handle(method: String, args: Any?): Pair<Any?, Exception?> {
        if (method != "show") {
            return Pair(null, IllegalArgumentException("Unknown method: $method"))
        }

        val params = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))

        // Parse initial date
        val initialTimestamp = (params["initialDate"] as? Number)?.toLong()
            ?: (System.currentTimeMillis() / 1000)
        val calendar = Calendar.getInstance()
        calendar.timeInMillis = initialTimestamp * 1000

        // Parse min/max dates
        val minTimestamp = (params["minDate"] as? Number)?.toLong()
        val maxTimestamp = (params["maxDate"] as? Number)?.toLong()

        val activity = PlatformChannelManager.currentActivity()
            ?: return Pair(null, IllegalStateException("No active activity"))

        // Show picker on main thread and wait for result
        var result: Long? = null
        val latch = CountDownLatch(1)

        activity.runOnUiThread {
            val year = calendar.get(Calendar.YEAR)
            val month = calendar.get(Calendar.MONTH)
            val day = calendar.get(Calendar.DAY_OF_MONTH)

            val dialog = DatePickerDialog(
                activity,
                { _, selectedYear, selectedMonth, selectedDay ->
                    val selectedCalendar = Calendar.getInstance()
                    selectedCalendar.set(selectedYear, selectedMonth, selectedDay, 0, 0, 0)
                    selectedCalendar.set(Calendar.MILLISECOND, 0)
                    result = selectedCalendar.timeInMillis / 1000
                    latch.countDown()
                },
                year,
                month,
                day
            )

            // Set min/max dates if provided
            minTimestamp?.let {
                dialog.datePicker.minDate = it * 1000
            }
            maxTimestamp?.let {
                dialog.datePicker.maxDate = it * 1000
            }

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
        return if (result != null) {
            Pair(mapOf("timestamp" to result), null)
        } else {
            Pair(null, null)
        }
    }
}
