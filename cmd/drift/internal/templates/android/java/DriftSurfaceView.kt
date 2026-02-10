/**
 * DriftSurfaceView is the main rendering surface for the Drift engine on Android.
 *
 * This class extends GLSurfaceView to provide:
 *   1. An OpenGL ES 2.0 rendering context for displaying frames
 *   2. On-demand, one-shot frame scheduling via Choreographer
 *   3. Touch event handling that forwards input to the Go engine
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
 *           v  requestRender()
 *     DriftRenderer.onDrawFrame()
 *           |
 *           v  NativeBridge.renderFrameSkia()
 *     Go Engine (Skia GPU render)
 *
 * Frame Scheduling:
 *   Uses on-demand, one-shot Choreographer callbacks instead of a continuous
 *   polling loop. The Choreographer goes completely idle when no work is needed.
 *   Two paths schedule frames:
 *     1. Go-initiated: RequestFrame()/Dispatch() triggers a JNI callback
 *     2. Post-render: DriftRenderer checks NeedsFrame() after each render
 *   Input events call requestRender() directly for sub-vsync touch latency.
 *
 * Lifecycle:
 *   - Active: onAttachedToWindow()/resumeScheduling() enable scheduling
 *   - Inactive: onDetachedFromWindow()/pauseScheduling() disable scheduling
 */
package {{.PackageName}}

import android.content.Context
import android.opengl.GLSurfaceView
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.util.Log
import android.view.Choreographer
import android.view.MotionEvent
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Custom GLSurfaceView that integrates the Drift engine with Android's display system.
 *
 * @param context The Android context, typically the parent Activity.
 */
class DriftSurfaceView(context: Context) : GLSurfaceView(context) {
    /**
     * The OpenGL renderer that handles drawing each frame.
     * Initialized in the init block after configuring the OpenGL context.
     */
    private val renderer: DriftRenderer

    /**
     * Tracks active pointer IDs and their last known positions.
     * Used to properly cancel all pointers when ACTION_CANCEL is received,
     * since the event may have pointerCount=0 at that point.
     */
    private val activePointers = mutableMapOf<Long, Pair<Double, Double>>()

    /**
     * Whether the view is in an active lifecycle state (attached and resumed).
     * When false, no Choreographer callbacks are posted.
     * Volatile because scheduleFrame() reads this from GL and JNI threads.
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
     * Unlike a continuous loop, this callback does not re-register itself.
     * A new callback is posted only when scheduleFrame() is called again
     * (from Go-initiated requests or the post-render NeedsFrame() check).
     *
     * A no-op here (needsFrame returns 0) is safe: either the work was
     * already rendered by a direct requestRender(), or Go will call
     * notifyPlatform() on its next scheduling attempt, posting a fresh callback.
     */
    private val frameCallback = Choreographer.FrameCallback {
        frameScheduled.set(false)
        if (active && NativeBridge.needsFrame() != 0) {
            requestRender()
        }
    }

    /**
     * Initializes the OpenGL surface and renderer.
     *
     * Configuration:
     *   - EGL context version 3: Use OpenGL ES 3.0 for Skia Ganesh
     *   - RENDERMODE_WHEN_DIRTY: Only render when requestRender() is called
     *     This saves battery compared to RENDERMODE_CONTINUOUSLY
     */
    init {
        // Prefer OpenGL ES 3.0 when available (required by Skia on devices).
        // Emulators can be unstable with ES 3, so fall back to ES 2 there.
        val isEmulator = Build.HARDWARE.contains("goldfish") || Build.HARDWARE.contains("ranchu")
        val glesVersion = if (isEmulator) 2 else 3
        if (isEmulator) {
            Log.w("DriftSurfaceView", "Emulator detected; using GLES 2 for stability")
        }
        setEGLContextClientVersion(glesVersion)

        // Create and set the renderer, passing this view for post-render scheduling
        renderer = DriftRenderer(this)
        setRenderer(renderer)

        // Only render when explicitly requested via requestRender()
        renderMode = RENDERMODE_WHEN_DIRTY

        // Send the device scale to the Go engine for consistent sizing.
        updateDeviceScale()
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
     * Marks the engine dirty, queues an immediate GL render for low-latency
     * response, and schedules a Choreographer callback for follow-up work.
     */
    fun renderNow() {
        NativeBridge.requestFrame()
        requestRender()
        scheduleFrame()
    }

    /**
     * Called when the view's dimensions change (e.g. device rotation).
     *
     * Schedules a frame so the engine re-renders at the new size.
     * The GL thread's onSurfaceChanged already updates the viewport dimensions;
     * this ensures the Choreographer runs to pick them up.
     */
    override fun onSizeChanged(w: Int, h: Int, oldw: Int, oldh: Int) {
        super.onSizeChanged(w, h, oldw, oldh)
        if (w != oldw || h != oldh) {
            renderer.updateSize(w, h)
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
     * Enables frame scheduling and posts an initial Choreographer callback.
     * Called from MainActivity.onResume().
     */
    fun resumeScheduling() {
        active = true
        scheduleFrame()
    }

    /**
     * Intercepts touch events to handle accessibility explore-by-touch.
     * When touch exploration is enabled, single taps should focus elements.
     */
    override fun dispatchTouchEvent(event: MotionEvent): Boolean {
        if (event.actionMasked == MotionEvent.ACTION_DOWN) {
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
        renderNow()
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

        // Return true to indicate we handled this event
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
