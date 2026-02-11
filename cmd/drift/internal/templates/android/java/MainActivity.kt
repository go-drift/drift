/**
 * MainActivity is the entry point for the Drift Android application.
 *
 * This activity hosts the DriftSurfaceView, which displays the Go-rendered content.
 * It handles the Android activity lifecycle, ensuring the OpenGL surface is properly
 * paused and resumed with the application.
 *
 * Architecture:
 *
 *     Android System
 *           |
 *           v Activity lifecycle
 *     MainActivity
 *           |
 *           v setContentView()
 *     DriftSurfaceView (SurfaceView + SurfaceControl)
 *           |
 *           v render thread
 *     DriftRenderer
 *           |
 *           v JNI calls
 *     Go Engine
 *
 * Lifecycle Management:
 *   - onCreate(): Creates the DriftSurfaceView and sets it as content
 *   - onResume(): Resumes the GL rendering thread
 *   - onPause(): Pauses the GL rendering thread to save battery
 */
package {{.PackageName}}

import android.os.Bundle
import android.util.Log
import androidx.activity.OnBackPressedCallback
import androidx.appcompat.app.AppCompatActivity
import androidx.core.view.ViewCompat
import androidx.core.view.WindowCompat

/**
 * Main activity that hosts the Drift rendering surface.
 *
 * Extends AppCompatActivity for compatibility with older Android versions
 * and access to the AndroidX support libraries.
 */
class MainActivity : AppCompatActivity() {
    /**
     * The OpenGL surface view that displays the Drift engine output.
     *
     * Using lateinit because it's initialized in onCreate() before any
     * other methods access it. This avoids null checks throughout the class.
     */
    private lateinit var surfaceView: DriftSurfaceView

    /**
     * Container layout that holds the surface view and platform views.
     */
    private lateinit var container: DriftContainer

    /**
     * Called when the activity is first created.
     *
     * Creates the DriftSurfaceView and sets it as the activity's content.
     * The surface view will initialize OpenGL and start the render loop
     * when it's attached to the window.
     *
     * @param savedInstanceState Previously saved state (unused in this app)
     */
    override fun onCreate(savedInstanceState: Bundle?) {
        setTheme(R.style.AppTheme)
        super.onCreate(savedInstanceState)

        PlatformChannelManager.init(applicationContext)
        Log.i("DriftDeepLink", "onCreate intent action=${intent?.action} data=${intent?.dataString}")
        NotificationHandler.handleNotificationOpen(intent)
        DeepLinkHandler.handleIntent(intent, "launch")

        // Create a container for the surface view and platform views.
        container = DriftContainer(this)

        surfaceView = DriftSurfaceView(this)
        container.addView(surfaceView, android.widget.FrameLayout.LayoutParams(
            android.widget.FrameLayout.LayoutParams.MATCH_PARENT,
            android.widget.FrameLayout.LayoutParams.MATCH_PARENT
        ))

        setContentView(container)
        WindowCompat.setDecorFitsSystemWindows(window, false)
        window.decorView.requestApplyInsets()

        PlatformChannelManager.setView(surfaceView)
        PlatformChannelManager.setOnFrameNeeded { surfaceView.scheduleFrame() }

        // Initialize platform view handler with the container and surface view
        PlatformViewHandler.init(this, container)
        PlatformViewHandler.setSurfaceView(surfaceView)

        // Initialize accessibility support
        AccessibilityHandler.initialize(this, surfaceView)

        // Set up safe area insets listener
        ViewCompat.setOnApplyWindowInsetsListener(container) { _, insets ->
            SafeAreaHandler.sendInsetsUpdate()
            insets
        }

        // Push initial insets eagerly so the first Go layout has correct safe area
        // values. The listener above only fires when the system dispatches insets,
        // which may happen after the first frame. Posting defers the call until
        // the view is attached and getRootWindowInsets() returns non-null.
        container.post { SafeAreaHandler.sendInsetsUpdate() }

        // Handle back button presses via the Go navigation system
        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                val handled = NativeBridge.backButtonPressed()
                if (handled == 0) {
                    // Go didn't handle it (at root), let system handle (exit app)
                    isEnabled = false
                    onBackPressedDispatcher.onBackPressed()
                    isEnabled = true
                } else {
                    // Go handled the back button - render the navigation change
                    surfaceView.renderNow()
                }
            }
        })
    }

    override fun onNewIntent(intent: android.content.Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        Log.i("DriftDeepLink", "onNewIntent action=${intent.action} data=${intent.dataString}")
        NotificationHandler.handleNotificationOpen(intent)
        DeepLinkHandler.handleIntent(intent, "open")
        if (::surfaceView.isInitialized) {
            surfaceView.resumeRendering()
            surfaceView.resumeScheduling()
            // renderNow() adds an immediate render on top of the Choreographer
            // callback from resumeScheduling(), giving lower latency for deep links.
            surfaceView.renderNow()
        }
    }

    override fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        PermissionHandler.onRequestPermissionsResult(this, requestCode, permissions, grantResults)
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: android.content.Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        CameraHandler.onActivityResult(requestCode, resultCode, data, this)
        StorageHandler.onActivityResult(requestCode, resultCode, data, this)
    }

    /**
     * Called when the activity becomes visible and interactive.
     *
     * Resumes the render thread and re-enables Choreographer callbacks.
     */
    override fun onResume() {
        super.onResume()
        surfaceView.resumeRendering()  // Resume render thread
        surfaceView.resumeScheduling() // Enable Choreographer callbacks
    }

    /**
     * Called when the activity is no longer in the foreground.
     *
     * Pauses the render thread to conserve battery and CPU.
     * This is called when:
     *   - The user presses Home or switches to another app
     *   - A dialog or other activity appears on top
     *   - The screen turns off
     *
     * The EGL context is preserved, so rendering will resume seamlessly
     * when onResume() is called.
     */
    override fun onPause() {
        super.onPause()
        surfaceView.pauseScheduling() // Disable Choreographer callbacks
        surfaceView.pauseRendering()  // Pause render thread
    }
}
