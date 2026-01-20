/**
 * DriftRenderer implements the Skia GPU rendering pipeline for Android.
 *
 * This renderer:
 *   1. Initializes the Skia GL backend once the context is ready
 *   2. Calls into the Go engine to draw directly into the current framebuffer
 *   3. Relies on GLSurfaceView to swap buffers after each frame
 */
package {{.PackageName}}

import android.opengl.GLES20
import android.opengl.GLSurfaceView
import android.util.Log
import javax.microedition.khronos.egl.EGLConfig
import javax.microedition.khronos.opengles.GL10

/**
 * OpenGL ES renderer that delegates drawing to the Go + Skia backend.
 */
class DriftRenderer : GLSurfaceView.Renderer {
    /** Current viewport width in pixels. */
    private var width = 0

    /** Current viewport height in pixels. */
    private var height = 0

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
    }

    override fun onDrawFrame(gl: GL10?) {
        if (!skiaReady || width <= 0 || height <= 0) {
            GLES20.glClearColor(0.8f, 0.1f, 0.1f, 1f)
            GLES20.glClear(GLES20.GL_COLOR_BUFFER_BIT)
            return
        }

        val result = NativeBridge.renderFrameSkia(width, height)
        if (result != 0) {
            GLES20.glClearColor(0.8f, 0.1f, 0.1f, 1f)
            GLES20.glClear(GLES20.GL_COLOR_BUFFER_BIT)
        }
    }
}
