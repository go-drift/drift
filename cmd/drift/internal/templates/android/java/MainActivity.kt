/**
 * MainActivity is the entry point for the Drift Android application.
 *
 * Uses the unified frame orchestrator: all rendering happens on the UI thread
 * via Choreographer callbacks, with HardwareBuffer + HWUI onDraw for Skia
 * output. Platform views are positioned synchronously within the same callback,
 * eliminating visual lag during scrolling.
 */
package {{.PackageName}}

import android.content.Intent
import android.os.Bundle
import android.util.Log
import android.view.ViewGroup
import androidx.activity.OnBackPressedCallback
import androidx.appcompat.app.AppCompatActivity
import androidx.core.view.ViewCompat
import com.drift.runner.DriftPlugins

class MainActivity : AppCompatActivity() {

    private lateinit var container: DriftContainer
    private lateinit var orchestrator: UnifiedFrameOrchestrator

    override fun onCreate(savedInstanceState: Bundle?) {
        // Channels and plugins register once per process (later calls are
        // no-ops).
        PlatformChannelManager.init(applicationContext)
        DriftPlugins.register(PlatformChannelManager)

        setTheme(R.style.AppTheme)
        super.onCreate(savedInstanceState)

        Log.i("DriftDeepLink", "onCreate intent action=${intent?.action} data=${intent?.dataString}")
        // A recreated Activity (dark mode, locale) or one relaunched from
        // Recents carries the launch intent again; its tap was handled.
        val relaunched = savedInstanceState != null ||
            (intent?.flags ?: 0) and Intent.FLAG_ACTIVITY_LAUNCHED_FROM_HISTORY != 0
        if (!relaunched) {
            NotificationHandler.handleNotificationOpen(intent)
        }
        DeepLinkHandler.handleIntent(intent, "launch")

        container = DriftContainer(this)
        setContentView(container)

        val density = resources.displayMetrics.density
        val overlayController = InputOverlayController(container.overlayLayout, density)
        orchestrator = UnifiedFrameOrchestrator(container.skiaView, overlayController)

        // Wire frame scheduling from SkiaHostView to orchestrator
        container.skiaView.onFrameNeeded = { orchestrator.scheduleFrame() }

        PlatformChannelManager.setView(container.skiaView)
        PlatformChannelManager.setOnFrameNeeded { orchestrator.scheduleFrame() }

        PlatformViewHandler.init(this, container.overlayLayout, container.skiaView, overlayController)

        AccessibilityHandler.initialize(this, container.skiaView)

        // Set up safe area insets listener
        ViewCompat.setOnApplyWindowInsetsListener(container) { _, insets ->
            SafeAreaHandler.sendInsetsUpdate()
            insets
        }
        container.post { SafeAreaHandler.sendInsetsUpdate() }

        // Handle back button presses via the Go navigation system
        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                val handled = NativeBridge.backButtonPressed()
                if (handled == 0) {
                    isEnabled = false
                    onBackPressedDispatcher.onBackPressed()
                    isEnabled = true
                } else {
                    NativeBridge.requestFrame()
                    orchestrator.scheduleFrame()
                }
            }
        })

        // Plugins attach once the views exist; the window's root view hosts
        // their overlays above Drift's content.
        DriftPlugins.attach(this, window.decorView as ViewGroup)
    }

    override fun onNewIntent(intent: Intent) {
        super.onNewIntent(intent)
        setIntent(intent)
        Log.i("DriftDeepLink", "onNewIntent action=${intent.action} data=${intent.dataString}")
        NotificationHandler.handleNotificationOpen(intent)
        if (!DriftPlugins.onNewIntent(intent)) {
            DeepLinkHandler.handleIntent(intent, "open")
        }
        if (::orchestrator.isInitialized) {
            orchestrator.start()
        }
    }

    override fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        PermissionHandler.onRequestPermissionsResult(this, requestCode, permissions, grantResults)
    }

    @Deprecated("Deprecated in Java")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        super.onActivityResult(requestCode, resultCode, data)
        CameraHandler.onActivityResult(requestCode, resultCode, data, this)
        StorageHandler.onActivityResult(requestCode, resultCode, data, this)
    }

    override fun onResume() {
        super.onResume()
        container.skiaView.notifyResume()
        orchestrator.start()
    }

    override fun onPause() {
        super.onPause()
        orchestrator.stop()
    }

    override fun onDestroy() {
        DriftPlugins.detach(this)
        super.onDestroy()
    }
}
