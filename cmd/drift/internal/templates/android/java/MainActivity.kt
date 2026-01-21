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
 *           │
 *           ▼ Activity lifecycle
 *     MainActivity
 *           │
 *           ▼ setContentView()
 *     DriftSurfaceView (GLSurfaceView)
 *           │
 *           ▼ OpenGL rendering
 *     DriftRenderer
 *           │
 *           ▼ JNI calls
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
import android.widget.FrameLayout
import androidx.activity.OnBackPressedCallback
import androidx.appcompat.app.AppCompatActivity
import androidx.core.view.ViewCompat

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
    private lateinit var container: FrameLayout

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
        super.onCreate(savedInstanceState)

        PlatformChannelManager.init(applicationContext)
        Log.i("DriftDeepLink", "onCreate intent action=${intent?.action} data=${intent?.dataString}")
        NotificationHandler.handleNotificationOpen(intent)
        DeepLinkHandler.handleIntent(intent, "launch")

        // Create a container for the surface view and platform views
        container = FrameLayout(this)

        surfaceView = DriftSurfaceView(this)
        container.addView(surfaceView, FrameLayout.LayoutParams(
            FrameLayout.LayoutParams.MATCH_PARENT,
            FrameLayout.LayoutParams.MATCH_PARENT
        ))

        setContentView(container)

        PlatformChannelManager.setView(surfaceView)

        // Initialize text input and platform view handlers with the container
        TextInputHandler.init(this, container)
        PlatformViewHandler.init(this, container)

        // Initialize accessibility support
        AccessibilityHandler.initialize(this, surfaceView)

        // Set up safe area insets listener
        ViewCompat.setOnApplyWindowInsetsListener(container) { _, insets ->
            SafeAreaHandler.sendInsetsUpdate()
            insets
        }

        // Handle back button presses via the Go navigation system
        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                val handled = NativeBridge.backButtonPressed()
                if (handled == 0) {
                    // Go didn't handle it (at root), let system handle (exit app)
                    isEnabled = false
                    onBackPressedDispatcher.onBackPressed()
                    isEnabled = true
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
            surfaceView.onResume()
            NativeBridge.requestFrame()
            surfaceView.requestRender()
        }
    }

    override fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        NotificationHandler.onRequestPermissionsResult(requestCode, grantResults)
    }

    /**
     * Called when the activity becomes visible and interactive.
     *
     * Resumes the OpenGL rendering thread. This is important because:
     *   - onPause() stops the GL thread to save battery
     *   - The GL thread needs to be explicitly resumed when returning
     *
     * GLSurfaceView manages its own thread, but we must call onResume()
     * to restart it after it was paused.
     */
    override fun onResume() {
        super.onResume()
        surfaceView.onResume()
        NativeBridge.requestFrame()
        surfaceView.requestRender()
    }

    /**
     * Called when the activity is no longer in the foreground.
     *
     * Pauses the OpenGL rendering thread to conserve battery and CPU.
     * This is called when:
     *   - The user presses Home or switches to another app
     *   - A dialog or other activity appears on top
     *   - The screen turns off
     *
     * The GL context is preserved, so rendering will resume seamlessly
     * when onResume() is called.
     */
    override fun onPause() {
        super.onPause()
        surfaceView.onPause()
    }
}
