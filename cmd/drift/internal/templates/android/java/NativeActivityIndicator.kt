/**
 * NativeActivityIndicator.kt
 * Provides native ProgressBar (indeterminate) embedded in Drift UI.
 */
package {{.PackageName}}

import android.content.Context
import android.content.res.ColorStateList
import android.view.View
import android.widget.FrameLayout
import android.widget.ProgressBar

/**
 * Platform view container for native activity indicator.
 */
class NativeActivityIndicatorContainer(
    context: Context,
    override val viewId: Int,
    params: Map<String, Any?>
) : PlatformViewContainer {

    override val view: View
    private val progressBar: ProgressBar

    init {
        // Determine size based on parameter
        val sizeParam = (params["size"] as? Number)?.toInt() ?: 1 // Default to medium
        val styleAttr = when (sizeParam) {
            0 -> android.R.attr.progressBarStyleSmall
            2 -> android.R.attr.progressBarStyleLarge
            else -> android.R.attr.progressBarStyle // Medium
        }

        progressBar = ProgressBar(context, null, styleAttr).apply {
            isIndeterminate = true

            // Apply color if provided
            (params["color"] as? Number)?.let { colorValue ->
                if (colorValue.toInt() != 0) {
                    indeterminateTintList = ColorStateList.valueOf(colorValue.toInt())
                }
            }

            // Set visibility based on animating state (default: true)
            val animating = params["animating"] as? Boolean ?: true
            visibility = if (animating) View.VISIBLE else View.INVISIBLE

            layoutParams = FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.WRAP_CONTENT,
                FrameLayout.LayoutParams.WRAP_CONTENT
            )
        }

        view = progressBar
    }

    override fun dispose() {
        // Nothing special to clean up
    }

    fun setAnimating(animating: Boolean) {
        // Android ProgressBar doesn't have start/stop - we control via visibility
        progressBar.visibility = if (animating) View.VISIBLE else View.INVISIBLE
    }

    fun updateConfig(params: Map<String, Any?>) {
        // Update color
        (params["color"] as? Number)?.let { colorValue ->
            if (colorValue.toInt() != 0) {
                progressBar.indeterminateTintList = ColorStateList.valueOf(colorValue.toInt())
            }
        }

        // Update animating state
        (params["animating"] as? Boolean)?.let { animating ->
            progressBar.visibility = if (animating) View.VISIBLE else View.INVISIBLE
        }

        // Note: Changing size requires recreating the progress bar, which we don't support
        // for simplicity. Size should be set at creation time.
    }
}
