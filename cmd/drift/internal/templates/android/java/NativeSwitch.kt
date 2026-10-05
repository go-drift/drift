/**
 * NativeSwitch.kt
 * Provides native SwitchCompat embedded in Drift UI.
 */
package {{.PackageName}}

import com.drift.runner.DriftPlatformView
import android.content.Context
import android.content.res.ColorStateList
import android.view.View
import android.widget.FrameLayout
import androidx.appcompat.widget.SwitchCompat

/**
 * Platform view container for native switch.
 */
class NativeSwitchContainer(
    context: Context,
    val viewId: Int,
    params: Map<String, Any?>
) : DriftPlatformView {

    override val view: View
    private val switch: SwitchCompat

    init {
        switch = SwitchCompat(context).apply {
            // Apply styling
            (params["onTintColor"] as? Number)?.let { color ->
                trackTintList = ColorStateList.valueOf(color.toInt())
            }
            (params["thumbTintColor"] as? Number)?.let { color ->
                thumbTintList = ColorStateList.valueOf(color.toInt())
            }

            // Set initial value
            isChecked = params["value"] as? Boolean ?: false

            // Add listener for value changes
            setOnCheckedChangeListener { _, isChecked ->
                PlatformChannelManager.sendEvent(
                    "drift/platform_views",
                    mapOf(
                        "method" to "onSwitchChanged",
                        "viewId" to viewId,
                        "value" to isChecked
                    )
                )
            }

            layoutParams = FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.WRAP_CONTENT,
                FrameLayout.LayoutParams.WRAP_CONTENT
            )
        }

        view = switch
    }

    override fun handle(method: String, args: Map<String, Any?>) {
        when (method) {
            "setValue" -> setValue(args["value"] as? Boolean ?: false)
            "updateConfig" -> updateConfig(args)
            else -> throw unknownViewMethod(method)
        }
    }

    override fun dispose() {
        switch.setOnCheckedChangeListener(null)
    }

    fun setValue(value: Boolean) {
        // Temporarily remove listener to avoid feedback loop
        switch.setOnCheckedChangeListener(null)
        switch.isChecked = value
        switch.setOnCheckedChangeListener { _, isChecked ->
            PlatformChannelManager.sendEvent(
                "drift/platform_views",
                mapOf(
                    "method" to "onSwitchChanged",
                    "viewId" to viewId,
                    "value" to isChecked
                )
            )
        }
    }

    fun updateConfig(params: Map<String, Any?>) {
        (params["onTintColor"] as? Number)?.let { color ->
            switch.trackTintList = ColorStateList.valueOf(color.toInt())
        }
        (params["thumbTintColor"] as? Number)?.let { color ->
            switch.thumbTintList = ColorStateList.valueOf(color.toInt())
        }
    }
}
