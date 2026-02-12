/**
 * InputOverlayLayout is a transparent FrameLayout that hosts pooled
 * NativeEditText views (and other platform overlay views) on top of
 * the Skia rendering surface.
 *
 * This layout receives no independent scroll. All overlay positioning
 * is driven by the engine's FrameSnapshot via InputOverlayController.
 */
package {{.PackageName}}

import android.content.Context
import android.view.View
import android.widget.FrameLayout

class InputOverlayLayout(context: Context) : FrameLayout(context) {

    private val viewMap = mutableMapOf<Int, View>()

    init {
        // Transparent, non-clickable, non-focusable container
        isClickable = false
        isFocusable = false
    }

    fun addOverlayView(viewId: Int, view: View) {
        view.tag = viewId
        // Set pivots once at creation so InputOverlayController doesn't need to repeat per frame
        view.pivotX = 0f
        view.pivotY = 0f
        viewMap[viewId] = view
        addView(view, LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT))
    }

    fun removeOverlayView(viewId: Int) {
        val view = viewMap.remove(viewId) ?: return
        removeView(view)
    }

    fun findOverlayView(viewId: Int): View? = viewMap[viewId]
}
