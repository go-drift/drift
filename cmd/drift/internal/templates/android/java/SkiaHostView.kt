/**
 * SkiaHostView renders Skia content via an AHardwareBuffer-backed FBO.
 *
 * Instead of rendering to a separate SurfaceView/TextureView surface, Skia
 * draws into a GPU-memory HardwareBuffer. That buffer is wrapped as a hardware
 * Bitmap and drawn in onDraw() through the standard HWUI pipeline. Because
 * overlay views (EditText, etc.) are drawn in the same HWUI display list,
 * Skia content and overlays land in a single RenderThread buffer and a single
 * SurfaceFlinger layer, eliminating cross-surface sync lag.
 *
 * Requires API 31+ (minSdk) for Bitmap.wrapHardwareBuffer().
 *
 * EGL initialization happens on a background thread. After init, all rendering
 * runs synchronously on the UI thread (called from UnifiedFrameOrchestrator.doFrame
 * during the ANIMATION phase).
 */
package {{.PackageName}}

import android.content.Context
import android.graphics.Bitmap
import android.graphics.Canvas
import android.graphics.ColorSpace
import android.os.Handler
import android.os.HandlerThread
import android.util.Log
import android.view.MotionEvent
import android.view.View

class SkiaHostView(context: Context) : View(context), DriftSkiaHost {

    private val TAG = "SkiaHostView"

    private var initThread: HandlerThread? = HandlerThread("drift-init").also { it.start() }
    private var initHandler: Handler? = initThread?.let { Handler(it.looper) }

    // HardwareBuffer bitmap for onDraw
    private var hwBitmap: Bitmap? = null

    @Volatile override var surfaceWidth = 0
        private set
    @Volatile override var surfaceHeight = 0
        private set

    @Volatile override var engineReady = false
        private set

    /** Callback to request a new frame. Set by MainActivity after construction. */
    var onFrameNeeded: (() -> Unit)? = null

    /** Set to true on resume/resize; checked by renderFrame to purge stale GPU caches. */
    @Volatile private var needsResourcePurge = false

    private val activePointers = mutableMapOf<Long, Pair<Double, Double>>()

    init {
        setWillNotDraw(false)
        updateDeviceScale()
    }

    override fun onSizeChanged(w: Int, h: Int, oldw: Int, oldh: Int) {
        super.onSizeChanged(w, h, oldw, oldh)
        if (w <= 0 || h <= 0) return
        if (w == oldw && h == oldh) return

        surfaceWidth = w
        surfaceHeight = h

        if (!engineReady) {
            // First size: kick off init on background thread
            initHandler?.post {
                initEGL()
                createHwbResources(w, h)
                if (!initEngine()) {
                    initThread?.quitSafely()
                    initThread = null
                    initHandler = null
                    return@post
                }
                NativeBridge.releaseContext()
                engineReady = true
                initThread?.quitSafely()
                initThread = null
                initHandler = null
                onFrameNeeded?.invoke()
            }
        } else {
            // Resize: recreate HWB FBO at new size
            NativeBridge.makeCurrent()
            NativeBridge.destroyHwbFBO()
            createHwbResources(w, h)
            NativeBridge.releaseContext()
            needsResourcePurge = true
            NativeBridge.requestFrame()
            onFrameNeeded?.invoke()
        }
    }

    /**
     * Renders a frame synchronously on the UI thread. Called from doFrame
     * (ANIMATION phase) after stepping the engine and applying overlay positions.
     *
     * Renders into the HWB FBO, then invalidates the View so onDraw() picks up
     * the updated hardware Bitmap in the same TRAVERSAL pass as overlay views.
     */
    override fun renderFrame() {
        val w = surfaceWidth
        val h = surfaceHeight
        if (w <= 0 || h <= 0 || !engineReady) return

        NativeBridge.makeCurrent()

        // Purge stale GPU caches (glyph atlas, textures) after sleep/wake or resize.
        // Must happen while EGL context is current.
        if (needsResourcePurge) {
            needsResourcePurge = false
            NativeBridge.purgeResources()
        }

        NativeBridge.bindHwbFBO()

        val result = NativeBridge.renderFrameSync(w, h)
        if (result != 0) {
            Log.e(TAG, "renderFrameSync failed: $result")
        }

        NativeBridge.unbindHwbFBO()
        android.opengl.GLES20.glFlush()
        NativeBridge.releaseContext()

        // Mark this View dirty so onDraw runs during TRAVERSAL
        invalidate()
    }

    override fun onDraw(canvas: Canvas) {
        hwBitmap?.let { bitmap ->
            canvas.drawBitmap(bitmap, 0f, 0f, null)
        }
    }

    /**
     * Called from MainActivity.onResume(). Marks GPU caches for purging
     * on the next render to handle stale textures after sleep/wake.
     */
    fun notifyResume() {
        if (engineReady) {
            needsResourcePurge = true
        }
    }

    // EGL setup (runs on init thread)

    private fun initEGL() {
        if (NativeBridge.initEGL() != 0) {
            Log.e(TAG, "Failed to initialize EGL")
            return
        }
        NativeBridge.makeCurrent()
        Log.i(TAG, "EGL initialized (HardwareBuffer, UI-thread render)")
    }

    private fun createHwbResources(w: Int, h: Int) {
        val fboId = NativeBridge.createHwbFBO(w, h)
        if (fboId < 0) {
            Log.e(TAG, "createHwbFBO failed")
            return
        }

        val hwb = NativeBridge.getHardwareBuffer()
        if (hwb == null) {
            Log.e(TAG, "getHardwareBuffer returned null")
            return
        }

        hwBitmap?.recycle()
        hwBitmap = Bitmap.wrapHardwareBuffer(hwb, ColorSpace.get(ColorSpace.Named.SRGB))
        hwb.close()
        Log.i(TAG, "HWB bitmap created: ${w}x${h}")
    }

    private fun initEngine(): Boolean {
        if (NativeBridge.appInit() != 0) {
            Log.e(TAG, "Failed to initialize Drift app")
            return false
        }
        if (NativeBridge.initSkiaGL() != 0) {
            Log.e(TAG, "Failed to initialize Skia GL backend")
            return false
        }
        if (NativeBridge.platformInit() != 0) {
            Log.e(TAG, "Failed to initialize platform channels")
            return false
        }
        Log.i(TAG, "Drift engine initialized")
        return true
    }

    // Touch handling

    override fun dispatchTouchEvent(event: MotionEvent): Boolean {
        if (event.actionMasked == MotionEvent.ACTION_DOWN) {
            NativeBridge.requestFrame()
            if (AccessibilityHandler.handleExploreByTouch(event.x, event.y)) {
                return true
            }
        }
        return super.dispatchTouchEvent(event)
    }

    override fun dispatchGenericMotionEvent(event: MotionEvent): Boolean {
        NativeBridge.requestFrame()
        onFrameNeeded?.invoke()
        return super.dispatchGenericMotionEvent(event)
    }

    override fun onTouchEvent(event: MotionEvent): Boolean {
        when (event.actionMasked) {
            MotionEvent.ACTION_DOWN, MotionEvent.ACTION_POINTER_DOWN -> {
                val index = event.actionIndex
                val pointerID = event.getPointerId(index).toLong()
                val x = event.getX(index).toDouble()
                val y = event.getY(index).toDouble()
                activePointers[pointerID] = Pair(x, y)
                NativeBridge.pointerEvent(pointerID, 0, x, y)
            }

            MotionEvent.ACTION_MOVE -> {
                for (index in 0 until event.pointerCount) {
                    val pointerID = event.getPointerId(index).toLong()
                    val x = event.getX(index).toDouble()
                    val y = event.getY(index).toDouble()
                    activePointers[pointerID] = Pair(x, y)
                    NativeBridge.pointerEvent(pointerID, 1, x, y)
                }
            }

            MotionEvent.ACTION_UP, MotionEvent.ACTION_POINTER_UP -> {
                val index = event.actionIndex
                val pointerID = event.getPointerId(index).toLong()
                val x = event.getX(index).toDouble()
                val y = event.getY(index).toDouble()
                activePointers.remove(pointerID)
                NativeBridge.pointerEvent(pointerID, 2, x, y)
            }

            MotionEvent.ACTION_CANCEL -> {
                for ((pointerID, position) in activePointers) {
                    NativeBridge.pointerEvent(pointerID, 3, position.first, position.second)
                }
                activePointers.clear()
            }

            else -> return false
        }

        NativeBridge.requestFrame()
        onFrameNeeded?.invoke()
        return true
    }

    // Accessibility

    override fun dispatchHoverEvent(event: MotionEvent): Boolean {
        if (AccessibilityHandler.onHoverEvent(event.x, event.y, event.actionMasked)) {
            return true
        }
        return super.dispatchHoverEvent(event)
    }

    private fun updateDeviceScale() {
        val density = resources.displayMetrics.density.toDouble()
        NativeBridge.setDeviceScale(density)
    }
}
