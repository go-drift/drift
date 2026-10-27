/**
 * PlatformView.kt
 * Provides platform view management for embedding native views in Drift UI.
 */
package {{.PackageName}}

import android.annotation.SuppressLint
import android.app.Activity
import android.content.Context
import android.content.MutableContextWrapper
import android.util.Log
import android.view.LayoutInflater
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
    /**
     * Each view's context: the hosting Activity while there is one, the
     * Application between Activities, so a view never holds an Activity
     * that is gone.
     */
    private val contexts = mutableMapOf<Int, PlatformViewContext>()
    private var activity: Activity? = null
    private var hostView: ViewGroup? = null
    private var surfaceView: View? = null
    private var overlayController: InputOverlayController? = null
    private val main = Handler(Looper.getMainLooper())
    /** Whether warmUp has run in this process. Main thread only. */
    private var warmedUp = false

    /**
     * Hosts platform views in [activity]'s [hostView]. Views outlive their
     * Activity (recreation for a configuration change, or a new Activity
     * after the last finished): those created earlier move into this one,
     * hidden until the next frame positions them. Main thread.
     */
    fun init(activity: Activity, hostView: ViewGroup, surfaceView: View, overlayController: InputOverlayController) {
        this.activity?.let { detachHost(it) }
        this.activity = activity
        this.hostView = hostView
        this.surfaceView = surfaceView
        this.overlayController = overlayController
        for ((viewId, interceptor) in interceptors) {
            contexts[viewId]?.baseContext = activity
            interceptor.surfaceView = surfaceView
            interceptor.visibility = View.GONE
            addToHost(viewId, interceptor)
            views[viewId]?.onHostChanged()
        }
    }

    /**
     * Takes the platform views out of [activity], which is being destroyed,
     * and points their contexts at the Application, so the Activity is not
     * kept alive; the next Activity's [init] takes them in. Main thread.
     */
    fun detachHost(activity: Activity) {
        if (this.activity !== activity) return
        for ((viewId, interceptor) in interceptors) {
            removeFromHost(viewId, interceptor)
            interceptor.surfaceView = null
            contexts[viewId]?.baseContext = activity.applicationContext
            views[viewId]?.onHostChanged()
        }
        this.activity = null
        hostView = null
        surfaceView = null
        overlayController = null
    }

    private fun addToHost(viewId: Int, interceptor: TouchInterceptorView) {
        val host = hostView ?: return
        (host as? InputOverlayLayout)?.addOverlayView(viewId, interceptor) ?: host.addView(interceptor)
    }

    private fun removeFromHost(viewId: Int, interceptor: TouchInterceptorView) {
        overlayController?.removeView(viewId.toLong())
        val host = hostView ?: return
        (host as? InputOverlayLayout)?.removeOverlayView(viewId) ?: host.removeView(interceptor)
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
     * Once per process: the SkiaHostView of every Activity calls it when
     * its engine is ready, and only the first call warms up, so a
     * recreated Activity (dark mode, locale) does not post the event again.
     *
     * Must be called on the main thread.
     */
    fun warmUp(context: Context) {
        if (warmedUp) return
        warmedUp = true
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

    /**
     * Registers the drift/platform_views channel. `create` runs the factory
     * on the main thread and replies with its outcome, so a factory's
     * exception reaches Go's Create. Every later operation on a view runs
     * asynchronously on the main thread, after earlier ones, and Go does not
     * wait for it.
     */
    fun start(host: DriftPluginHost) {
        host.registerChannel("drift/platform_views") {
            method("create") { args -> create(viewArgs(args)) }
            method("dispose") { args ->
                onView(viewArgs(args)) { viewId, container ->
                    views.remove(viewId)
                    viewTypes.remove(viewId)
                    contexts.remove(viewId)
                    container.dispose()
                    interceptors.remove(viewId)?.let { removeFromHost(viewId, it) }
                }
            }
            method("setVisible") { args ->
                val argsMap = viewArgs(args)
                val visible = argsMap["visible"] as? Boolean ?: true
                onView(argsMap) { viewId, container ->
                    (interceptors[viewId] ?: container.view).visibility = if (visible) View.VISIBLE else View.GONE
                }
            }
            method("setEnabled") { args ->
                val argsMap = viewArgs(args)
                val enabled = argsMap["enabled"] as? Boolean ?: true
                onView(argsMap) { _, container ->
                    // The inner view, not the interceptor wrapper.
                    container.view.isEnabled = enabled
                    container.view.alpha = if (enabled) 1.0f else 0.5f
                }
            }
            method("invokeViewMethod") { args ->
                val argsMap = viewArgs(args)
                val viewMethod = argsMap["method"] as? String
                    ?: throw IllegalArgumentException("Missing method")
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
        }
    }

    private fun viewArgs(args: Any?): Map<*, *> =
        args as? Map<*, *> ?: throw IllegalArgumentException("Invalid arguments")

    /**
     * Runs [body] on the main thread, after earlier calls, with the view
     * args' viewId names; a view already disposed is skipped (Go may call it
     * after disposing).
     */
    private fun onView(args: Map<*, *>, body: (Int, DriftPlatformView) -> Unit): Any? {
        val viewId = (args["viewId"] as? Number)?.toInt()
            ?: throw IllegalArgumentException("Missing viewId")
        main.post {
            val container = views[viewId] ?: return@post
            body(viewId, container)
        }
        return null
    }

    /**
     * Creates the view on the main thread, failing the call if the factory
     * throws, and puts it in the host (or the next host, if no Activity
     * hosts views now). Go is notified once the view is in the hierarchy,
     * so resendGeometry targets it.
     */
    private fun create(args: Map<*, *>): Any? {
        val viewId = (args["viewId"] as? Number)?.toInt()
            ?: throw IllegalArgumentException("Missing viewId")
        val viewType = args["viewType"] as? String
            ?: throw IllegalArgumentException("Missing viewType")
        @Suppress("UNCHECKED_CAST")
        val params = args["params"] as? Map<String, Any?> ?: emptyMap()
        val factory = factories[viewType] ?: throw DriftCallError(
            "view_type_not_found",
            "no native platform view type \"$viewType\"; add the plugin that provides it to drift.yaml",
        )

        val ctx = PlatformViewContext(activity ?: PlatformChannelManager.context)
        val events = DriftPlatformViewEvents(viewId) { event ->
            PlatformChannelManager.sendEvent("drift/platform_views", event)
        }
        val container = factory.create(ctx, viewId, params, events)
        views[viewId] = container
        viewTypes[viewId] = viewType
        contexts[viewId] = ctx

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
        addToHost(viewId, interceptor)

        events.send("onViewCreated")
        return null
    }
}

/**
 * A platform view's context, which Drift points at the hosting Activity
 * (MutableContextWrapper.setBaseContext). Its LayoutInflater is its own,
 * cloned from the Application's: the Activity's would give the views a
 * factory inflates (a library's layout) the Activity itself as their
 * context, and holds the Activity as its private factory, so they would
 * keep a destroyed Activity alive instead of following this context.
 */
private class PlatformViewContext(base: Context) : MutableContextWrapper(base) {
    private val inflater by lazy { LayoutInflater.from(applicationContext).cloneInContext(this) }

    override fun getSystemService(name: String): Any? =
        if (name == Context.LAYOUT_INFLATER_SERVICE) inflater else super.getSystemService(name)
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
                // Called for every resource: only the page's own failure
                // is a load failure.
                if (request?.isForMainFrame != true) return
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
