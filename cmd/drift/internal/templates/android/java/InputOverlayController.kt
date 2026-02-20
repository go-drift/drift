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
 *
 * Clip strategy per view:
 *   - No clip: clear all clips, full view visible.
 *   - Rect clip (clipBounds): conservative single-rect clip from Go engine.
 *   - Region clip (Path): precise occlusion masking for TextureView-backed views.
 *     Only used when the interceptor's supportsRegionMask is true and the snapshot
 *     contains occlusion rects. Uses cached Path objects on TouchInterceptorView
 *     to avoid per-frame allocation.
 *
 * Verification scenarios (manual or instrumentation):
 *   - Transition: region clip -> rect clip -> no clip (and reverse).
 *   - Empty visible rect hides the view.
 *   - Empty clip viewport hides the view.
 *   - Touch forwarding works correctly when region clipping is active.
 *   - Multiple views with mixed capabilities in the same frame.
 */
package {{.PackageName}}

import android.graphics.Matrix
import android.graphics.Path
import android.view.View
import android.view.ViewGroup
import android.widget.FrameLayout
import kotlin.math.roundToInt

class InputOverlayController(
    private val overlayLayout: InputOverlayLayout,
    private val density: Float
) {

    // Cached base size per viewId to avoid unnecessary layoutParams changes.
    private val cachedBaseSize = mutableMapOf<Long, Pair<Int, Int>>()

    // Cached clip rect per viewId to avoid allocating a new Rect every frame.
    private val cachedClipRect = mutableMapOf<Long, android.graphics.Rect>()

    /**
     * Applies a FrameSnapshot to position all overlay views.
     * Called on the UI thread from UnifiedFrameOrchestrator.doFrame().
     *
     * Zero allocations after warmup: reuses cached maps, Rect objects, and
     * Path buffers on TouchInterceptorView.
     */
    fun applySnapshot(snapshot: FrameSnapshot) {
        for (view in snapshot.views) {
            applyViewSnapshot(view)
        }
    }

    private fun applyViewSnapshot(vs: ViewSnapshot) {
        val interceptor = overlayLayout.findOverlayView(vs.viewId.toInt()) ?: return
        val tiv = interceptor as? TouchInterceptorView
        val regionEligible = tiv != null && tiv.supportsRegionMask && vs.occlusionPaths.isNotEmpty()

        applyBaseGeometry(interceptor, vs)
        val clipHidden = applyClipStrategy(interceptor, vs)

        val targetVisibility = when {
            clipHidden -> View.INVISIBLE
            regionEligible -> View.VISIBLE
            vs.visible -> View.VISIBLE
            else -> View.INVISIBLE
        }
        if (interceptor.visibility != targetVisibility) {
            interceptor.visibility = targetVisibility
        }
    }

    /**
     * Shared base geometry: size the interceptor, position it, and ensure the
     * child fills it. Only triggers requestLayout when the base size changes.
     */
    private fun applyBaseGeometry(interceptor: View, vs: ViewSnapshot) {
        val baseW = (vs.width * density).roundToInt().coerceAtLeast(0)
        val baseH = (vs.height * density).roundToInt().coerceAtLeast(0)

        val cached = cachedBaseSize[vs.viewId]
        if (cached == null || cached.first != baseW || cached.second != baseH) {
            interceptor.layoutParams = FrameLayout.LayoutParams(baseW, baseH)
            cachedBaseSize[vs.viewId] = Pair(baseW, baseH)
            if (interceptor is ViewGroup && interceptor.childCount > 0) {
                val child = interceptor.getChildAt(0)
                child.layoutParams = FrameLayout.LayoutParams(baseW, baseH)
                child.translationX = 0f
                child.translationY = 0f
            }
        }

        interceptor.translationX = vs.x * density
        interceptor.translationY = vs.y * density
    }

    /**
     * Determines and applies the appropriate clip for this view.
     * Returns true if the clip collapses to empty (view should be hidden).
     */
    private fun applyClipStrategy(interceptor: View, vs: ViewSnapshot): Boolean {
        val tiv = interceptor as? TouchInterceptorView

        // Region masking: only for capable views with occlusion paths.
        if (tiv != null && tiv.supportsRegionMask && vs.occlusionPaths.isNotEmpty()) {
            return applyRegionClip(tiv, vs)
        }

        // Leaving region clip: clear it.
        tiv?.clearRegionClip()

        // Rect clip from clipBounds.
        val hasClip = vs.clipLeft != null && vs.clipTop != null &&
                      vs.clipRight != null && vs.clipBottom != null

        if (hasClip) {
            return applyRectClip(interceptor, vs)
        }

        // No clip: clear any previous clipBounds.
        if (interceptor.clipBounds != null) {
            interceptor.clipBounds = null
            cachedClipRect.remove(vs.viewId)
        }
        return false
    }

    /**
     * Rect clip: restrict rendering to the viewport defined by clipBounds.
     * Returns true if the viewport is empty (view should be hidden).
     */
    private fun applyRectClip(interceptor: View, vs: ViewSnapshot): Boolean {
        val fullLeft = vs.x * density
        val fullTop = vs.y * density
        val fullRight = fullLeft + (vs.width * density)
        val fullBottom = fullTop + (vs.height * density)

        val viewportLeft = maxOf(fullLeft, vs.clipLeft!! * density)
        val viewportTop = maxOf(fullTop, vs.clipTop!! * density)
        val viewportRight = minOf(fullRight, vs.clipRight!! * density)
        val viewportBottom = minOf(fullBottom, vs.clipBottom!! * density)

        if (viewportRight <= viewportLeft || viewportBottom <= viewportTop) {
            return true
        }

        val clipLeft = (viewportLeft - fullLeft).toInt()
        val clipTop = (viewportTop - fullTop).toInt()
        val clipRight = (viewportRight - fullLeft).toInt()
        val clipBottom = (viewportBottom - fullTop).toInt()

        val rect = cachedClipRect.getOrPut(vs.viewId) { android.graphics.Rect() }
        if (rect.left != clipLeft || rect.top != clipTop || rect.right != clipRight || rect.bottom != clipBottom) {
            rect.set(clipLeft, clipTop, clipRight, clipBottom)
            interceptor.clipBounds = rect
        }
        return false
    }

    /**
     * Region clip: set the visible rect and occlusion holes on the interceptor.
     * The interceptor clips children in dispatchDraw using native canvas clip
     * operations (clipRect + clipOutPath) which are hardware-accelerated and
     * reliable. Returns true if the visible rect is empty.
     */
    private fun applyRegionClip(tiv: TouchInterceptorView, vs: ViewSnapshot): Boolean {
        val originX = vs.x * density
        val originY = vs.y * density
        val baseW = (vs.width * density).roundToInt()
        val baseH = (vs.height * density).roundToInt()

        val visLeft = (vs.visibleLeft * density - originX).coerceAtLeast(0f)
        val visTop = (vs.visibleTop * density - originY).coerceAtLeast(0f)
        val visRight = (vs.visibleRight * density - originX).coerceAtMost(baseW.toFloat())
        val visBottom = (vs.visibleBottom * density - originY).coerceAtMost(baseH.toFloat())

        if (visRight <= visLeft || visBottom <= visTop) {
            tiv.clearRegionClip()
            return true
        }

        // Set visible rect on the interceptor.
        tiv.visRectLeft = visLeft
        tiv.visRectTop = visTop
        tiv.visRectRight = visRight
        tiv.visRectBottom = visBottom

        // Transform each occlusion path from logical points to local pixel coords.
        // Matrix: scale by density, translate by -origin.
        val matrix = Matrix()
        matrix.setScale(density, density)
        matrix.postTranslate(-originX, -originY)

        val holes = tiv.holePaths
        var holeIdx = 0
        for (srcPath in vs.occlusionPaths) {
            val localPath: Path
            if (holeIdx < holes.size) {
                localPath = holes[holeIdx]
                localPath.reset()
            } else {
                localPath = Path()
                holes.add(localPath)
            }
            srcPath.transform(matrix, localPath)
            holeIdx++
        }
        // Trim excess cached paths from previous frames.
        while (holes.size > holeIdx) {
            holes.removeAt(holes.size - 1)
        }

        // Clear rect clip (may have been rect-clipped previously).
        if (tiv.clipBounds != null) {
            tiv.clipBounds = null
            cachedClipRect.remove(vs.viewId)
        }
        tiv.hasRegionClip = true
        tiv.invalidate()
        return false
    }

    /** Removes cached state for a disposed view. */
    fun removeView(viewId: Long) {
        cachedBaseSize.remove(viewId)
        cachedClipRect.remove(viewId)
    }
}
