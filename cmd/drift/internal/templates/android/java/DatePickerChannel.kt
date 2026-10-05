/**
 * DatePickerChannel.kt
 * The drift/date_picker channel: `show` replies when the user picks a date
 * ({"year", "month" 1-12, "day"}) or cancels (null). Dates cross the
 * channel as calendar days, so no time zone shifts them.
 */
package {{.PackageName}}

import android.app.DatePickerDialog
import com.drift.runner.DriftCallError
import com.drift.runner.DriftPluginHost
import java.util.Calendar

object DatePickerHandler {
    fun start(host: DriftPluginHost) {
        host.registerChannel("drift/date_picker") {
            asyncMethod("show") { args, result ->
                val params = args as? Map<*, *> ?: emptyMap<String, Any?>()
                val activity = PlatformChannelManager.currentActivity()
                    ?: throw DriftCallError("platform_unavailable", "no screen to show the date picker over")
                val initial = calendarDay(params["initialDate"]) ?: Calendar.getInstance()
                var replied = false
                val dialog = DatePickerDialog(
                    activity,
                    { _, year, month, day ->
                        replied = true
                        result.success(mapOf("year" to year, "month" to month + 1, "day" to day))
                    },
                    initial.get(Calendar.YEAR),
                    initial.get(Calendar.MONTH),
                    initial.get(Calendar.DAY_OF_MONTH),
                )
                calendarDay(params["minDate"])?.let { dialog.datePicker.minDate = it.timeInMillis }
                calendarDay(params["maxDate"])?.let { dialog.datePicker.maxDate = it.timeInMillis }
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

    /** Local midnight of a wire calendar day ({year, month 1-12, day}). */
    private fun calendarDay(value: Any?): Calendar? {
        val day = value as? Map<*, *> ?: return null
        val year = (day["year"] as? Number)?.toInt() ?: return null
        val month = (day["month"] as? Number)?.toInt() ?: return null
        val dayOfMonth = (day["day"] as? Number)?.toInt() ?: return null
        return Calendar.getInstance().apply {
            clear()
            set(year, month - 1, dayOfMonth)
        }
    }
}
