/**
 * DriftRenderer implements the Skia GPU rendering pipeline for Android.
 *
 * This renderer:
 *   1. Initializes the Skia GL backend once the context is ready
 *   2. Calls into the Go engine to draw directly into the current framebuffer
 *   3. Relies on GLSurfaceView to swap buffers after each frame
 *   4. After each render, checks NeedsFrame() and schedules a follow-up if needed
 */
package {{.PackageName}}

import android.opengl.GLES20
import android.opengl.GLSurfaceView
import android.util.Log
import javax.microedition.khronos.egl.EGLConfig
import javax.microedition.khronos.opengles.GL10

/**
 * OpenGL ES renderer that delegates drawing to the Go + Skia backend.
 *
 * @param surfaceView The DriftSurfaceView to call scheduleFrame() on after rendering,
 *                    enabling animation continuity without a continuous polling loop.
 */
class DriftRenderer(private val surfaceView: DriftSurfaceView) : GLSurfaceView.Renderer {
    /** Current viewport width in pixels. Volatile for cross-thread visibility. */
    @Volatile var width = 0
        private set

    /** Current viewport height in pixels. Volatile for cross-thread visibility. */
    @Volatile var height = 0
        private set

    /**
     * Updates the cached dimensions from the UI thread.
     *
     * Called by DriftSurfaceView.onSizeChanged() to push new dimensions
     * immediately, avoiding a stale-size render before the GL thread's
     * onSurfaceChanged() has run.
     */
    fun updateSize(w: Int, h: Int) {
        width = w
        height = h
    }

    /** Whether the Skia backend initialized successfully. */
    private var skiaReady = false

    override fun onSurfaceCreated(gl: GL10?, config: EGLConfig?) {
        if (NativeBridge.appInit() != 0) {
            Log.e("DriftRenderer", "Failed to initialize Drift app")
        }
        skiaReady = NativeBridge.initSkiaGL() == 0
        if (!skiaReady) {
            Log.e("DriftRenderer", "Failed to initialize Skia GL backend")
        } else if (NativeBridge.platformInit() != 0) {
            Log.e("DriftRenderer", "Failed to initialize platform channels")
        }
        GLES20.glClearColor(0f, 0f, 0f, 1f)
    }

    override fun onSurfaceChanged(gl: GL10?, width: Int, height: Int) {
        this.width = width
        this.height = height
        GLES20.glViewport(0, 0, width, height)
        // Mark a frame as needed so the engine re-renders at the new size.
        // GLSurfaceView calls onDrawFrame immediately after this on the GL thread,
        // so the next frame will use the updated dimensions.
        NativeBridge.requestFrame()
    }

    override fun onDrawFrame(gl: GL10?) {
        val w = width
        val h = height
        if (!skiaReady || w <= 0 || h <= 0) {
            GLES20.glClearColor(0.8f, 0.1f, 0.1f, 1f)
            GLES20.glClear(GLES20.GL_COLOR_BUFFER_BIT)
            return
        }

        // Ensure the viewport matches the latest dimensions. updateSize() from
        // the UI thread may have set new values before onSurfaceChanged() ran.
        GLES20.glViewport(0, 0, w, h)

        // Always render - GLSurfaceView swaps buffers after onDrawFrame returns,
        // so skipping render causes flickering on physical devices with triple-buffering.
        // The Go engine has layer caching, so rendering unchanged content is efficient.
        val result = NativeBridge.renderFrameSkia(w, h)
        if (result != 0) {
            GLES20.glClearColor(0.8f, 0.1f, 0.1f, 1f)
            GLES20.glClear(GLES20.GL_COLOR_BUFFER_BIT)
        }

        // Post-render check: if the engine still has work (animations, ballistics,
        // pending dispatch callbacks), schedule another frame. This handles the case
        // where RequestFrame() was called mid-render and couldn't notify the platform
        // due to lock contention.
        if (NativeBridge.needsFrame() != 0) {
            surfaceView.scheduleFrame()
        }
    }
}
