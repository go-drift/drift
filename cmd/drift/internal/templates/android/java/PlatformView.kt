/**
 * PlatformView.kt
 * Provides platform view management for embedding native views in Drift UI.
 */
package {{.PackageName}}

import android.annotation.SuppressLint
import android.content.Context
import android.view.View
import android.view.ViewGroup
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.FrameLayout

/**
 * Handles platform view channel methods from Go.
 */
object PlatformViewHandler {
    private val views = mutableMapOf<Int, PlatformViewContainer>()
    private val interceptors = mutableMapOf<Int, TouchInterceptorView>()
    private var context: Context? = null
    private var hostView: ViewGroup? = null
    private var surfaceView: View? = null
    private var overlayController: InputOverlayController? = null

    // Supported methods for each view type
    private val webViewMethods = setOf("load", "goBack", "goForward", "reload")
    private val textInputMethods = setOf("setText", "setSelection", "setValue", "focus", "blur", "updateConfig")
    private val switchMethods = setOf("setValue", "updateConfig")
    private val activityIndicatorMethods = setOf("setAnimating", "updateConfig")
    private val videoPlayerMethods = setOf("play", "pause", "stop", "seekTo", "setVolume", "setLooping", "setPlaybackSpeed", "load")

    fun init(context: Context, hostView: ViewGroup, surfaceView: View, overlayController: InputOverlayController) {
        this.context = context
        this.hostView = hostView
        this.surfaceView = surfaceView
        this.overlayController = overlayController
    }

    fun handle(method: String, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))

        return when (method) {
            "create" -> create(argsMap)
            "dispose" -> dispose(argsMap)
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

        // Post the entire lookup + dispatch to the UI thread so it is ordered
        // after the create() post that populates views[viewId].
        host.post {
            val container = views[viewId] ?: return@post

            val supported = when (container) {
                is NativeWebViewContainer -> method in webViewMethods
                is NativeTextInputContainer -> method in textInputMethods
                is NativeSwitchContainer -> method in switchMethods
                is NativeActivityIndicatorContainer -> method in activityIndicatorMethods
                is NativeVideoPlayerContainer -> method in videoPlayerMethods
                else -> false
            }
            if (!supported) return@post

            when (container) {
                is NativeWebViewContainer -> {
                    when (method) {
                        "load" -> {
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
                            val multiline = args["multiline"] as? Boolean ?: false
                            interceptors[viewId]?.enableUnfocusedTextScrollForwarding = !multiline
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
                is NativeVideoPlayerContainer -> {
                    when (method) {
                        "play" -> container.play()
                        "pause" -> container.pause()
                        "stop" -> container.stop()
                        "seekTo" -> {
                            val positionMs = (args["positionMs"] as? Number)?.toLong() ?: 0L
                            container.seekTo(positionMs)
                        }
                        "setVolume" -> {
                            val volume = (args["volume"] as? Number)?.toFloat() ?: 1.0f
                            container.setVolume(volume)
                        }
                        "setLooping" -> {
                            val looping = args["looping"] as? Boolean ?: false
                            container.setLooping(looping)
                        }
                        "setPlaybackSpeed" -> {
                            val rate = (args["rate"] as? Number)?.toFloat() ?: 1.0f
                            container.setPlaybackSpeed(rate)
                        }
                        "load" -> {
                            val url = args["url"] as? String
                            if (url != null) {
                                container.load(url)
                            }
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
            "video_player" -> { { NativeVideoPlayerContainer(ctx, viewId, params) } }
            else -> null
        }

        if (creator == null) {
            return Pair(null, IllegalArgumentException("Unknown view type: $viewType"))
        }

        // Add to host view on main thread, wrapped in a TouchInterceptorView
        host.post {
            val container = creator()
            views[viewId] = container

            val interceptor = TouchInterceptorView(ctx, viewId)
            if (viewType == "textinput") {
                val multiline = params["multiline"] as? Boolean ?: false
                interceptor.enableUnfocusedTextScrollForwarding = !multiline
            }
            interceptor.surfaceView = surfaceView
            interceptor.addView(container.view, FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT))
            interceptor.visibility = View.GONE // Hidden until positioned
            interceptors[viewId] = interceptor
            (host as? InputOverlayLayout)?.addOverlayView(viewId, interceptor) ?: host.addView(interceptor)

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
            overlayController?.removeView(viewId.toLong())
            val interceptor = interceptors.remove(viewId)
            if (interceptor != null) {
                (host as? InputOverlayLayout)?.removeOverlayView(viewId) ?: host.removeView(interceptor)
            } else {
                host.removeView(container.view)
            }
        }

        return Pair(null, null)
    }

    private fun setVisible(args: Map<*, *>): Pair<Any?, Exception?> {
        val viewId = (args["viewId"] as? Number)?.toInt() ?: return Pair(null, null)
        val visible = args["visible"] as? Boolean ?: true
        val host = hostView ?: return Pair(null, null)

        host.post {
            val container = views[viewId] ?: return@post
            val targetView = interceptors[viewId] ?: container.view
            targetView.visibility = if (visible) View.VISIBLE else View.GONE
        }

        return Pair(null, null)
    }

    private fun setEnabled(args: Map<*, *>): Pair<Any?, Exception?> {
        val viewId = (args["viewId"] as? Number)?.toInt() ?: return Pair(null, null)
        val enabled = args["enabled"] as? Boolean ?: true
        val host = hostView ?: return Pair(null, null)

        host.post {
            val container = views[viewId] ?: return@post
            // Apply enabled/alpha to the inner view, not the interceptor wrapper
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
                val code = if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.M) {
                    webViewErrorCodeString(error?.errorCode ?: 0)
                } else {
                    "load_failed"
                }
                val message = if (android.os.Build.VERSION.SDK_INT >= android.os.Build.VERSION_CODES.M) {
                    error?.description?.toString() ?: "Unknown error"
                } else {
                    "Unknown error"
                }
                PlatformChannelManager.sendEvent(
                    "drift/platform_views",
                    mapOf(
                        "method" to "onWebViewError",
                        "viewId" to viewId,
                        "code" to code,
                        "message" to message
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

private fun webViewErrorCodeString(errorCode: Int): String = when (errorCode) {
    android.webkit.WebViewClient.ERROR_HOST_LOOKUP,
    android.webkit.WebViewClient.ERROR_CONNECT,
    android.webkit.WebViewClient.ERROR_IO,
    android.webkit.WebViewClient.ERROR_TIMEOUT,
    android.webkit.WebViewClient.ERROR_REDIRECT_LOOP,
    android.webkit.WebViewClient.ERROR_TOO_MANY_REQUESTS -> "network_error"
    android.webkit.WebViewClient.ERROR_FAILED_SSL_HANDSHAKE -> "ssl_error"
    else -> "load_failed"
}
