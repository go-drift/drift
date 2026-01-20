/**
 * DriftSurfaceView is the main rendering surface for the Drift engine on Android.
 *
 * This class extends GLSurfaceView to provide:
 *   1. An OpenGL ES 2.0 rendering context for displaying frames
 *   2. VSync-synchronized frame callbacks using Android's Choreographer
 *   3. Touch event handling that forwards input to the Go engine
 *
 * Rendering Pipeline:
 *
 *     Choreographer (vsync signal)
 *           │
 *           ▼
 *     DriftSurfaceView.doFrame()
 *           │
 *           ▼ requestRender()
 *     DriftRenderer.onDrawFrame()
 *           │
 *           ▼ NativeBridge.renderFrameSkia()
 *     Go Engine (Skia GPU render)
 *           │
 *           ▼ OpenGL (displays on screen)
 *
 * Frame Timing:
 *   Uses RENDERMODE_WHEN_DIRTY to avoid unnecessary CPU/GPU usage.
 *   The Choreographer callback requests a render at the display's refresh rate
 *   (typically 60Hz), ensuring smooth, tear-free animation.
 *
 * Lifecycle:
 *   - Start: onAttachedToWindow() registers the Choreographer callback
 *   - Stop: onDetachedFromWindow() unregisters the callback
 *   - The parent Activity must call onResume()/onPause() appropriately
 */
package {{.PackageName}}

import android.content.Context
import android.opengl.GLSurfaceView
import android.os.Build
import android.util.Log
import android.view.Choreographer
import android.view.MotionEvent

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
     * Choreographer callback for vsync-synchronized frame rendering.
     *
     * The Choreographer provides callbacks aligned with the display's vsync signal,
     * ensuring frames are rendered at the optimal time for smooth animation.
     *
     * This callback:
     *   1. Requests a new render (which triggers DriftRenderer.onDrawFrame())
     *   2. Re-registers itself for the next frame
     *
     * The self-re-registration pattern creates a continuous render loop that runs
     * as long as the callback is registered (between onAttachedToWindow and onDetachedFromWindow).
     */
    private val frameCallback = object : Choreographer.FrameCallback {
        override fun doFrame(frameTimeNanos: Long) {
            // Request the GL thread to call onDrawFrame() for the next render
            requestRender()

            // Schedule this callback again for the next vsync
            // This creates the continuous animation loop
            Choreographer.getInstance().postFrameCallback(this)
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

        // Create and set the renderer that will handle drawing
        renderer = DriftRenderer()
        setRenderer(renderer)

        // Only render when explicitly requested via requestRender()
        // The Choreographer callback handles the render timing
        renderMode = RENDERMODE_WHEN_DIRTY

        // Send the device scale to the Go engine for consistent sizing.
        updateDeviceScale()
    }

    /**
     * Called when the view is attached to a window.
     *
     * Starts the render loop by registering the Choreographer callback.
     * From this point, doFrame() will be called at the display's refresh rate.
     */
    override fun onAttachedToWindow() {
        super.onAttachedToWindow()
        // Start receiving vsync callbacks to drive the render loop
        Choreographer.getInstance().postFrameCallback(frameCallback)

        // Refresh scale in case configuration changed while detached.
        updateDeviceScale()
    }

    /**
     * Called when the view is detached from its window.
     *
     * Stops the render loop by removing the Choreographer callback.
     * This prevents unnecessary work when the view is not visible.
     */
    override fun onDetachedFromWindow() {
        // Stop receiving vsync callbacks
        Choreographer.getInstance().removeFrameCallback(frameCallback)
        super.onDetachedFromWindow()
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
     * Multi-touch Note:
     *   For ACTION_MOVE, we use pointer index 0 (the first touch).
     *   For other actions, we use the actionIndex to get the correct pointer.
     *   This simple approach works for single-touch demos; multi-touch would
     *   require tracking individual pointer IDs.
     */
    override fun onTouchEvent(event: MotionEvent): Boolean {
        // Convert Android action to Drift pointer phase
        val phase = when (event.actionMasked) {
            // Touch began (first finger or additional fingers)
            MotionEvent.ACTION_DOWN, MotionEvent.ACTION_POINTER_DOWN -> 0

            // Touch position changed
            MotionEvent.ACTION_MOVE -> 1

            // Touch ended (finger lifted)
            MotionEvent.ACTION_UP, MotionEvent.ACTION_POINTER_UP -> 2

            // Touch cancelled by system (e.g., gesture detector took over)
            MotionEvent.ACTION_CANCEL -> 3

            // Unknown action - don't handle
            else -> return false
        }

        // Determine which pointer index to use for coordinates
        // For MOVE events, use the first pointer (index 0)
        // For other events, use the action index (which finger triggered the event)
        val index = if (event.actionMasked == MotionEvent.ACTION_MOVE) 0 else event.actionIndex

        // Get coordinates in view pixels (not density-independent pixels)
        // These match the render buffer dimensions
        val x = event.getX(index).toDouble()
        val y = event.getY(index).toDouble()

        // Forward the event to the Go engine
        NativeBridge.pointerEvent(phase, x, y)

        // Return true to indicate we handled this event
        return true
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
