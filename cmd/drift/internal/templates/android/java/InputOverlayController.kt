/**
 * InputOverlayController positions native overlay views based on FrameSnapshot data.
 *
 * On each frame, applySnapshot() receives the authoritative geometry for all
 * platform views and updates their position using fast View properties
 * (translationX/Y, visibility) that sync to the RenderThread without
 * a full measure/layout traversal.
 *
 * layoutParams are only updated when the base size changes, avoiding
 * requestLayout churn during scroll.
 */
package {{.PackageName}}

import android.view.View
import android.view.ViewGroup
import android.widget.FrameLayout
import kotlin.math.roundToInt

class InputOverlayController(
    private val overlayLayout: InputOverlayLayout,
    private val density: Float
) {

    // Cached base size per viewId to avoid unnecessary layoutParams changes
    private val cachedBaseSize = mutableMapOf<Long, Pair<Int, Int>>()

    // Cached clip rect per viewId to avoid allocating a new Rect every frame
    private val cachedClipRect = mutableMapOf<Long, android.graphics.Rect>()

    /**
     * Applies a FrameSnapshot to position all overlay views.
     * Called on the UI thread from UnifiedFrameOrchestrator.doFrame().
     *
     * Zero allocations after warmup: reuses maps, no object creation.
     */
    fun applySnapshot(snapshot: FrameSnapshot) {
        for (view in snapshot.views) {
            applyViewSnapshot(view)
        }
    }

    private fun applyViewSnapshot(vs: ViewSnapshot) {
        // Find the interceptor in the overlay layout by tag
        val interceptor = findViewByTag(vs.viewId) ?: return

        val hasClip = vs.clipLeft != null && vs.clipTop != null &&
                      vs.clipRight != null && vs.clipBottom != null

        if (hasClip) {
            applyClippedSnapshot(interceptor, vs)
        } else {
            applyUnclippedSnapshot(interceptor, vs)
        }
    }

    /**
     * Unclipped path: size the interceptor to the full view dimensions and
     * position it with translationX/Y. The child inside fills MATCH_PARENT.
     */
    private fun applyUnclippedSnapshot(interceptor: View, vs: ViewSnapshot) {
        val baseW = (vs.width * density).roundToInt().coerceAtLeast(0)
        val baseH = (vs.height * density).roundToInt().coerceAtLeast(0)

        val cached = cachedBaseSize[vs.viewId]
        if (cached == null || cached.first != baseW || cached.second != baseH) {
            interceptor.layoutParams = FrameLayout.LayoutParams(baseW, baseH)
            cachedBaseSize[vs.viewId] = Pair(baseW, baseH)
        }

        interceptor.translationX = vs.x * density
        interceptor.translationY = vs.y * density

        // Clear clip bounds (may have been clipped previously)
        if (interceptor.clipBounds != null) {
            interceptor.clipBounds = null
            cachedClipRect.remove(vs.viewId)
        }

        // Reset child offset (may have been clipped previously)
        if (interceptor is ViewGroup && interceptor.childCount > 0) {
            val child = interceptor.getChildAt(0)
            if (child.translationX != 0f || child.translationY != 0f) {
                child.translationX = 0f
                child.translationY = 0f
            }
        }

        val targetVisibility = if (vs.visible) View.VISIBLE else View.INVISIBLE
        if (interceptor.visibility != targetVisibility) {
            interceptor.visibility = targetVisibility
        }
    }

    /**
     * Clipped path: position the interceptor at the full view origin and
     * apply clipBounds to restrict rendering to the visible region. Uses
     * only render properties (translationX/Y, clipBounds) so no
     * requestLayout() is triggered during scroll. This keeps the View
     * hierarchy buffer submission fast enough to land in the same SF vsync
     * as the Skia buffer.
     */
    private fun applyClippedSnapshot(interceptor: View, vs: ViewSnapshot) {
        val fullLeft = vs.x * density
        val fullTop = vs.y * density
        val fullRight = fullLeft + (vs.width * density)
        val fullBottom = fullTop + (vs.height * density)

        val viewportLeft = maxOf(fullLeft, vs.clipLeft!! * density)
        val viewportTop = maxOf(fullTop, vs.clipTop!! * density)
        val viewportRight = minOf(fullRight, vs.clipRight!! * density)
        val viewportBottom = minOf(fullBottom, vs.clipBottom!! * density)

        if (viewportRight <= viewportLeft || viewportBottom <= viewportTop) {
            if (interceptor.visibility != View.INVISIBLE) {
                interceptor.visibility = View.INVISIBLE
            }
            return
        }

        // Set the full base size only when it changes (triggers layout once)
        val baseW = (vs.width * density).roundToInt().coerceAtLeast(0)
        val baseH = (vs.height * density).roundToInt().coerceAtLeast(0)
        val cached = cachedBaseSize[vs.viewId]
        if (cached == null || cached.first != baseW || cached.second != baseH) {
            interceptor.layoutParams = FrameLayout.LayoutParams(baseW, baseH)
            cachedBaseSize[vs.viewId] = Pair(baseW, baseH)
            // Size the child to fill the interceptor (one-time)
            if (interceptor is ViewGroup && interceptor.childCount > 0) {
                val child = interceptor.getChildAt(0)
                child.layoutParams = FrameLayout.LayoutParams(baseW, baseH)
                child.translationX = 0f
                child.translationY = 0f
            }
        }

        // Position at the full view origin (render property, no layout pass)
        interceptor.translationX = fullLeft
        interceptor.translationY = fullTop

        // Clip to the visible region, reusing the cached Rect to avoid allocation
        val clipLeft = (viewportLeft - fullLeft).toInt()
        val clipTop = (viewportTop - fullTop).toInt()
        val clipRight = (viewportRight - fullLeft).toInt()
        val clipBottom = (viewportBottom - fullTop).toInt()

        val rect = cachedClipRect.getOrPut(vs.viewId) { android.graphics.Rect() }
        if (rect.left != clipLeft || rect.top != clipTop || rect.right != clipRight || rect.bottom != clipBottom) {
            rect.set(clipLeft, clipTop, clipRight, clipBottom)
            interceptor.clipBounds = rect
        }

        val targetVisibility = if (vs.visible) View.VISIBLE else View.INVISIBLE
        if (interceptor.visibility != targetVisibility) {
            interceptor.visibility = targetVisibility
        }
    }

    /** Removes cached state for a disposed view. */
    fun removeView(viewId: Long) {
        cachedBaseSize.remove(viewId)
        cachedClipRect.remove(viewId)
    }

    private fun findViewByTag(viewId: Long): View? {
        return overlayLayout.findOverlayView(viewId.toInt())
    }

}
