/**
 * PlatformView.kt
 * Provides platform view management for embedding native views in Drift UI.
 */
package {{.PackageName}}

import android.annotation.SuppressLint
import android.content.Context
import android.util.Log
import android.view.View
import android.view.ViewGroup
import android.webkit.WebView
import android.webkit.WebViewClient
import android.widget.EditText
import android.widget.FrameLayout
import android.os.Handler
import android.os.Looper
import com.drift.runner.DRIFT_PLATFORM_VIEW_WARM_UP_EVENT
import com.drift.runner.DriftCallError
import com.drift.runner.DriftPlatformView
import com.drift.runner.DriftPlatformViewEvents
import com.drift.runner.DriftPlatformViewFactory
import com.drift.runner.DriftPluginHost
import com.drift.runner.driftCrash
import java.util.concurrent.ConcurrentHashMap

/**
 * Handles platform view channel methods from Go.
 */
object PlatformViewHandler {
    /** View factories by type, registered at launch (Drift's and plugins'). */
    private val factories = ConcurrentHashMap<String, DriftPlatformViewFactory>()

    // Main thread only: every operation on a view runs there, in the order
    // Go made the calls.
    private val views = mutableMapOf<Int, DriftPlatformView>()
    private val viewTypes = mutableMapOf<Int, String>()
    private val interceptors = mutableMapOf<Int, TouchInterceptorView>()
    private var context: Context? = null
    private var hostView: ViewGroup? = null
    private var surfaceView: View? = null
    private var overlayController: InputOverlayController? = null
    private val main = Handler(Looper.getMainLooper())

    fun init(context: Context, hostView: ViewGroup, surfaceView: View, overlayController: InputOverlayController) {
        this.context = context
        this.hostView = hostView
        this.surfaceView = surfaceView
        this.overlayController = overlayController
    }

    /** Registers the view type [type]; a second registration crashes. */
    fun register(type: String, factory: DriftPlatformViewFactory) {
        if (factories.putIfAbsent(type, factory) != null) {
            driftCrash("drift: platform view type \"$type\" is already registered")
        }
    }

    /** Registers Drift's own view types. */
    fun registerBuiltIns(host: DriftPluginHost) {
        host.registerPlatformView("native_webview") { ctx, id, params, _ -> NativeWebViewContainer(ctx, id, params) }
        host.registerPlatformView("textinput") { ctx, id, params, _ -> NativeTextInputContainer(ctx, id, params) }
        host.registerPlatformView("switch") { ctx, id, params, _ -> NativeSwitchContainer(ctx, id, params) }
        host.registerPlatformView("activity_indicator") { ctx, id, params, _ -> NativeActivityIndicatorContainer(ctx, id, params) }
    }

    /**
     * Pre-warms expensive platform view classes by creating and immediately
     * destroying throwaway instances. This forces the classloader to load
     * Chromium WebView early so the cost is absorbed before the user
     * navigates to pages using it. Plugins warm their own views on
     * DRIFT_PLATFORM_VIEW_WARM_UP_EVENT.
     *
     * Must be called on the main thread.
     */
    fun warmUp(context: Context) {
        try {
            WebView(context).destroy()
        } catch (e: Exception) {
            Log.w("DriftWarmUp", "WebView warmup failed: ${e.message}")
        }
        try {
            EditText(context)
        } catch (e: Exception) {
            Log.w("DriftWarmUp", "EditText warmup failed: ${e.message}")
        }
        PlatformChannelManager.notifyEventObservers(DRIFT_PLATFORM_VIEW_WARM_UP_EVENT, null)
        Log.i("DriftWarmUp", "Platform view warmup complete")
    }

    fun handle(method: String, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))

        return when (method) {
            "create" -> create(argsMap)
            "dispose" -> onView(argsMap) { viewId, container ->
                views.remove(viewId)
                viewTypes.remove(viewId)
                container.dispose()
                overlayController?.removeView(viewId.toLong())
                val host = hostView
                val interceptor = interceptors.remove(viewId)
                if (interceptor != null) {
                    (host as? InputOverlayLayout)?.removeOverlayView(viewId) ?: host?.removeView(interceptor)
                } else {
                    host?.removeView(container.view)
                }
            }
            "setVisible" -> {
                val visible = argsMap["visible"] as? Boolean ?: true
                onView(argsMap) { viewId, container ->
                    (interceptors[viewId] ?: container.view).visibility = if (visible) View.VISIBLE else View.GONE
                }
            }
            "setEnabled" -> {
                val enabled = argsMap["enabled"] as? Boolean ?: true
                onView(argsMap) { _, container ->
                    // The inner view, not the interceptor wrapper.
                    container.view.isEnabled = enabled
                    container.view.alpha = if (enabled) 1.0f else 0.5f
                }
            }
            "invokeViewMethod" -> {
                val viewMethod = argsMap["method"] as? String
                    ?: return Pair(null, IllegalArgumentException("Missing method"))
                @Suppress("UNCHECKED_CAST")
                val viewArgs = argsMap as Map<String, Any?>
                onView(argsMap) { viewId, container ->
                    try {
                        container.handle(viewMethod, viewArgs)
                    } catch (e: Exception) {
                        Log.e("DriftPlatformView", "platform view $viewId (${viewTypes[viewId]}): $viewMethod: ${e.message}")
                    }
                    if (viewTypes[viewId] == "textinput" && viewMethod == "updateConfig") {
                        val multiline = viewArgs["multiline"] as? Boolean ?: false
                        interceptors[viewId]?.enableUnfocusedTextScrollForwarding = !multiline
                    }
                }
            }
            else -> Pair(null, DriftCallError("method_not_found", "Unknown method: $method"))
        }
    }

    /**
     * Runs [body] on the main thread, after earlier calls, with the view
     * args' viewId names; a view already disposed is skipped (Go may call it
     * after disposing).
     */
    private fun onView(args: Map<*, *>, body: (Int, DriftPlatformView) -> Unit): Pair<Any?, Exception?> {
        val viewId = (args["viewId"] as? Number)?.toInt()
            ?: return Pair(null, IllegalArgumentException("Missing viewId"))
        main.post {
            val container = views[viewId] ?: return@post
            body(viewId, container)
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
        val factory = factories[viewType] ?: return Pair(null, DriftCallError(
            "view_type_not_found",
            "no native platform view type \"$viewType\"; add the plugin that provides it to drift.yaml",
        ))

        // Created on the main thread, after earlier calls. Go is notified
        // once the view is in the hierarchy, so resendGeometry targets it.
        main.post {
            val ctx = context
            val host = hostView
            if (ctx == null || host == null) {
                Log.e("DriftPlatformView", "platform view $viewId created before the host view")
                return@post
            }
            val events = DriftPlatformViewEvents(viewId) { event ->
                PlatformChannelManager.sendEvent("drift/platform_views", event)
            }
            val container = factory.create(ctx, viewId, params, events)
            views[viewId] = container
            viewTypes[viewId] = viewType

            val interceptor = TouchInterceptorView(ctx, viewId)
            interceptor.supportsRegionMask = container.supportsRegionMask
            if (viewType == "textinput") {
                val multiline = params["multiline"] as? Boolean ?: false
                interceptor.enableUnfocusedTextScrollForwarding = !multiline
            }
            interceptor.surfaceView = surfaceView
            interceptor.addView(container.view, FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT))
            interceptor.visibility = View.GONE // Hidden until positioned
            interceptors[viewId] = interceptor
            (host as? InputOverlayLayout)?.addOverlayView(viewId, interceptor) ?: host.addView(interceptor)

            events.send("onViewCreated")
        }

        return Pair(mapOf("created" to true), null)
    }
}

/** The exception a view's handle throws for a method it does not know. */
internal fun unknownViewMethod(method: String) = DriftCallError("method_not_found", "unknown view method $method")

/**
 * Native web view container.
 */
@SuppressLint("SetJavaScriptEnabled")
class NativeWebViewContainer(
    context: Context,
    val viewId: Int,
    params: Map<String, Any?>
) : DriftPlatformView {

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
                val code = webViewErrorCodeString(error?.errorCode ?: 0)
                val message = error?.description?.toString() ?: "Unknown error"
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

    override fun handle(method: String, args: Map<String, Any?>) {
        when (method) {
            "load" -> (args["url"] as? String)?.let { view.loadUrl(it) }
            "goBack" -> view.goBack()
            "goForward" -> view.goForward()
            "reload" -> view.reload()
            else -> throw unknownViewMethod(method)
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
