/**
 * DriftSurfaceView is the main rendering surface for the Drift engine on Android.
 *
 * This class extends SurfaceView and uses SurfaceControl transactions to present
 * rendered content atomically with the View hierarchy. This ensures platform views
 * (EditText, WebView) stay in sync with GPU-rendered Drift content during scrolling.
 *
 * Rendering Pipeline:
 *
 *     Go engine calls RequestFrame()
 *           |
 *           v  JNI callback
 *     PlatformChannelManager.nativeScheduleFrame()
 *           |
 *           v
 *     DriftSurfaceView.scheduleFrame()
 *           |
 *           v  one-shot Choreographer callback
 *     DriftSurfaceView.doFrame()
 *           |
 *           v  signal render thread
 *     DriftRenderer.renderFrame()
 *           |
 *           v  NativeBridge.renderFrameSkia()
 *     Go Engine (Skia GPU render to AHardwareBuffer)
 *           |
 *           v  post to main thread
 *     DriftSurfaceView.presentFrame()
 *           |
 *           v  SurfaceControl.Transaction + applyTransactionOnDraw
 *     Atomic present with View hierarchy
 *
 * Frame Scheduling:
 *   Uses on-demand, one-shot Choreographer callbacks instead of a continuous
 *   polling loop. The Choreographer goes completely idle when no work is needed.
 *   Two paths schedule frames:
 *     1. Go-initiated: RequestFrame()/Dispatch() triggers a JNI callback
 *     2. Post-render: DriftRenderer checks NeedsFrame() after each render
 *   Input events trigger an immediate render for sub-vsync touch latency.
 *
 * Lifecycle:
 *   - Active: onAttachedToWindow()/resumeScheduling() enable scheduling
 *   - Inactive: onDetachedFromWindow()/pauseScheduling() disable scheduling
 */
package {{.PackageName}}

import android.content.Context
import android.os.Handler
import android.os.Looper
import android.view.Choreographer
import android.view.MotionEvent
import android.view.SurfaceHolder
import android.view.SurfaceView
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Custom SurfaceView that integrates the Drift engine with Android's display system
 * using SurfaceControl transactions for synchronized rendering.
 *
 * @param context The Android context, typically the parent Activity.
 */
class DriftSurfaceView(context: Context) : SurfaceView(context), SurfaceHolder.Callback {
    /**
     * The render thread that handles EGL context and frame rendering.
     * Initialized when the surface is created.
     */
    private var renderer: DriftRenderer? = null

    /**
     * Tracks active pointer IDs and their last known positions.
     * Used to properly cancel all pointers when ACTION_CANCEL is received,
     * since the event may have pointerCount=0 at that point.
     */
    private val activePointers = mutableMapOf<Long, Pair<Double, Double>>()

    /**
     * Whether the view is in an active lifecycle state (attached and resumed).
     * When false, no Choreographer callbacks are posted.
     * Volatile because scheduleFrame() reads this from render and JNI threads.
     */
    @Volatile
    private var active = false

    /**
     * Coalesces multiple scheduleFrame() calls into a single Choreographer callback.
     * Set to true when a callback is pending, cleared in doFrame().
     */
    private val frameScheduled = AtomicBoolean(false)

    /** Main-thread handler for posting Choreographer callbacks. */
    private val mainHandler = Handler(Looper.getMainLooper())

    /** Named Runnable for targeted removal via mainHandler.removeCallbacks(). */
    private val postFrameRunnable = Runnable {
        if (active) {
            Choreographer.getInstance().postFrameCallback(frameCallback)
        } else {
            frameScheduled.set(false)
        }
    }

    /**
     * One-shot Choreographer callback for vsync-synchronized rendering.
     *
     * When work is needed, this callback signals the render thread and
     * re-registers itself immediately via scheduleFrame() so the next vsync
     * fires without waiting for the render thread's post-render check.
     *
     * The post-render NeedsFrame() check in DriftRenderer remains as a safety
     * net for edge cases where a frame request arrives mid-render.
     */
    private val frameCallback = Choreographer.FrameCallback {
        frameScheduled.set(false)
        if (active && NativeBridge.needsFrame() != 0) {
            renderer?.requestRender()
            scheduleFrame()
        }
    }

    init {
        holder.addCallback(this)
        updateDeviceScale()
    }

    override fun surfaceCreated(holder: SurfaceHolder) {
        // Create child SurfaceControl via NDK (manages native ASurfaceControl)
        NativeBridge.createSurfaceControl(holder.surface)

        // Start the render thread
        val r = DriftRenderer(this)
        renderer = r
        r.start(width, height)
    }

    override fun surfaceChanged(holder: SurfaceHolder, format: Int, width: Int, height: Int) {
        renderer?.onSurfaceChanged(width, height)
    }

    override fun surfaceDestroyed(holder: SurfaceHolder) {
        renderer?.stop()
        renderer = null
        NativeBridge.destroySurfaceControl()
    }

    /**
     * Presents a rendered frame by submitting a SurfaceControl transaction via NDK.
     * The transaction sets the buffer from the pool and applies it atomically.
     *
     * Must be called on the main thread.
     *
     * @param poolPtr     Native buffer pool pointer.
     * @param bufferIndex Buffer index from acquireBuffer().
     * @param fenceFd     Native fence FD, or -1. Consumed by this call.
     */
    fun presentFrame(poolPtr: Long, bufferIndex: Int, fenceFd: Int) {
        NativeBridge.presentBuffer(poolPtr, bufferIndex, fenceFd)
    }

    /**
     * Schedules a one-shot Choreographer callback if one is not already pending.
     * Safe to call from any thread. The callback runs on the main thread.
     */
    fun scheduleFrame() {
        if (active && frameScheduled.compareAndSet(false, true)) {
            mainHandler.post(postFrameRunnable)
        }
    }

    /**
     * Marks the engine dirty, signals an immediate render for low-latency
     * response, and schedules a Choreographer callback for follow-up work.
     */
    fun renderNow() {
        NativeBridge.requestFrame()
        renderer?.requestRender()
        scheduleFrame()
    }

    /**
     * Called when the view's dimensions change (e.g. device rotation).
     *
     * Schedules a frame so the engine re-renders at the new size.
     */
    override fun onSizeChanged(w: Int, h: Int, oldw: Int, oldh: Int) {
        super.onSizeChanged(w, h, oldw, oldh)
        if (w != oldw || h != oldh) {
            renderer?.onSurfaceChanged(w, h)
            renderNow()
        }
    }

    /**
     * Called when the view is attached to a window.
     *
     * Enables frame scheduling and posts an initial frame.
     */
    override fun onAttachedToWindow() {
        super.onAttachedToWindow()
        active = true
        scheduleFrame()
        updateDeviceScale()
    }

    /**
     * Called when the view is detached from its window.
     *
     * Disables frame scheduling and removes any pending callback.
     */
    override fun onDetachedFromWindow() {
        active = false
        mainHandler.removeCallbacks(postFrameRunnable)
        Choreographer.getInstance().removeFrameCallback(frameCallback)
        frameScheduled.set(false)
        super.onDetachedFromWindow()
    }

    /**
     * Disables frame scheduling and clears pending callbacks.
     * Called from MainActivity.onPause().
     */
    fun pauseScheduling() {
        active = false
        mainHandler.removeCallbacks(postFrameRunnable)
        Choreographer.getInstance().removeFrameCallback(frameCallback)
        frameScheduled.set(false)
    }

    /**
     * Pauses the render thread.
     * Called from MainActivity.onPause().
     */
    fun pauseRendering() {
        renderer?.onPause()
    }

    /**
     * Enables frame scheduling and posts an initial Choreographer callback.
     * Called from MainActivity.onResume().
     */
    fun resumeScheduling() {
        active = true
        scheduleFrame()
    }

    /**
     * Resumes the render thread.
     * Called from MainActivity.onResume().
     */
    fun resumeRendering() {
        renderer?.onResume()
    }

    /**
     * Intercepts touch events to handle accessibility explore-by-touch.
     * When touch exploration is enabled, single taps should focus elements.
     */
    override fun dispatchTouchEvent(event: MotionEvent): Boolean {
        if (event.actionMasked == MotionEvent.ACTION_DOWN) {
            // Flush a frame before the accessibility hit-test so the
            // semantics tree reflects the current layout. This may cause
            // a benign double-render with onTouchEvent's renderNow() on
            // the same ACTION_DOWN, but the second call is a no-op when
            // NeedsFrame() returns false.
            renderNow()
            if (AccessibilityHandler.handleExploreByTouch(event.x, event.y)) {
                return true
            }
        }
        return super.dispatchTouchEvent(event)
    }

    /**
     * Handle generic motion events including hover events.
     */
    override fun dispatchGenericMotionEvent(event: MotionEvent): Boolean {
        renderNow()
        return super.dispatchGenericMotionEvent(event)
    }

    /**
     * Handles touch events and forwards them to the Go engine.
     *
     * Converts Android MotionEvent actions to Drift pointer phases:
     *   - ACTION_DOWN / ACTION_POINTER_DOWN -> Phase 0 (Down)
     *   - ACTION_MOVE -> Phase 1 (Move)
     *   - ACTION_UP / ACTION_POINTER_UP -> Phase 2 (Up)
     *   - ACTION_CANCEL -> Phase 3 (Cancel)
     *
     * @param event The MotionEvent from the Android system.
     * @return true if the event was handled, false otherwise.
     *
     * Multi-touch:
     *   Each pointer is tracked by its unique ID (from getPointerId()).
     *   For MOVE events, all active pointers are reported.
     *   For DOWN/UP events, only the affected pointer is reported.
     *   For CANCEL, all tracked pointers are cancelled using their last known positions.
     */
    override fun onTouchEvent(event: MotionEvent): Boolean {
        when (event.actionMasked) {
            // Touch began (first finger or additional fingers)
            MotionEvent.ACTION_DOWN, MotionEvent.ACTION_POINTER_DOWN -> {
                val index = event.actionIndex
                val pointerID = event.getPointerId(index).toLong()
                val x = event.getX(index).toDouble()
                val y = event.getY(index).toDouble()
                activePointers[pointerID] = Pair(x, y)
                NativeBridge.pointerEvent(pointerID, 0, x, y)
            }

            // Touch position changed - report all active pointers
            MotionEvent.ACTION_MOVE -> {
                for (index in 0 until event.pointerCount) {
                    val pointerID = event.getPointerId(index).toLong()
                    val x = event.getX(index).toDouble()
                    val y = event.getY(index).toDouble()
                    activePointers[pointerID] = Pair(x, y)
                    NativeBridge.pointerEvent(pointerID, 1, x, y)
                }
            }

            // Touch ended (finger lifted)
            MotionEvent.ACTION_UP, MotionEvent.ACTION_POINTER_UP -> {
                val index = event.actionIndex
                val pointerID = event.getPointerId(index).toLong()
                val x = event.getX(index).toDouble()
                val y = event.getY(index).toDouble()
                activePointers.remove(pointerID)
                NativeBridge.pointerEvent(pointerID, 2, x, y)
            }

            // Touch cancelled by system - cancel all tracked pointers
            // Note: event.pointerCount may be zero, so we use our tracked map
            MotionEvent.ACTION_CANCEL -> {
                for ((pointerID, position) in activePointers) {
                    NativeBridge.pointerEvent(pointerID, 3, position.first, position.second)
                }
                activePointers.clear()
            }

            // Unknown action - don't handle
            else -> return false
        }

        // Render after dispatching the pointer event so the render thread sees
        // the latest scroll position. Previously renderNow() ran first,
        // which could render a frame with the old offset.
        renderNow()
        return true
    }

    /**
     * Handles hover events for accessibility explore-by-touch.
     *
     * When TalkBack is enabled, touch events are converted to hover events
     * for exploration. This allows users to drag their finger to hear
     * descriptions of UI elements without activating them.
     *
     * @param event The hover MotionEvent from the Android system.
     * @return true if the event was handled, false otherwise.
     */
    override fun dispatchHoverEvent(event: MotionEvent): Boolean {
        // Let the accessibility handler try to handle hover for explore-by-touch
        if (AccessibilityHandler.onHoverEvent(event.x, event.y, event.actionMasked)) {
            return true
        }
        return super.dispatchHoverEvent(event)
    }

    /**
     * Alternative hover event handler (called by dispatchHoverEvent).
     */
    override fun onHoverEvent(event: MotionEvent): Boolean {
        // Try accessibility handler first
        if (AccessibilityHandler.onHoverEvent(event.x, event.y, event.actionMasked)) {
            return true
        }
        return super.onHoverEvent(event)
    }

    /**
     * Sends the current display density to the Go engine.
     *
     * Android provides density as a scale factor (1.0 on mdpi, 2.0 on xhdpi, etc).
     * The engine uses this to scale logical sizes to pixels for consistent physical size.
     */
    private fun updateDeviceScale() {
        val density = resources.displayMetrics.density.toDouble()
        NativeBridge.setDeviceScale(density)
    }
}
