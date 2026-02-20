/**
 * TouchInterceptorView.kt
 * Wraps each platform view to intercept touches when the view is obscured by
 * Drift widgets (modal barriers, dropdowns, bottom sheets, etc.).
 *
 * On ACTION_DOWN, synchronously queries the Go engine's hit test. If the
 * platform view is not the topmost target, all events are intercepted and
 * forwarded to the surface view for engine processing. If the view IS the
 * topmost target, touches pass through to the native view normally, preserving
 * all native gesture behavior (cursor placement, long press selection, etc.).
 *
 * When the view is topmost but unfocused, tap-vs-scroll detection (touch slop)
 * ensures scrolls starting on the view are forwarded to the engine.
 */
package {{.PackageName}}

import android.content.Context
import android.graphics.Canvas
import android.graphics.Path
import android.os.SystemClock
import android.view.MotionEvent
import android.view.View
import android.view.ViewConfiguration
import android.view.ViewGroup
import android.widget.EditText
import android.widget.FrameLayout
import kotlin.math.abs

class TouchInterceptorView(
    context: Context,
    private val viewId: Int
) : FrameLayout(context) {

    var surfaceView: View? = null
    var enableUnfocusedTextScrollForwarding: Boolean = true
    private val touchSlop = ViewConfiguration.get(context).scaledTouchSlop

    /** Set once at creation. True for TextureView-backed views that support
     *  non-rectangular region clipping via canvas clip operations. */
    var supportsRegionMask = false

    // Region clip state: a visible rect with rectangular holes subtracted.
    // Uses native canvas clip operations (clipRect + clipOutRect) which are
    // hardware-accelerated and reliable, unlike Path.op + clipPath.
    internal var hasRegionClip = false
    internal var visRectLeft = 0f
    internal var visRectTop = 0f
    internal var visRectRight = 0f
    internal var visRectBottom = 0f
    internal val holePaths = ArrayList<Path>(4)

    fun clearRegionClip() {
        if (hasRegionClip) {
            hasRegionClip = false
            holePaths.clear()
            invalidate()
        }
    }

    // Touch interception state
    private var blockMode = false        // true when view is obscured
    private var slopTracking = false     // true when tracking unfocused tap-vs-scroll
    private var isForwardingScroll = false
    private var touchStartX = 0f
    private var touchStartY = 0f
    private var pendingDownTime = 0L

    override fun dispatchDraw(canvas: Canvas) {
        if (hasRegionClip) {
            canvas.save()
            canvas.clipRect(visRectLeft, visRectTop, visRectRight, visRectBottom)
            for (hole in holePaths) {
                canvas.clipOutPath(hole)
            }
            super.dispatchDraw(canvas)
            canvas.restore()
        } else {
            super.dispatchDraw(canvas)
        }
    }

    override fun onInterceptTouchEvent(ev: MotionEvent): Boolean {
        when (ev.actionMasked) {
            MotionEvent.ACTION_DOWN -> {
                blockMode = false
                slopTracking = false
                isForwardingScroll = false

                // Query the Go engine: is this platform view the topmost target?
                // Convert screen-absolute coordinates to surface view space,
                // matching the coordinate system used by the normal touch path.
                val surface = surfaceView ?: return super.onInterceptTouchEvent(ev)
                val surfaceLoc = intArrayOf(0, 0)
                surface.getLocationOnScreen(surfaceLoc)
                val pixelX = (ev.rawX - surfaceLoc[0]).toDouble()
                val pixelY = (ev.rawY - surfaceLoc[1]).toDouble()
                val result = NativeBridge.hitTestPlatformView(viewId.toLong(), pixelX, pixelY)

                if (result == 0) {
                    // Obscured: intercept all events immediately
                    blockMode = true
                    return true
                }

                // Topmost: check if an unfocused EditText is the target
                val editText = findEditTextAtPosition(ev.x, ev.y)
                if (enableUnfocusedTextScrollForwarding && editText != null && !editText.hasFocus()) {
                    // Track for tap-vs-scroll detection
                    slopTracking = true
                    touchStartX = ev.x
                    touchStartY = ev.y
                    pendingDownTime = ev.downTime
                    return false // Let child see DOWN
                }

                // Focused or non-EditText: pass through normally
                return false
            }

            MotionEvent.ACTION_MOVE -> {
                if (slopTracking && !isForwardingScroll) {
                    val dx = abs(ev.x - touchStartX)
                    val dy = abs(ev.y - touchStartY)
                    if (dx > touchSlop || dy > touchSlop) {
                        // Movement exceeded slop: this is a scroll
                        isForwardingScroll = true

                        // Cancel the child's touch
                        val cancel = MotionEvent.obtain(
                            pendingDownTime,
                            SystemClock.uptimeMillis(),
                            MotionEvent.ACTION_CANCEL,
                            ev.x, ev.y, 0
                        )
                        super.dispatchTouchEvent(cancel)
                        cancel.recycle()

                        // Send DOWN to surface at original position
                        forwardToSurface(MotionEvent.ACTION_DOWN, touchStartX, touchStartY, pendingDownTime, pendingDownTime)

                        return true // Intercept remaining events
                    }
                }
            }

            MotionEvent.ACTION_UP, MotionEvent.ACTION_CANCEL -> {
                // Touch ended without exceeding slop: it's a tap, child handles it
                slopTracking = false
            }
        }
        return super.onInterceptTouchEvent(ev)
    }

    override fun onTouchEvent(event: MotionEvent): Boolean {
        if (blockMode) {
            // Obscured: forward all events to surface view
            forwardToSurface(event)
            if (event.actionMasked == MotionEvent.ACTION_UP ||
                event.actionMasked == MotionEvent.ACTION_CANCEL) {
                blockMode = false
            }
            return true
        }

        if (isForwardingScroll) {
            // Scroll forwarding: send to surface view
            forwardToSurface(event)
            if (event.actionMasked == MotionEvent.ACTION_UP ||
                event.actionMasked == MotionEvent.ACTION_CANCEL) {
                slopTracking = false
                isForwardingScroll = false
            }
            return true
        }

        return super.onTouchEvent(event)
    }

    private fun forwardToSurface(event: MotionEvent) {
        val surface = surfaceView ?: return
        // Convert from interceptor coordinates to surface view coordinates
        val location = intArrayOf(0, 0)
        getLocationOnScreen(location)
        val surfaceLoc = intArrayOf(0, 0)
        surface.getLocationOnScreen(surfaceLoc)

        val adjustedX = event.x + (location[0] - surfaceLoc[0])
        val adjustedY = event.y + (location[1] - surfaceLoc[1])

        val adjusted = MotionEvent.obtain(
            event.downTime,
            event.eventTime,
            event.actionMasked,
            adjustedX, adjustedY,
            event.metaState
        )
        surface.dispatchTouchEvent(adjusted)
        adjusted.recycle()
    }

    private fun forwardToSurface(action: Int, localX: Float, localY: Float, downTime: Long, eventTime: Long) {
        val surface = surfaceView ?: return
        val location = intArrayOf(0, 0)
        getLocationOnScreen(location)
        val surfaceLoc = intArrayOf(0, 0)
        surface.getLocationOnScreen(surfaceLoc)

        val adjustedX = localX + (location[0] - surfaceLoc[0])
        val adjustedY = localY + (location[1] - surfaceLoc[1])

        val event = MotionEvent.obtain(downTime, eventTime, action, adjustedX, adjustedY, 0)
        surface.dispatchTouchEvent(event)
        event.recycle()
    }

    private fun findEditTextAtPosition(x: Float, y: Float): EditText? {
        return findEditTextIn(this, x, y)
    }

    private fun findEditTextIn(parent: ViewGroup, x: Float, y: Float): EditText? {
        for (i in parent.childCount - 1 downTo 0) {
            val child = parent.getChildAt(i)
            if (child.visibility != View.VISIBLE) continue
            val childX = x - child.left
            val childY = y - child.top
            if (childX >= 0 && childX < child.width && childY >= 0 && childY < child.height) {
                if (child is EditText) return child
                if (child is ViewGroup) {
                    val found = findEditTextIn(child, childX, childY)
                    if (found != null) return found
                }
            }
        }
        return null
    }
}
