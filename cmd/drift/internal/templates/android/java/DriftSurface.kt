/**
 * DriftSurface manages the AHardwareBuffer pool used for SurfaceControl rendering.
 *
 * Wraps the native buffer pool (triple-buffered AHardwareBuffers with EGL image-backed
 * FBOs) and provides Kotlin-friendly methods for acquiring buffers, creating GPU
 * fences, and handling resizes.
 *
 * Must be created and used on the render thread (requires an active EGL context).
 */
package {{.PackageName}}

class DriftSurface(width: Int, height: Int) {
    /** Native pool pointer, exposed for presentBuffer() calls. */
    var poolPtr: Long = 0L
        private set

    private var currentWidth = width
    private var currentHeight = height

    init {
        if (width > 0 && height > 0) {
            poolPtr = NativeBridge.createBufferPool(width, height, BUFFER_COUNT)
        }
    }

    /**
     * Acquires the next buffer in the pool and binds its FBO.
     * Returns the buffer index, or -1 on failure.
     */
    fun acquireBuffer(): Int {
        if (poolPtr == 0L) return -1
        return NativeBridge.acquireBuffer(poolPtr)
    }

    /**
     * Creates a GPU fence for the current rendering operation.
     * Returns a native fence FD, or -1 if fences are unsupported (glFinish fallback).
     */
    fun createFence(): Int {
        if (poolPtr == 0L) return -1
        return NativeBridge.createFence(poolPtr)
    }

    /**
     * Resizes the buffer pool. Destroys and recreates all buffers at the new size.
     * On failure, destroys the old pool and creates a fresh one.
     */
    fun resize(width: Int, height: Int) {
        if (width <= 0 || height <= 0) return
        if (width == currentWidth && height == currentHeight && poolPtr != 0L) return

        if (poolPtr == 0L) {
            // Pool was never created (zero-size start) or a previous failure
            // left it destroyed. Create fresh.
            poolPtr = NativeBridge.createBufferPool(width, height, BUFFER_COUNT)
            if (poolPtr != 0L) {
                currentWidth = width
                currentHeight = height
            }
            return
        }

        if (NativeBridge.resizeBufferPool(poolPtr, width, height) == 0) {
            currentWidth = width
            currentHeight = height
            return
        }
        // Resize failed and the pool is now empty. Destroy it and try a fresh allocation.
        NativeBridge.destroyBufferPool(poolPtr)
        poolPtr = NativeBridge.createBufferPool(width, height, BUFFER_COUNT)
        if (poolPtr != 0L) {
            currentWidth = width
            currentHeight = height
        }
    }

    /**
     * Destroys the buffer pool and releases all native resources.
     */
    fun destroy() {
        if (poolPtr != 0L) {
            NativeBridge.destroyBufferPool(poolPtr)
            poolPtr = 0L
        }
    }

    companion object {
        private const val BUFFER_COUNT = 3
    }
}
