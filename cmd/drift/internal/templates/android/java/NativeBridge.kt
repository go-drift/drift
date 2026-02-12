/**
 * NativeBridge provides the Kotlin interface to the native Drift engine.
 *
 * This singleton object serves as the bridge between Kotlin code and the native
 * C/Go code that powers the Drift rendering engine. It handles:
 *   1. Loading the required native libraries (libdrift.so and libdrift_jni.so)
 *   2. Declaring external JNI functions that are implemented in C (drift_jni.c)
 *
 * Architecture:
 *
 *     Kotlin (this file)
 *           │
 *           ▼ JNI
 *     C bridge (drift_jni.c)
 *           │
 *           ▼ dlopen/dlsym
 *     Go engine (libdrift.so)
 *
 * Library Loading Order:
 *   The libraries MUST be loaded in this specific order because:
 *   1. libdrift.so is the Go shared library containing the engine
 *   2. libdrift_jni.so depends on libdrift.so (it calls its functions)
 *
 * Usage:
 *   - Call renderFrameSkia() from your OpenGL renderer's onDrawFrame() method
 *   - Call pointerEvent() from your view's onTouchEvent() method
 *
 * Thread Safety:
 *   These functions are safe to call from any thread. The Go engine handles
 *   synchronization internally using mutexes.
 */
package {{.PackageName}}


/**
 * Singleton object providing JNI access to the native Drift engine.
 *
 * Using an object (singleton) ensures the native libraries are loaded exactly once,
 * when this class is first accessed. The init block runs before any external functions
 * can be called, guaranteeing the libraries are available.
 */
object NativeBridge {
    /**
     * Static initializer that loads the native libraries.
     *
     * This block runs when the NativeBridge object is first referenced.
     * Loading happens in dependency order: the Go library first, then the JNI bridge.
     *
     * Throws: UnsatisfiedLinkError if the libraries cannot be found or loaded.
     * This typically means the .so files are missing from the APK's lib/ directory.
     */
    init {
        // Ensure the shared C++ runtime is loaded before native libraries.
        System.loadLibrary("c++_shared")

        // Load the Go engine library first (contains DriftSkiaInitGL, DriftSkiaRenderGL, DriftPointerEvent)
        System.loadLibrary("drift")

        // Load the JNI bridge library second (contains Java_com_drift_embedder_* functions)
        // This library dynamically links to libdrift.so at runtime
        System.loadLibrary("drift_jni")
    }

    /**
     * Initializes the Go application by calling main() once.
     *
     * Call this before any rendering begins.
     *
     * @return 0 on success, non-zero on failure.
     */
    external fun appInit(): Int

    /**
     * Initializes the Skia GL backend using the current OpenGL context.
     *
     * Call this once from the GL thread (e.g., in GLSurfaceView.Renderer.onSurfaceCreated).
     *
     * @return 0 on success, non-zero on failure.
     */
    external fun initSkiaGL(): Int

    /**
     * Renders a frame directly into the current OpenGL framebuffer using Skia.
     *
     * @param width  Width of the render target in pixels.
     * @param height Height of the render target in pixels.
     *
     * @return 0 on success, non-zero on failure.
     */
    external fun renderFrameSkia(width: Int, height: Int): Int

    /**
     * Sends a pointer/touch event to the Go engine.
     *
     * This function notifies the engine of touch input, allowing interactive
     * elements (like the draggable circle in the demo) to respond to user input.
     *
     * @param pointerID Unique identifier for this pointer/touch (enables multi-touch).
     *                  On Android, use MotionEvent.getPointerId() for each pointer.
     * @param phase The phase of the touch event:
     *              0 = Down (finger touched screen)
     *              1 = Move (finger moved while touching)
     *              2 = Up (finger lifted from screen)
     *              3 = Cancel (touch cancelled by system)
     * @param x     X coordinate in pixels (view coordinates, not dp)
     * @param y     Y coordinate in pixels (view coordinates, not dp)
     *
     * Coordinate System:
     *   - Origin (0, 0) is at the top-left corner of the view
     *   - X increases to the right
     *   - Y increases downward
     *   - Coordinates should match the framebuffer dimensions passed to renderFrameSkia()
     *
     * Thread Safety:
     *   This function is thread-safe. Typically called from the main/UI thread
     *   in response to MotionEvents, but can be called from any thread.
     */
    external fun pointerEvent(pointerID: Long, phase: Int, x: Double, y: Double)

    /**
     * Updates the device scale factor used by the Go engine for logical sizing.
     *
     * @param scale The display density scale (e.g., 2.0 on xhdpi devices).
     *
     * Thread Safety:
     *   This function is thread-safe and can be called from any thread.
     */
    external fun setDeviceScale(scale: Double)

    // Platform Channel methods

    /**
     * Sends an event to Go event listeners for the given channel.
     *
     * @param channel The channel name (e.g., "drift/lifecycle/events").
     * @param data    JSON-encoded event data.
     * @param dataLen Length of the data array.
     */
    external fun platformHandleEvent(channel: String, data: ByteArray, dataLen: Int)

    /**
     * Sends an error to Go event listeners for the given channel.
     *
     * @param channel The channel name.
     * @param code    Error code.
     * @param message Error message.
     */
    external fun platformHandleEventError(channel: String, code: String, message: String)

    /**
     * Notifies Go that an event stream has ended for the given channel.
     *
     * @param channel The channel name.
     */
    external fun platformHandleEventDone(channel: String)

    /**
     * Checks if Go is listening to events on the given channel.
     *
     * @param channel The channel name.
     * @return 1 if active, 0 if not.
     */
    external fun platformIsStreamActive(channel: String): Int

    /**
     * Initializes platform channels by registering the native callback with Go.
     * Must be called after the Go library is loaded and before using platform channels.
     *
     * @return 0 on success, non-zero on failure.
     */
    external fun platformInit(): Int

    /**
     * Notifies the Go engine that the Android back button was pressed.
     *
     * Called from MainActivity when the system back button is pressed.
     * The Go engine will attempt to pop the current navigation route.
     *
     * @return 1 if the back was handled (route popped), 0 if not (at root route).
     */
    external fun backButtonPressed(): Int

    /**
     * Requests the Go engine to schedule a new frame.
     */
    external fun requestFrame()

    /**
     * Checks if a new frame needs to be rendered.
     *
     * Call this before renderFrameSkia() to skip unnecessary render cycles
     * when nothing has changed (no animations, no user input, no state changes).
     *
     * @return 1 if a new frame should be rendered, 0 if the frame can be skipped.
     */
    external fun needsFrame(): Int

    /**
     * Queries the Go engine's hit test to determine if a platform view is the
     * topmost target at the given pixel coordinates.
     *
     * Called synchronously from TouchInterceptorView on ACTION_DOWN before
     * deciding whether to intercept touches.
     *
     * @param viewID The platform view ID to check.
     * @param x      X coordinate in pixels (relative to the surface view).
     * @param y      Y coordinate in pixels (relative to the surface view).
     * @return 1 if the view is topmost (allow touch), 0 if obscured (block touch).
     */
    external fun hitTestPlatformView(viewID: Long, x: Double, y: Double): Int

    /**
     * Signals the Go render thread that platform view geometry has been applied.
     *
     * Called from the main thread after applying geometry updates so the
     * render thread can proceed with surface presentation (eglSwapBuffers).
     */
    external fun geometryApplied()
}
