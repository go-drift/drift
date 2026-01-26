/**
 * PlatformView.kt
 * Provides platform view management for embedding native views in Drift UI.
 */
package {{.PackageName}}

import android.annotation.SuppressLint
import android.content.Context
import android.os.Looper
import android.view.View
import android.view.ViewGroup
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit

/**
 * Handles platform view channel methods from Go.
 */
object PlatformViewHandler {
    private val views = mutableMapOf<Int, PlatformViewContainer>()
    private var context: Context? = null
    private var hostView: ViewGroup? = null

    // Frame sequence tracking for geometry batches
    private var lastAppliedSeq: Long = 0

    // Supported methods for each view type
    private val webViewMethods = setOf("loadUrl", "goBack", "goForward", "reload")
    private val textInputMethods = setOf("setText", "setSelection", "setValue", "focus", "blur", "updateConfig")
    private val switchMethods = setOf("setValue", "updateConfig")
    private val activityIndicatorMethods = setOf("setAnimating", "updateConfig")

    fun init(context: Context, hostView: ViewGroup) {
        this.context = context
        this.hostView = hostView
    }

    fun handle(method: String, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))

        return when (method) {
            "create" -> create(argsMap)
            "dispose" -> dispose(argsMap)
            "setGeometry" -> setGeometry(argsMap)
            "batchSetGeometry" -> batchSetGeometry(argsMap)
            "setVisible" -> setVisible(argsMap)
            "setEnabled" -> setEnabled(argsMap)
            "invokeViewMethod" -> invokeViewMethod(argsMap)
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun invokeViewMethod(args: Map<*, *>): Pair<Any?, Exception?> {
        val viewId = (args["viewId"] as? Number)?.toInt()
            ?: return Pair(null, IllegalArgumentException("Missing viewId"))
        val method = args["method"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing method"))
        val host = hostView ?: return Pair(null, IllegalStateException("Host view not initialized"))

        val container = views[viewId]
            ?: return Pair(null, IllegalArgumentException("View not found: $viewId"))

        // Validate method is supported before posting
        val supported = when (container) {
            is NativeWebViewContainer -> method in webViewMethods
            is NativeTextInputContainer -> method in textInputMethods
            is NativeSwitchContainer -> method in switchMethods
            is NativeActivityIndicatorContainer -> method in activityIndicatorMethods
            else -> false
        }
        if (!supported) {
            return Pair(null, IllegalArgumentException("Unknown method '$method' for view type"))
        }

        host.post {
            when (container) {
                is NativeWebViewContainer -> {
                    when (method) {
                        "loadUrl" -> {
                            val url = args["url"] as? String
                            if (url != null) {
                                container.view.loadUrl(url)
                            }
                        }
                        "goBack" -> container.view.goBack()
                        "goForward" -> container.view.goForward()
                        "reload" -> container.view.reload()
                    }
                }
                is NativeTextInputContainer -> {
                    when (method) {
                        "setText" -> {
                            val text = args["text"] as? String ?: ""
                            container.setText(text)
                        }
                        "setSelection" -> {
                            val base = (args["selectionBase"] as? Number)?.toInt() ?: 0
                            val extent = (args["selectionExtent"] as? Number)?.toInt() ?: 0
                            container.setSelection(base, extent)
                        }
                        "setValue" -> {
                            val text = args["text"] as? String ?: ""
                            val base = (args["selectionBase"] as? Number)?.toInt() ?: text.length
                            val extent = (args["selectionExtent"] as? Number)?.toInt() ?: text.length
                            container.setValue(text, base, extent)
                        }
                        "focus" -> container.focus()
                        "blur" -> container.blur()
                        "updateConfig" -> {
                            @Suppress("UNCHECKED_CAST")
                            container.updateConfig(args as Map<String, Any?>)
                        }
                    }
                }
                is NativeSwitchContainer -> {
                    when (method) {
                        "setValue" -> {
                            val value = args["value"] as? Boolean ?: false
                            container.setValue(value)
                        }
                        "updateConfig" -> {
                            @Suppress("UNCHECKED_CAST")
                            container.updateConfig(args as Map<String, Any?>)
                        }
                    }
                }
                is NativeActivityIndicatorContainer -> {
                    when (method) {
                        "setAnimating" -> {
                            val animating = args["animating"] as? Boolean ?: true
                            container.setAnimating(animating)
                        }
                        "updateConfig" -> {
                            @Suppress("UNCHECKED_CAST")
                            container.updateConfig(args as Map<String, Any?>)
                        }
                    }
                }
            }
        }

        return Pair(null, null)
    }

    private fun create(args: Map<*, *>): Pair<Any?, Exception?> {
        val viewId = (args["viewId"] as? Number)?.toInt()
            ?: return Pair(null, IllegalArgumentException("Missing viewId"))
        val viewType = args["viewType"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing viewType"))

        @Suppress("UNCHECKED_CAST")
        val params = args["params"] as? Map<String, Any?> ?: emptyMap()

        val ctx = context ?: return Pair(null, IllegalStateException("Context not initialized"))
        val host = hostView ?: return Pair(null, IllegalStateException("Host view not initialized"))

        val creator: (() -> PlatformViewContainer)? = when (viewType) {
            "native_webview" -> { { NativeWebViewContainer(ctx, viewId, params) } }
            "textinput" -> { { NativeTextInputContainer(ctx, viewId, params) } }
            "switch" -> { { NativeSwitchContainer(ctx, viewId, params) } }
            "activity_indicator" -> { { NativeActivityIndicatorContainer(ctx, viewId, params) } }
            else -> null
        }

        if (creator == null) {
            return Pair(null, IllegalArgumentException("Unknown view type: $viewType"))
        }

        // Add to host view on main thread
        host.post {
            val container = creator()
            views[viewId] = container
            container.view.visibility = View.GONE // Hidden until positioned
            host.addView(container.view)

            // Notify Go that view is created
            PlatformChannelManager.sendEvent(
                "drift/platform_views",
                mapOf(
                    "method" to "onViewCreated",
                    "viewId" to viewId
                )
            )
        }

        return Pair(mapOf("created" to true), null)
    }

    private fun dispose(args: Map<*, *>): Pair<Any?, Exception?> {
        val viewId = (args["viewId"] as? Number)?.toInt() ?: return Pair(null, null)
        val host = hostView ?: return Pair(null, null)

        host.post {
            val container = views.remove(viewId) ?: return@post
            container.dispose()
            host.removeView(container.view)
        }

        return Pair(null, null)
    }

    private fun setGeometry(args: Map<*, *>): Pair<Any?, Exception?> {
        val viewId = (args["viewId"] as? Number)?.toInt() ?: return Pair(null, null)
        val host = hostView ?: return Pair(null, null)

        val x = (args["x"] as? Number)?.toFloat() ?: 0f
        val y = (args["y"] as? Number)?.toFloat() ?: 0f
        val width = (args["width"] as? Number)?.toFloat() ?: 0f
        val height = (args["height"] as? Number)?.toFloat() ?: 0f

        host.post {
            val container = views[viewId] ?: return@post
            val density = context?.resources?.displayMetrics?.density ?: 1f
            container.view.layoutParams = FrameLayout.LayoutParams(
                (width * density).toInt(),
                (height * density).toInt()
            ).apply {
                leftMargin = (x * density).toInt()
                topMargin = (y * density).toInt()
            }
            container.view.visibility = View.VISIBLE
        }

        return Pair(null, null)
    }

    /**
     * Batch geometry update with synchronization.
     * Blocks until all geometries are applied on the main thread.
     * This ensures native views are positioned before the frame is displayed.
     */
    private fun batchSetGeometry(args: Map<*, *>): Pair<Any?, Exception?> {
        val frameSeq = (args["frameSeq"] as? Number)?.toLong() ?: return Pair(null, null)
        @Suppress("UNCHECKED_CAST")
        val geometries = args["geometries"] as? List<Map<String, Any?>> ?: return Pair(null, null)
        val host = hostView ?: return Pair(null, null)

        if (geometries.isEmpty()) {
            return Pair(null, null)
        }

        // Skip stale batches (older than last applied)
        if (frameSeq <= lastAppliedSeq) {
            return Pair(null, null)
        }

        val density = context?.resources?.displayMetrics?.density ?: 1f

        val applyGeometries = {
            for (geom in geometries) {
                val viewId = (geom["viewId"] as? Number)?.toInt() ?: continue
                val x = (geom["x"] as? Number)?.toFloat() ?: 0f
                val y = (geom["y"] as? Number)?.toFloat() ?: 0f
                val width = (geom["width"] as? Number)?.toFloat() ?: 0f
                val height = (geom["height"] as? Number)?.toFloat() ?: 0f

                val container = views[viewId] ?: continue
                container.view.layoutParams = FrameLayout.LayoutParams(
                    (width * density).toInt(),
                    (height * density).toInt()
                ).apply {
                    leftMargin = (x * density).toInt()
                    topMargin = (y * density).toInt()
                }
                container.view.visibility = View.VISIBLE
            }
            lastAppliedSeq = frameSeq
        }

        // If already on main thread, apply directly (avoid deadlock)
        if (Looper.myLooper() == Looper.getMainLooper()) {
            applyGeometries()
            return Pair(null, null)
        }

        // Block until main thread applies all geometries
        val latch = CountDownLatch(1)
        host.post {
            applyGeometries()
            latch.countDown()
        }

        // Wait with timeout to prevent indefinite blocking
        // 16ms is roughly one frame at 60fps
        val completed = latch.await(16, TimeUnit.MILLISECONDS)
        if (!completed) {
            // Timeout - main thread is busy. The geometries will still be applied
            // asynchronously, but this frame may show slight lag.
            return Pair(mapOf("timeout" to true), null)
        }

        return Pair(null, null)
    }

    private fun setVisible(args: Map<*, *>): Pair<Any?, Exception?> {
        val viewId = (args["viewId"] as? Number)?.toInt() ?: return Pair(null, null)
        val visible = args["visible"] as? Boolean ?: true
        val host = hostView ?: return Pair(null, null)

        host.post {
            val container = views[viewId] ?: return@post
            container.view.visibility = if (visible) View.VISIBLE else View.GONE
        }

        return Pair(null, null)
    }

    private fun setEnabled(args: Map<*, *>): Pair<Any?, Exception?> {
        val viewId = (args["viewId"] as? Number)?.toInt() ?: return Pair(null, null)
        val enabled = args["enabled"] as? Boolean ?: true
        val host = hostView ?: return Pair(null, null)

        host.post {
            val container = views[viewId] ?: return@post
            container.view.isEnabled = enabled
            container.view.alpha = if (enabled) 1.0f else 0.5f
        }

        return Pair(null, null)
    }
}

/**
 * Interface for platform view containers.
 */
interface PlatformViewContainer {
    val viewId: Int
    val view: View
    fun dispose()
}

/**
 * Native web view container.
 */
@SuppressLint("SetJavaScriptEnabled")
class NativeWebViewContainer(
    context: Context,
    override val viewId: Int,
    params: Map<String, Any?>
) : PlatformViewContainer {

    override val view: WebView = WebView(context).apply {
        layoutParams = FrameLayout.LayoutParams(
            FrameLayout.LayoutParams.MATCH_PARENT,
            FrameLayout.LayoutParams.MATCH_PARENT
        )

        // Enable JavaScript
        settings.javaScriptEnabled = true
        settings.domStorageEnabled = true

        webViewClient = object : WebViewClient() {
            override fun onPageStarted(view: WebView?, url: String?, favicon: android.graphics.Bitmap?) {
                super.onPageStarted(view, url, favicon)
                PlatformChannelManager.sendEvent(
                    "drift/platform_views",
                    mapOf(
                        "method" to "onPageStarted",
                        "viewId" to viewId,
                        "url" to (url ?: "")
                    )
                )
            }

            override fun onPageFinished(view: WebView?, url: String?) {
                super.onPageFinished(view, url)
                PlatformChannelManager.sendEvent(
                    "drift/platform_views",
                    mapOf(
                        "method" to "onPageFinished",
                        "viewId" to viewId,
                        "url" to (url ?: "")
                    )
                )
            }

            override fun onReceivedError(
                view: WebView?,
                request: android.webkit.WebResourceRequest?,
                error: android.webkit.WebResourceError?
            ) {
                super.onReceivedError(view, request, error)
                val errorMessage = if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.M) {
                    error?.description?.toString() ?: "Unknown error"
                } else {
                    "Unknown error"
                }
                PlatformChannelManager.sendEvent(
                    "drift/platform_views",
                    mapOf(
                        "method" to "onError",
                        "viewId" to viewId,
                        "error" to errorMessage
                    )
                )
            }
        }

        // Load initial URL if provided
        (params["initialUrl"] as? String)?.let { url ->
            loadUrl(url)
        }
    }

    override fun dispose() {
        view.stopLoading()
        view.destroy()
    }
}
