/**
 * PlatformView.kt
 * Provides platform view management for embedding native views in Drift UI.
 */
package {{.PackageName}}

import android.annotation.SuppressLint
import android.content.Context
import android.graphics.Rect
import android.os.Looper
import android.view.View
import android.view.ViewGroup
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout
import kotlin.math.ceil
import kotlin.math.floor

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
        val clipLeft = (args["clipLeft"] as? Number)?.toFloat()
        val clipTop = (args["clipTop"] as? Number)?.toFloat()
        val clipRight = (args["clipRight"] as? Number)?.toFloat()
        val clipBottom = (args["clipBottom"] as? Number)?.toFloat()

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
            applyClipBounds(container.view, x, y, width, height, clipLeft, clipTop, clipRight, clipBottom, density)
        }

        return Pair(null, null)
    }

    /**
     * Apply clip bounds to a view.
     * Clip bounds are in global logical pixels. We convert to local view coordinates.
     * Uses floor for left/top and ceil for right/bottom to avoid over-clipping.
     */
    private fun applyClipBounds(
        view: View,
        viewX: Float, viewY: Float,
        viewWidth: Float, viewHeight: Float,
        clipLeft: Float?, clipTop: Float?,
        clipRight: Float?, clipBottom: Float?,
        density: Float
    ) {
        // No clip provided - clear any existing clip, but don't change visibility
        // (visibility is controlled by SetVisible or by full clipping below)
        if (clipLeft == null || clipTop == null || clipRight == null || clipBottom == null) {
            view.clipBounds = null
            return
        }

        // Convert global clip to local view coordinates
        val localClipLeft = (clipLeft - viewX) * density
        val localClipTop = (clipTop - viewY) * density
        val localClipRight = (clipRight - viewX) * density
        val localClipBottom = (clipBottom - viewY) * density

        val viewWidthPx = viewWidth * density
        val viewHeightPx = viewHeight * density

        // Clamp to view bounds with floor/ceil for safe rounding
        val left = floor(localClipLeft.coerceIn(0f, viewWidthPx)).toInt()
        val top = floor(localClipTop.coerceIn(0f, viewHeightPx)).toInt()
        val right = ceil(localClipRight.coerceIn(0f, viewWidthPx)).toInt()
        val bottom = ceil(localClipBottom.coerceIn(0f, viewHeightPx)).toInt()

        // Completely clipped - hide view (INVISIBLE keeps layout, GONE would not)
        if (left >= right || top >= bottom) {
            view.visibility = View.INVISIBLE
            view.clipBounds = null  // Clear clip when hidden
            return
        }

        // Fully visible (clip covers entire view) - no clip needed
        // Compare against float values to avoid sub-pixel edge exposure
        if (localClipLeft <= 0f && localClipTop <= 0f &&
            localClipRight >= viewWidthPx && localClipBottom >= viewHeightPx) {
            view.clipBounds = null
            view.visibility = View.VISIBLE
            return
        }

        // Partial clip
        view.clipBounds = Rect(left, top, right, bottom)
        view.visibility = View.VISIBLE
    }

    /**
     * Batch geometry update. Posts geometry changes to the main thread and returns
     * immediately. The frameSeq mechanism ensures stale batches are skipped if the
     * main thread falls behind.
     */
    private fun batchSetGeometry(args: Map<*, *>): Pair<Any?, Exception?> {
        val frameSeq = (args["frameSeq"] as? Number)?.toLong() ?: return Pair(null, null)
        @Suppress("UNCHECKED_CAST")
        val geometries = args["geometries"] as? List<Map<String, Any?>> ?: return Pair(null, null)
        val host = hostView ?: return Pair(null, null)

        if (geometries.isEmpty()) {
            return Pair(null, null)
        }

        // Skip stale batches (older than last applied).
        // Still signal geometry applied so the render thread doesn't timeout.
        if (frameSeq <= lastAppliedSeq) {
            NativeBridge.geometryApplied()
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
                val clipLeft = (geom["clipLeft"] as? Number)?.toFloat()
                val clipTop = (geom["clipTop"] as? Number)?.toFloat()
                val clipRight = (geom["clipRight"] as? Number)?.toFloat()
                val clipBottom = (geom["clipBottom"] as? Number)?.toFloat()

                val container = views[viewId] ?: continue
                container.view.layoutParams = FrameLayout.LayoutParams(
                    (width * density).toInt(),
                    (height * density).toInt()
                ).apply {
                    leftMargin = (x * density).toInt()
                    topMargin = (y * density).toInt()
                }
                applyClipBounds(container.view, x, y, width, height, clipLeft, clipTop, clipRight, clipBottom, density)
            }
            lastAppliedSeq = frameSeq
        }

        // If already on main thread, apply directly and signal immediately.
        if (Looper.myLooper() == Looper.getMainLooper()) {
            applyGeometries()
            NativeBridge.geometryApplied()
            return Pair(null, null)
        }

        // Post to main thread and return immediately.
        // frameSeq ensures stale batches are skipped if main thread falls behind.
        // Signal geometry applied after the closure runs so the render thread
        // can defer surface presentation until geometry lands.
        host.post {
            applyGeometries()
            NativeBridge.geometryApplied()
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
