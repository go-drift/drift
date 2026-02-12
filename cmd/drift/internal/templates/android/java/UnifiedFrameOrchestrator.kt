/**
 * UnifiedFrameOrchestrator provides deterministic frame-perfect synchronization
 * between Skia content and platform views.
 *
 * All frame operations run synchronously in a single Choreographer callback
 * during the ANIMATION phase (before Android's TRAVERSAL phase). This ensures:
 *
 * 1. Engine step (dispatch, layout, paint) runs on UI thread
 * 2. Platform view geometry is applied immediately (no async posting)
 * 3. Skia rendering submits GPU work from same context
 * 4. Both Skia and platform views hit the same vsync
 */
package {{.PackageName}}

import android.os.Handler
import android.os.Looper
import android.view.Choreographer
import org.json.JSONObject
import java.util.concurrent.atomic.AtomicBoolean

class UnifiedFrameOrchestrator(
    private val skiaHost: DriftSkiaHost,
    private val overlayController: InputOverlayController
) : Choreographer.FrameCallback {

    @Volatile
    private var active = false

    private val frameScheduled = AtomicBoolean(false)
    private val mainHandler = Handler(Looper.getMainLooper())

    private val postFrameRunnable = Runnable {
        if (active) {
            Choreographer.getInstance().postFrameCallback(this)
        } else {
            frameScheduled.set(false)
        }
    }

    override fun doFrame(frameTimeNanos: Long) {
        frameScheduled.set(false)
        if (!active || !skiaHost.engineReady) return

        val w = skiaHost.surfaceWidth
        val h = skiaHost.surfaceHeight
        if (w <= 0 || h <= 0) return

        // 1. Step engine pipeline, get geometry snapshot
        val snapshotBytes = NativeBridge.stepAndSnapshot(w, h)

        // 2. Apply platform view geometry synchronously on UI thread
        if (snapshotBytes != null) {
            val snapshot = parseSnapshot(snapshotBytes)
            if (snapshot != null) {
                overlayController.applySnapshot(snapshot)
            }
        }

        // 3. Render Skia into HardwareBuffer + present
        skiaHost.renderFrame()

        // 4. Continue animation if needed
        if (NativeBridge.needsFrame() != 0) {
            scheduleFrame()
        }
    }

    private fun parseSnapshot(data: ByteArray): FrameSnapshot? {
        return try {
            val json = JSONObject(String(data, Charsets.UTF_8))
            val viewsArray = json.optJSONArray("views") ?: return FrameSnapshot(emptyList())

            val views = ArrayList<ViewSnapshot>(viewsArray.length())
            for (i in 0 until viewsArray.length()) {
                val v = viewsArray.getJSONObject(i)
                val hasClip = v.optBoolean("hasClip", false)
                views.add(ViewSnapshot(
                    viewId = v.getLong("viewId"),
                    x = v.getDouble("x").toFloat(),
                    y = v.getDouble("y").toFloat(),
                    width = v.getDouble("width").toFloat(),
                    height = v.getDouble("height").toFloat(),
                    clipLeft = if (hasClip) v.getDouble("clipLeft").toFloat() else null,
                    clipTop = if (hasClip) v.getDouble("clipTop").toFloat() else null,
                    clipRight = if (hasClip) v.getDouble("clipRight").toFloat() else null,
                    clipBottom = if (hasClip) v.getDouble("clipBottom").toFloat() else null,
                    visible = v.optBoolean("visible", true)
                ))
            }
            FrameSnapshot(views)
        } catch (e: Exception) {
            null
        }
    }

    fun scheduleFrame() {
        if (active && frameScheduled.compareAndSet(false, true)) {
            mainHandler.post(postFrameRunnable)
        }
    }

    fun start() {
        active = true
        scheduleFrame()
    }

    fun stop() {
        active = false
        mainHandler.removeCallbacks(postFrameRunnable)
        Choreographer.getInstance().removeFrameCallback(this)
        frameScheduled.set(false)
    }
}

// Data classes for frame snapshot

data class FrameSnapshot(
    val views: List<ViewSnapshot>
)

data class ViewSnapshot(
    val viewId: Long,
    val x: Float,
    val y: Float,
    val width: Float,
    val height: Float,
    val clipLeft: Float?,
    val clipTop: Float?,
    val clipRight: Float?,
    val clipBottom: Float?,
    val visible: Boolean
)
