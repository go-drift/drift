/**
 * DriftContainer.kt
 * Root FrameLayout that hosts the rendering surface and overlay layers.
 *
 * View hierarchy:
 *   DriftContainer (FrameLayout)
 *     - skiaView (SkiaHostView, HardwareBuffer + HWUI onDraw rendering)
 *     - overlayLayout (transparent, on top, for native platform views)
 */
package {{.PackageName}}

import android.content.Context
import android.widget.FrameLayout

/**
 * Interface for the Skia rendering host, providing surface dimensions
 * and frame rendering capability.
 */
interface DriftSkiaHost {
    val surfaceWidth: Int
    val surfaceHeight: Int
    val engineReady: Boolean
    fun renderFrame()
}

/**
 * Container that hosts the Skia host view and an overlay layout for
 * native platform views (text inputs, web views, etc.).
 */
class DriftContainer(context: Context) : FrameLayout(context) {
    val skiaView: SkiaHostView = SkiaHostView(context)
    val overlayLayout: InputOverlayLayout = InputOverlayLayout(context)

    init {
        addView(skiaView, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))
        addView(overlayLayout, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))
    }
}
