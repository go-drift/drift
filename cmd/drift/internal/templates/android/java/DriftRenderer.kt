/**
 * DriftRenderer implements the Skia GPU rendering pipeline for Android.
 *
 * Uses SurfaceControl + AHardwareBuffer for synchronized presentation with
 * platform views. Rendering targets offscreen FBOs backed by AHardwareBuffers,
 * and completed frames are presented via SurfaceControl transactions on the
 * main thread.
 */
package {{.PackageName}}

import android.opengl.EGL14
import android.opengl.EGLConfig
import android.opengl.EGLContext
import android.opengl.EGLDisplay
import android.opengl.EGLExt
import android.opengl.EGLSurface
import android.opengl.GLES20
import android.os.Build
import android.util.Log
import java.util.concurrent.locks.ReentrantLock
import kotlin.concurrent.withLock

class DriftRenderer(
    private val surfaceView: DriftSurfaceView,
    private val surfaceControlHandle: Long,
    private val renderGeneration: Int,
) {
    @Volatile var width = 0
        private set
    @Volatile var height = 0
        private set

    private var eglDisplay: EGLDisplay = EGL14.EGL_NO_DISPLAY
    private var eglConfig: EGLConfig? = null
    private var eglContext: EGLContext = EGL14.EGL_NO_CONTEXT
    private var eglSurface: EGLSurface = EGL14.EGL_NO_SURFACE
    private var surface: DriftSurface? = null

    private val lock = ReentrantLock()
    private val renderRequested = lock.newCondition()

    @Volatile private var running = false
    @Volatile private var paused = false
    @Volatile private var needsRender = false
    @Volatile private var sizeChanged = false

    private var thread: Thread? = null

    /** Whether the Skia backend initialized successfully. */
    private var initialized = false

    /**
     * Starts the render thread at the given initial dimensions.
     */
    fun start(w: Int, h: Int) {
        width = w
        height = h
        running = true
        needsRender = true

        thread = Thread({
            if (!initEGL()) {
                return@Thread
            }
            if (!initSkia()) {
                releaseEGL()
                return@Thread
            }
            initialized = true

            surface = DriftSurface(width, height)

            renderLoop()

            surface?.destroy()
            surface = null
            releaseEGL()
        }, "DriftRenderThread").apply {
            start()
        }
    }

    /**
     * Stops the render thread and waits for it to finish.
     */
    fun stop() {
        running = false
        lock.withLock { renderRequested.signalAll() }
        try {
            thread?.join(2000)
        } catch (_: InterruptedException) {}
        thread = null
    }

    /**
     * Signals the render thread to render a frame.
     * Safe to call from any thread.
     */
    fun requestRender() {
        needsRender = true
        lock.withLock { renderRequested.signalAll() }
    }

    /**
     * Notifies the renderer of a surface size change.
     * Safe to call from any thread.
     */
    fun onSurfaceChanged(w: Int, h: Int) {
        if (w == width && h == height) return
        width = w
        height = h
        sizeChanged = true
        needsRender = true
        lock.withLock { renderRequested.signalAll() }
    }

    /**
     * Pauses the render thread (lifecycle).
     */
    fun onPause() {
        paused = true
    }

    /**
     * Resumes the render thread (lifecycle).
     */
    fun onResume() {
        paused = false
        requestRender()
    }

    private fun initEGL(): Boolean {
        eglDisplay = EGL14.eglGetDisplay(EGL14.EGL_DEFAULT_DISPLAY)
        val version = IntArray(2)
        if (!EGL14.eglInitialize(eglDisplay, version, 0, version, 1)) {
            Log.e("DriftRenderer", "eglInitialize failed")
            return false
        }

        // Prefer ES 3 on devices, ES 2 on emulators
        val isEmulator = Build.HARDWARE.contains("goldfish") || Build.HARDWARE.contains("ranchu")
        val glesVersion = if (isEmulator) 2 else 3
        if (isEmulator) {
            Log.w("DriftRenderer", "Emulator detected; using GLES 2 for stability")
        }

        val configAttribs = intArrayOf(
            EGL14.EGL_RED_SIZE, 8,
            EGL14.EGL_GREEN_SIZE, 8,
            EGL14.EGL_BLUE_SIZE, 8,
            EGL14.EGL_ALPHA_SIZE, 8,
            EGL14.EGL_DEPTH_SIZE, 0,
            EGL14.EGL_STENCIL_SIZE, 0,
            EGL14.EGL_RENDERABLE_TYPE, if (glesVersion == 3) EGLExt.EGL_OPENGL_ES3_BIT_KHR else EGL14.EGL_OPENGL_ES2_BIT,
            EGL14.EGL_NONE,
        )
        val configs = arrayOfNulls<EGLConfig>(1)
        val numConfigs = IntArray(1)
        EGL14.eglChooseConfig(eglDisplay, configAttribs, 0, configs, 0, 1, numConfigs, 0)

        if (numConfigs[0] == 0 || configs[0] == null) {
            Log.e("DriftRenderer", "No EGL config found")
            return false
        }
        eglConfig = configs[0]

        val contextAttribs = intArrayOf(
            EGL14.EGL_CONTEXT_CLIENT_VERSION, glesVersion,
            EGL14.EGL_NONE,
        )
        eglContext = EGL14.eglCreateContext(eglDisplay, eglConfig, EGL14.EGL_NO_CONTEXT, contextAttribs, 0)
        if (eglContext == EGL14.EGL_NO_CONTEXT) {
            Log.e("DriftRenderer", "eglCreateContext failed")
            return false
        }

        // SurfaceControl rendering targets AHardwareBuffer-backed FBOs, so the
        // EGL surface is only needed to make the context current. A 1x1 pbuffer
        // serves this purpose without tying us to a window surface.
        val pbufferAttribs = intArrayOf(
            EGL14.EGL_WIDTH, 1,
            EGL14.EGL_HEIGHT, 1,
            EGL14.EGL_NONE,
        )
        eglSurface = EGL14.eglCreatePbufferSurface(eglDisplay, eglConfig, pbufferAttribs, 0)
        if (eglSurface == EGL14.EGL_NO_SURFACE) {
            Log.e("DriftRenderer", "eglCreatePbufferSurface failed")
            return false
        }

        if (!EGL14.eglMakeCurrent(eglDisplay, eglSurface, eglSurface, eglContext)) {
            Log.e("DriftRenderer", "eglMakeCurrent failed")
            return false
        }
        return true
    }

    private fun initSkia(): Boolean {
        if (NativeBridge.appInit() != 0) {
            Log.e("DriftRenderer", "Failed to initialize Drift app")
            return false
        }
        if (NativeBridge.initSkiaGL() != 0) {
            Log.e("DriftRenderer", "Failed to initialize Skia GL backend")
            return false
        }
        if (NativeBridge.platformInit() != 0) {
            Log.e("DriftRenderer", "Failed to initialize platform channels")
            return false
        }
        return true
    }

    private fun renderLoop() {
        while (running) {
            lock.withLock {
                while (running && !needsRender) {
                    renderRequested.await()
                }
            }
            if (!running) break
            if (paused) {
                needsRender = false
                continue
            }

            needsRender = false

            if (sizeChanged) {
                sizeChanged = false
                surface?.resize(width, height)
                NativeBridge.requestFrame()
            }

            val w = width
            val h = height
            if (w <= 0 || h <= 0) continue

            val surf = surface ?: continue
            val bufferIndex = surf.acquireBuffer()
            if (bufferIndex < 0) continue

            val result = NativeBridge.renderFrameSkia(w, h)
            if (result != 0) {
                GLES20.glClearColor(0.8f, 0.1f, 0.1f, 1f)
                GLES20.glClear(GLES20.GL_COLOR_BUFFER_BIT)
            }

            val fenceFd = surf.createFence()
            val frameSeq = NativeBridge.currentFrameSeq()
            val geometryPending = NativeBridge.geometryPending() != 0
            val poolPtr = surf.poolPtr

            surfaceView.enqueueRenderedFrame(
                generation = renderGeneration,
                frameSeq = frameSeq,
                requiresGeometrySync = geometryPending,
                poolPtr = poolPtr,
                surfaceControlHandle = surfaceControlHandle,
                bufferIndex = bufferIndex,
                fenceFd = fenceFd,
            )

            if (NativeBridge.needsFrame() != 0) {
                surfaceView.scheduleFrame()
            }
        }
    }

    private fun releaseEGL() {
        if (eglDisplay != EGL14.EGL_NO_DISPLAY) {
            EGL14.eglMakeCurrent(eglDisplay, EGL14.EGL_NO_SURFACE, EGL14.EGL_NO_SURFACE, EGL14.EGL_NO_CONTEXT)
            if (eglSurface != EGL14.EGL_NO_SURFACE) {
                EGL14.eglDestroySurface(eglDisplay, eglSurface)
            }
            if (eglContext != EGL14.EGL_NO_CONTEXT) {
                EGL14.eglDestroyContext(eglDisplay, eglContext)
            }
            EGL14.eglTerminate(eglDisplay)
        }
        eglDisplay = EGL14.EGL_NO_DISPLAY
        eglContext = EGL14.EGL_NO_CONTEXT
        eglSurface = EGL14.EGL_NO_SURFACE
    }
}
