/**
 * DriftSurfaceView is the main rendering surface for the Drift engine on Android.
 *
 * This class extends SurfaceView and uses SurfaceControl transactions to present
 * rendered content atomically with the View hierarchy. This ensures platform views
 * (EditText, WebView) stay in sync with GPU-rendered Drift content during scrolling.
 */
package {{.PackageName}}

import android.content.Context
import android.os.Handler
import android.os.Looper
import android.util.Log
import android.view.Choreographer
import android.view.MotionEvent
import android.view.SurfaceHolder
import android.view.SurfaceView
import java.util.TreeMap
import java.util.concurrent.atomic.AtomicBoolean

class DriftSurfaceView(context: Context) : SurfaceView(context), SurfaceHolder.Callback {
    private var renderer: DriftRenderer? = null

    private val activePointers = mutableMapOf<Long, Pair<Double, Double>>()

    @Volatile
    private var active = false

    private val frameScheduled = AtomicBoolean(false)
    private val mainHandler = Handler(Looper.getMainLooper())

    @Volatile
    private var surfaceControlHandle: Long = 0L

    @Volatile
    private var renderGeneration: Int = 0

    private data class PendingPresent(
        val generation: Int,
        val frameSeq: Long,
        val poolPtr: Long,
        val surfaceControlHandle: Long,
        val bufferIndex: Int,
        val fenceFd: Int,
    )

    private val pendingPresents = TreeMap<Long, PendingPresent>()
    private var lastAppliedGeometrySeq: Long = 0L
    private var latestPresentedSeq: Long = 0L

    private val postFrameRunnable = Runnable {
        if (active) {
            Choreographer.getInstance().postFrameCallback(frameCallback)
        } else {
            frameScheduled.set(false)
        }
    }

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
        PlatformViewHandler.setOnGeometryAppliedListener { frameSeq ->
            onGeometryApplied(frameSeq)
        }
    }

    override fun surfaceCreated(holder: SurfaceHolder) {
        lastAppliedGeometrySeq = PlatformViewHandler.lastAppliedGeometrySeq()
        latestPresentedSeq = lastAppliedGeometrySeq

        val scHandle = NativeBridge.createSurfaceControl(holder.surface)
        surfaceControlHandle = scHandle
        if (scHandle == 0L) {
            Log.e("DriftSurfaceView", "Failed to create SurfaceControl; rendering disabled")
            post { setBackgroundColor(android.graphics.Color.RED) }
            return
        }

        clearPendingPresents()
        renderGeneration += 1
        val generation = renderGeneration

        val r = DriftRenderer(
            surfaceView = this,
            surfaceControlHandle = scHandle,
            renderGeneration = generation,
        )
        renderer = r
        r.start(width, height)
    }

    override fun surfaceChanged(holder: SurfaceHolder, format: Int, width: Int, height: Int) {
        renderer?.onSurfaceChanged(width, height)
    }

    override fun surfaceDestroyed(holder: SurfaceHolder) {
        renderGeneration += 1
        clearPendingPresents()

        renderer?.stop()
        renderer = null

        if (surfaceControlHandle != 0L) {
            NativeBridge.destroySurfaceControl(surfaceControlHandle)
            surfaceControlHandle = 0L
        }
    }

    fun scheduleFrame() {
        if (active && frameScheduled.compareAndSet(false, true)) {
            mainHandler.post(postFrameRunnable)
        }
    }

    fun renderNow() {
        NativeBridge.requestFrame()
        renderer?.requestRender()
        scheduleFrame()
    }

    override fun onSizeChanged(w: Int, h: Int, oldw: Int, oldh: Int) {
        super.onSizeChanged(w, h, oldw, oldh)
        if (w != oldw || h != oldh) {
            renderer?.onSurfaceChanged(w, h)
            renderNow()
        }
    }

    override fun onAttachedToWindow() {
        super.onAttachedToWindow()
        active = true
        PlatformViewHandler.setOnGeometryAppliedListener { frameSeq ->
            onGeometryApplied(frameSeq)
        }
        scheduleFrame()
        updateDeviceScale()
    }

    override fun onDetachedFromWindow() {
        active = false
        mainHandler.removeCallbacks(postFrameRunnable)
        Choreographer.getInstance().removeFrameCallback(frameCallback)
        frameScheduled.set(false)
        clearPendingPresents()
        PlatformViewHandler.setOnGeometryAppliedListener(null)
        super.onDetachedFromWindow()
    }

    fun pauseScheduling() {
        active = false
        mainHandler.removeCallbacks(postFrameRunnable)
        Choreographer.getInstance().removeFrameCallback(frameCallback)
        frameScheduled.set(false)
    }

    fun pauseRendering() {
        renderer?.onPause()
    }

    fun resumeScheduling() {
        active = true
        scheduleFrame()
    }

    fun resumeRendering() {
        renderer?.onResume()
    }

    fun enqueueRenderedFrame(
        generation: Int,
        frameSeq: Long,
        requiresGeometrySync: Boolean,
        poolPtr: Long,
        surfaceControlHandle: Long,
        bufferIndex: Int,
        fenceFd: Int,
    ) {
        if (Looper.myLooper() != Looper.getMainLooper()) {
            mainHandler.post {
                enqueueRenderedFrame(
                    generation,
                    frameSeq,
                    requiresGeometrySync,
                    poolPtr,
                    surfaceControlHandle,
                    bufferIndex,
                    fenceFd,
                )
            }
            return
        }

        if (generation != renderGeneration || surfaceControlHandle == 0L || poolPtr == 0L) {
            NativeBridge.closeFenceFd(fenceFd)
            return
        }

        val present = PendingPresent(
            generation = generation,
            frameSeq = frameSeq,
            poolPtr = poolPtr,
            surfaceControlHandle = surfaceControlHandle,
            bufferIndex = bufferIndex,
            fenceFd = fenceFd,
        )

        if (!requiresGeometrySync) {
            if (frameSeq <= latestPresentedSeq) {
                NativeBridge.closeFenceFd(fenceFd)
                return
            }
            // This frame is being presented immediately, so any queued geometry-synced
            // frames up to this sequence can never be presented without rollback.
            dropPendingPresentsUpTo(frameSeq)
            presentFrameInternal(present)
            latestPresentedSeq = frameSeq
            return
        }

        if (frameSeq <= latestPresentedSeq) {
            NativeBridge.closeFenceFd(fenceFd)
            return
        }

        // Keep only newest pending work to avoid lag buildup.
        val staleEntries = pendingPresents.headMap(frameSeq, false).entries.toList()
        for (entry in staleEntries) {
            NativeBridge.closeFenceFd(entry.value.fenceFd)
            pendingPresents.remove(entry.key)
        }

        val replaced = pendingPresents.put(frameSeq, present)
        if (replaced != null) {
            NativeBridge.closeFenceFd(replaced.fenceFd)
        }

        tryPresentReadyFrame()
    }

    private fun onGeometryApplied(frameSeq: Long) {
        if (Looper.myLooper() != Looper.getMainLooper()) {
            mainHandler.post { onGeometryApplied(frameSeq) }
            return
        }
        if (frameSeq > lastAppliedGeometrySeq) {
            lastAppliedGeometrySeq = frameSeq
        }
        tryPresentReadyFrame()
    }

    private fun tryPresentReadyFrame() {
        val readyEntry = pendingPresents.floorEntry(lastAppliedGeometrySeq) ?: return
        val chosenSeq = readyEntry.key
        val chosen = readyEntry.value

        val readyEntries = pendingPresents.headMap(chosenSeq, true).entries.toList()
        for (entry in readyEntries) {
            if (entry.key != chosenSeq) {
                NativeBridge.closeFenceFd(entry.value.fenceFd)
            }
            pendingPresents.remove(entry.key)
        }

        // A newer frame has already been shown; never roll back to an older sequence.
        if (chosenSeq <= latestPresentedSeq) {
            NativeBridge.closeFenceFd(chosen.fenceFd)
            return
        }

        if (chosen.generation != renderGeneration) {
            NativeBridge.closeFenceFd(chosen.fenceFd)
            return
        }

        presentFrameInternal(chosen)
        latestPresentedSeq = chosen.frameSeq
    }

    private fun presentFrameInternal(frame: PendingPresent) {
        NativeBridge.presentBuffer(
            pool = frame.poolPtr,
            surfaceControl = frame.surfaceControlHandle,
            bufferIndex = frame.bufferIndex,
            fenceFd = frame.fenceFd,
        )
    }

    private fun dropPendingPresentsUpTo(frameSeq: Long) {
        val staleEntries = pendingPresents.headMap(frameSeq, true).entries.toList()
        for (entry in staleEntries) {
            NativeBridge.closeFenceFd(entry.value.fenceFd)
            pendingPresents.remove(entry.key)
        }
    }

    private fun clearPendingPresents() {
        val entries = pendingPresents.values.toList()
        pendingPresents.clear()
        for (entry in entries) {
            NativeBridge.closeFenceFd(entry.fenceFd)
        }
    }

    override fun dispatchTouchEvent(event: MotionEvent): Boolean {
        if (event.actionMasked == MotionEvent.ACTION_DOWN) {
            // Flush a frame before the accessibility hit-test so the
            // semantics tree reflects the current layout.
            renderNow()
            if (AccessibilityHandler.handleExploreByTouch(event.x, event.y)) {
                return true
            }
        }
        return super.dispatchTouchEvent(event)
    }

    override fun dispatchGenericMotionEvent(event: MotionEvent): Boolean {
        renderNow()
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

        renderNow()
        return true
    }

    override fun dispatchHoverEvent(event: MotionEvent): Boolean {
        if (AccessibilityHandler.onHoverEvent(event.x, event.y, event.actionMasked)) {
            return true
        }
        return super.dispatchHoverEvent(event)
    }

    override fun onHoverEvent(event: MotionEvent): Boolean {
        if (AccessibilityHandler.onHoverEvent(event.x, event.y, event.actionMasked)) {
            return true
        }
        return super.onHoverEvent(event)
    }

    private fun updateDeviceScale() {
        val density = resources.displayMetrics.density.toDouble()
        NativeBridge.setDeviceScale(density)
    }
}
