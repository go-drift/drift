/**
 * PlatformChannel.kt
 * Provides platform channel communication between Kotlin and the Go Drift engine.
 *
 * This file implements the native side of platform channels, enabling Go code
 * to call Android APIs (clipboard, haptics, etc.) and receive events from Android.
 */
package {{.PackageName}}

import android.app.Activity
import android.app.Application
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.res.Configuration
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.VibrationEffect
import android.os.Vibrator
import android.os.VibratorManager
import android.util.Log
import android.view.HapticFeedbackConstants
import android.view.View
import androidx.core.content.FileProvider
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import com.drift.runner.DriftMethodHandler
import com.drift.runner.DriftPluginHost
import com.drift.runner.DriftResult
import com.drift.runner.DriftSubscription
import java.io.File
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CountDownLatch
import org.json.JSONArray
import org.json.JSONObject
import org.json.JSONTokener

/**
 * Handler for a built-in channel: synchronous, run on the calling (Go)
 * thread. Plugins use the asynchronous com.drift.runner.DriftMethodHandler
 * instead.
 */
fun interface MethodHandler {
    operator fun invoke(method: String, args: Any?): Pair<Any?, Exception?>
}

/**
 * Manages platform channel handlers and dispatches calls between Go and Android.
 *
 * Adopts com.drift.runner.DriftPluginHost so third-party plugins can register
 * channels and send events without depending on the user's app package.
 * Plugins register through com.drift.runner.DriftPlugins, once per process.
 */
object PlatformChannelManager : DriftPluginHost {
    override lateinit var context: Context
    private var view: View? = null
    private var currentActivity: Activity? = null
    /**
     * A registered channel. Every handler takes the asynchronous form;
     * built-ins are adapted. Plugin handlers run on the main thread,
     * built-ins on the calling thread.
     */
    private class Channel(val handler: DriftMethodHandler, val onMain: Boolean)

    // Written on the main thread during registration, read from Go threads.
    private val channels = ConcurrentHashMap<String, Channel>()
    private val codec = JsonCodec
    private val mainHandler = Handler(Looper.getMainLooper())

    // The JNI bridge reads a failed call's error right after the call, on
    // the same thread.
    private val lastError = ThreadLocal<String?>()
    @Volatile
    private var onFrameNeeded: (() -> Unit)? = null

    @Volatile
    private var initialized = false

    /**
     * Initializes the platform channel manager with the application context
     * and registers Drift's built-in channels, once per process.
     *
     * MainActivity.onCreate calls this on every Activity creation
     * (configuration changes, process-death restoration); the object
     * outlives Activities, so later calls are no-ops. Registration never
     * repeats, and the Application-scoped lifecycle observer is installed
     * exactly once.
     */
    fun init(context: Context) {
        if (initialized) return
        this.context = context.applicationContext
        registerBuiltInChannels()
        setupLifecycleObserver()
        initialized = true
    }

    /**
     * Sets the view to use for haptic feedback.
     */
    fun setView(view: View?) {
        this.view = view
    }

    /**
     * Sets a callback invoked after sending events to Go, so the
     * rendering surface can schedule a new frame for the state change.
     */
    fun setOnFrameNeeded(callback: () -> Unit) {
        onFrameNeeded = callback
    }

    fun currentActivity(): Activity? {
        return currentActivity
    }

    fun isAppForeground(): Boolean {
        return currentActivity != null
    }

    /**
     * Registers a handler for a platform channel.
     */
    /** Registers a built-in channel handler. */
    fun register(channel: String, handler: MethodHandler) {
        add(channel, Channel({ method, args, result ->
            val (value, error) = handler(method, args)
            if (error != null) result.error(error) else result.success(value)
        }, onMain = false))
    }

    /** DriftPluginHost: register a channel handler from a plugin source. */
    override fun registerChannel(name: String, handler: DriftMethodHandler) {
        add(name, Channel(handler, onMain = true))
    }

    private fun add(name: String, channel: Channel) {
        if (channels.putIfAbsent(name, channel) != null) {
            throw IllegalStateException(
                "drift: platform channel \"$name\" is already registered. " +
                    "Built-in channels run first; plugins must choose a unique " +
                    "<vendor>/<feature> namespace."
            )
        }
    }

    /**
     * JNI entry point for Go->Kotlin method calls.
     * Called by native code when Go invokes a platform channel method.
     * Returns JSON-encoded result or null on error.
     */
    @JvmStatic
    fun handleMethodCallNative(channel: String, method: String, argsData: ByteArray?): ByteArray? {
        lastError.set(null)
        val (result, error) = handleMethodCall(channel, method, argsData)
        if (error != null) {
            lastError.set(error)
            // Log error but return null - Go will handle it
            android.util.Log.e("PlatformChannel", "Error handling $channel.$method: $error")
            return null
        }
        return result
    }

    @JvmStatic
    fun consumeLastError(): String? {
        val error = lastError.get()
        lastError.set(null)
        return error
    }

    /**
     * Called from native (JNI) when the Go engine needs a new frame.
     * Invokes the onFrameNeeded callback which posts a one-shot Choreographer frame.
     */
    @JvmStatic
    fun nativeScheduleFrame() {
        onFrameNeeded?.invoke()
    }

    /**
     * Handles a method call from Go and returns the result. Go's call is
     * synchronous, so this blocks the calling thread until the handler
     * replies. A plugin handler runs on the main thread; when Go itself
     * calls from the main thread (inside a frame or a widget callback), the
     * handler must reply before returning, since waiting there would
     * deadlock any reply that needs the main thread.
     */
    fun handleMethodCall(channel: String, method: String, argsData: ByteArray?): Pair<ByteArray?, String?> {
        val registered = channels[channel]
            ?: return Pair(null, errorPayload("channel_not_found", "Channel not found: $channel"))

        val args = if (argsData != null && argsData.isNotEmpty()) {
            JsonCodec.decode(argsData)
        } else {
            null
        }

        // The latch orders the writes of value/error before their reads below.
        val replied = CountDownLatch(1)
        var value: Any? = null
        var error: Exception? = null
        val result = DriftResult("$channel.$method") { v, e ->
            value = v
            error = e
            replied.countDown()
        }
        val invoke = {
            try {
                registered.handler.onMethodCall(method, args, result)
            } catch (e: Exception) {
                result.errorIfPending(e)
            }
        }
        val onMainThread = Looper.myLooper() == Looper.getMainLooper()
        if (registered.onMain && !onMainThread) {
            mainHandler.post(invoke)
        } else {
            invoke()
            if (replied.count > 0 && onMainThread) {
                crash(
                    "drift: $channel.$method did not reply before returning, but Go called it " +
                        "on the main thread, where waiting would deadlock. Call it from a goroutine."
                )
            }
        }
        replied.await()

        error?.let { e ->
            val code = if (e is IllegalArgumentException) "invalid_arguments" else "native_error"
            val details = mapOf("exception" to e.javaClass.name)
            return Pair(null, errorPayload(code, e.message ?: "Unknown error", details))
        }
        return Pair(codec.encode(value), null)
    }

    /**
     * Fails fast on a programming error. A plain throw would only reach the
     * JNI bridge, which turns exceptions into a generic Go error.
     */
    private fun crash(message: String): Nothing {
        val e = IllegalStateException(message)
        Thread.getDefaultUncaughtExceptionHandler()?.uncaughtException(Thread.currentThread(), e)
        throw e
    }

    /**
     * Sends an event to Go listeners. Also fans out to any native-side
     * observers registered via `observeEvent(channel, handler)`, so plugins
     * can react to events originating from other native modules (e.g. the
     * splash plugin reacting to `drift/rendering/frame_events` posted by
     * `SkiaHostView`).
     *
     * After dispatching, wakes the frame loop so the engine renders the
     * state change.
     */
    override fun sendEvent(channel: String, data: Any?) {
        notifyEventObservers(channel, data)
        val encoded = codec.encode(data)
        NativeBridge.platformHandleEvent(channel, encoded, encoded.size)
        onFrameNeeded?.invoke()
    }

    private val eventObservers = mutableMapOf<String, MutableMap<Long, (Any?) -> Unit>>()
    private val eventObserversLock = Any()
    @Volatile private var nextObserverId = 1L

    /**
     * Implements [DriftPluginHost.observeEvent]. Returns a
     * [DriftSubscription] token; the token's `cancel()` unsubscribes.
     * Multiple observers per channel are supported.
     *
     * Observers run asynchronously on the main thread, in the order events
     * were sent.
     */
    override fun observeEvent(channel: String, handler: (Any?) -> Unit): DriftSubscription {
        val id = synchronized(eventObserversLock) {
            val id = nextObserverId++
            val perChannel = eventObservers.getOrPut(channel) { mutableMapOf() }
            perChannel[id] = handler
            id
        }
        return DriftSubscription {
            synchronized(eventObserversLock) {
                eventObservers[channel]?.remove(id)
                if (eventObservers[channel]?.isEmpty() == true) {
                    eventObservers.remove(channel)
                }
            }
        }
    }

    /**
     * Delivers data to the channel's observers on the main thread, skipping
     * any cancelled before delivery.
     */
    private fun notifyEventObservers(channel: String, data: Any?) {
        val ids = synchronized(eventObserversLock) {
            eventObservers[channel]?.keys?.toList()
        } ?: return
        if (ids.isEmpty()) return
        mainHandler.post {
            for (id in ids) {
                val handler = synchronized(eventObserversLock) { eventObservers[channel]?.get(id) } ?: continue
                handler(data)
            }
        }
    }

    /**
     * Sends an error to Go event listeners.
     */
    override fun sendEventError(channel: String, code: String, message: String) {
        NativeBridge.platformHandleEventError(channel, code, message)
    }

    /**
     * Notifies Go that an event stream has ended.
     */
    override fun sendEventDone(channel: String) {
        NativeBridge.platformHandleEventDone(channel)
    }

    private fun errorPayload(code: String, message: String, details: Map<String, Any?>? = null): String {
        val payload = mutableMapOf<String, Any?>("code" to code, "message" to message)
        if (details != null && details.isNotEmpty()) {
            payload["details"] = details
        }
        return String(codec.encode(payload), Charsets.UTF_8)
    }

    private fun registerBuiltInChannels() {
        // Clipboard channel
        register("drift/clipboard") { method, args ->
            ClipboardHandler.handle(context, method, args)
        }

        // Haptics channel
        register("drift/haptics") { method, args ->
            HapticsHandler.handle(context, view, method, args)
        }

        // Share channel
        register("drift/share") { method, args ->
            ShareHandler.handle(context, method, args)
        }

        // Lifecycle channel
        register("drift/lifecycle") { method, args ->
            LifecycleHandler.handle(method, args)
        }

        // System UI channel
        register("drift/system_ui") { method, args ->
            SystemUIHandler.handle(method, args)
        }

        // Notifications channel
        register("drift/notifications") { method, args ->
            NotificationHandler.handle(context, method, args)
        }

        // Deep links channel
        register("drift/deeplinks") { method, args ->
            DeepLinkHandler.handle(method, args)
        }

        // Platform Views channel
        register("drift/platform_views") { method, args ->
            PlatformViewHandler.handle(method, args)
        }

        // Permissions channel
        register("drift/permissions") { method, args ->
            PermissionHandler.handle(context, method, args)
        }

        // Location channel
        register("drift/location") { method, args ->
            LocationHandler.handle(context, method, args)
        }

        // Storage channel
        register("drift/storage") { method, args ->
            StorageHandler.handle(context, method, args)
        }

        // Camera channel
        register("drift/camera") { method, args ->
            CameraHandler.handle(context, method, args)
        }

        // Background tasks channel
        register("drift/background") { method, args ->
            BackgroundHandler.handle(context, method, args)
        }

        // Accessibility channel
        register("drift/accessibility") { method, args ->
            AccessibilityHandler.handle(context, method, args)
        }

        // Preferences channel
        register("drift/preferences") { method, args ->
            PreferencesHandler.handle(context, method, args)
        }

        // Secure Storage channel
        register("drift/secure_storage") { method, args ->
            SecureStorageHandler.handle(context, method, args)
        }

        // Date Picker channel
        register("drift/date_picker") { method, args ->
            DatePickerHandler.handle(method, args)
        }

        // Time Picker channel
        register("drift/time_picker") { method, args ->
            TimePickerHandler.handle(method, args)
        }

        // Audio Player channel
        register("drift/audio_player") { method, args ->
            AudioPlayerHandler.handle(context, method, args)
        }

        // URL Launcher channel
        register("drift/url_launcher") { method, args ->
            URLLauncherHandler.handle(context, method, args)
        }
    }

    private fun setupLifecycleObserver() {
        val app = context.applicationContext as Application
        app.registerActivityLifecycleCallbacks(object : Application.ActivityLifecycleCallbacks {
            override fun onActivityResumed(activity: Activity) {
                currentActivity = activity
                sendEvent("drift/lifecycle/events", mapOf("state" to "resumed"))
                LifecycleHandler.updateState("resumed")
            }

            override fun onActivityPaused(activity: Activity) {
                if (currentActivity === activity) {
                    currentActivity = null
                }
                sendEvent("drift/lifecycle/events", mapOf("state" to "inactive"))
                LifecycleHandler.updateState("inactive")
            }

            override fun onActivityStopped(activity: Activity) {
                sendEvent("drift/lifecycle/events", mapOf("state" to "paused"))
                LifecycleHandler.updateState("paused")
            }

            override fun onActivityCreated(activity: Activity, savedInstanceState: Bundle?) {}
            override fun onActivityStarted(activity: Activity) {}
            override fun onActivitySaveInstanceState(activity: Activity, outState: Bundle) {}
            override fun onActivityDestroyed(activity: Activity) {}
        })
    }
}

// MARK: - Clipboard Handler

object ClipboardHandler {
    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        val clipboard = context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager

        return when (method) {
            "getText" -> {
                val text = clipboard.primaryClip?.getItemAt(0)?.text?.toString() ?: ""
                Pair(mapOf("text" to text), null)
            }

            "setText" -> {
                val argsMap = args as? Map<*, *>
                val text = argsMap?.get("text") as? String
                    ?: return Pair(null, IllegalArgumentException("Missing text argument"))

                val clip = ClipData.newPlainText("text", text)
                clipboard.setPrimaryClip(clip)
                Pair(null, null)
            }

            "hasText" -> {
                Pair(clipboard.hasPrimaryClip() && clipboard.primaryClip?.getItemAt(0)?.text != null, null)
            }

            "clear" -> {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                    clipboard.clearPrimaryClip()
                } else {
                    clipboard.setPrimaryClip(ClipData.newPlainText("", ""))
                }
                Pair(null, null)
            }

            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }
}

// MARK: - Haptics Handler

object HapticsHandler {
    fun handle(context: Context, view: View?, method: String, args: Any?): Pair<Any?, Exception?> {
        return when (method) {
            "impact" -> {
                val argsMap = args as? Map<*, *>
                val style = argsMap?.get("style") as? String
                    ?: return Pair(null, IllegalArgumentException("Missing style argument"))

                performHaptic(context, view, style)
                Pair(null, null)
            }

            "vibrate" -> {
                val argsMap = args as? Map<*, *>
                val duration = (argsMap?.get("duration") as? Number)?.toLong() ?: 100L

                vibrate(context, duration)
                Pair(null, null)
            }

            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun performHaptic(context: Context, view: View?, style: String) {
        // Try to use view's performHapticFeedback first (preferred)
        val feedbackConstant = when (style) {
            "light" -> HapticFeedbackConstants.KEYBOARD_TAP
            "medium" -> HapticFeedbackConstants.VIRTUAL_KEY
            "heavy" -> HapticFeedbackConstants.LONG_PRESS
            "selection" -> HapticFeedbackConstants.CLOCK_TICK
            "success" -> if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                HapticFeedbackConstants.CONFIRM
            } else {
                HapticFeedbackConstants.VIRTUAL_KEY
            }
            "warning" -> if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                HapticFeedbackConstants.REJECT
            } else {
                HapticFeedbackConstants.LONG_PRESS
            }
            "error" -> if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                HapticFeedbackConstants.REJECT
            } else {
                HapticFeedbackConstants.LONG_PRESS
            }
            else -> HapticFeedbackConstants.VIRTUAL_KEY
        }

        if (view?.performHapticFeedback(feedbackConstant) == true) {
            return
        }

        // Fallback to vibrator
        val duration = when (style) {
            "light" -> 10L
            "medium" -> 20L
            "heavy" -> 50L
            "selection" -> 5L
            else -> 20L
        }
        vibrate(context, duration)
    }

    private fun vibrate(context: Context, durationMs: Long) {
        val vibrator = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            val vibratorManager = context.getSystemService(Context.VIBRATOR_MANAGER_SERVICE) as VibratorManager
            vibratorManager.defaultVibrator
        } else {
            @Suppress("DEPRECATION")
            context.getSystemService(Context.VIBRATOR_SERVICE) as Vibrator
        }

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            vibrator.vibrate(VibrationEffect.createOneShot(durationMs, VibrationEffect.DEFAULT_AMPLITUDE))
        } else {
            @Suppress("DEPRECATION")
            vibrator.vibrate(durationMs)
        }
    }
}

// MARK: - Share Handler

object ShareHandler {
    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        if (method != "share") {
            return Pair(null, IllegalArgumentException("Unknown method: $method"))
        }

        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))

        val intent = Intent(Intent.ACTION_SEND).apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }

        // Handle text
        val text = argsMap["text"] as? String
        val subject = argsMap["subject"] as? String
        val url = argsMap["url"] as? String

        if (subject != null) {
            intent.putExtra(Intent.EXTRA_SUBJECT, subject)
        }

        // Combine text and URL if both present
        val combinedText = when {
            text != null && url != null -> "$text\n$url"
            text != null -> text
            url != null -> url
            else -> null
        }

        if (combinedText != null) {
            intent.type = "text/plain"
            intent.putExtra(Intent.EXTRA_TEXT, combinedText)
        }

        // Handle single file
        val filePath = argsMap["file"] as? String
        val mimeType = argsMap["mimeType"] as? String ?: "*/*"

        if (filePath != null) {
            val file = File(filePath)
            val uri = FileProvider.getUriForFile(
                context,
                "${context.packageName}.fileprovider",
                file
            )
            intent.type = mimeType
            intent.putExtra(Intent.EXTRA_STREAM, uri)
            intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }

        // Handle multiple files
        @Suppress("UNCHECKED_CAST")
        val files = argsMap["files"] as? List<Map<String, Any>>
        if (files != null && files.isNotEmpty()) {
            val uris = ArrayList<android.net.Uri>()
            for (fileInfo in files) {
                val path = fileInfo["path"] as? String ?: continue
                val file = File(path)
                val uri = FileProvider.getUriForFile(
                    context,
                    "${context.packageName}.fileprovider",
                    file
                )
                uris.add(uri)
            }
            if (uris.isNotEmpty()) {
                intent.action = Intent.ACTION_SEND_MULTIPLE
                intent.type = files.firstOrNull()?.get("mimeType") as? String ?: "*/*"
                intent.putParcelableArrayListExtra(Intent.EXTRA_STREAM, uris)
                intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            }
        }

        // Create chooser and start activity
        val chooser = Intent.createChooser(intent, null).apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        context.startActivity(chooser)

        return Pair(mapOf("result" to "success"), null)
    }
}

// MARK: - Deep Link Handler

object DeepLinkHandler {
    private var initialLink: Map<String, Any>? = null
    private var lastLink: String? = null

    @Suppress("UNUSED_PARAMETER")
    fun handle(method: String, args: Any?): Pair<Any?, Exception?> {
        return when (method) {
            "getInitial" -> {
                val link = initialLink
                initialLink = null
                Pair(link, null)
            }
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    fun handleIntent(intent: Intent?, source: String) {
        val url = intent?.dataString ?: return
        if (url.isEmpty()) {
            return
        }
        val payload = mapOf(
            "url" to url,
            "source" to source,
            "timestamp" to System.currentTimeMillis()
        )
        if (initialLink == null) {
            initialLink = payload
        }
        if (lastLink == url) {
            return
        }
        lastLink = url
        Log.i("DriftDeepLink", "Received deep link: $url (source=$source)")
        PlatformChannelManager.sendEvent("drift/deeplinks/events", payload)
    }
}

// MARK: - Lifecycle Handler

object LifecycleHandler {
    private var currentState = "resumed"

    @Suppress("UNUSED_PARAMETER")
    fun handle(method: String, args: Any?): Pair<Any?, Exception?> {
        return when (method) {
            "getState" -> Pair(mapOf("state" to currentState), null)
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    fun updateState(state: String) {
        currentState = state
    }
}

// MARK: - System UI Handler

/**
 * Status bar visibility and icon colour. The window is always edge to edge
 * (MainActivity.onCreate; Android 15+ enforces it): Drift draws behind the
 * system bars and apps inset content with SafeArea, as on iOS.
 *
 * The style is state, not a command: MainActivity reapplies it to a
 * recreated Activity (dark mode, locale), which Go does not hear about.
 */
object SystemUIHandler {
    private data class Style(val statusBarHidden: Boolean, val statusBarStyle: String)

    @Volatile
    private var current = Style(statusBarHidden = false, statusBarStyle = "default")

    fun handle(method: String, args: Any?): Pair<Any?, Exception?> {
        if (method != "setStyle") {
            return Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        current = Style(
            statusBarHidden = argsMap["statusBarHidden"] as? Boolean ?: false,
            statusBarStyle = argsMap["statusBarStyle"] as? String ?: "default",
        )
        PlatformChannelManager.currentActivity()?.let { activity ->
            activity.runOnUiThread { apply(activity) }
        }
        return Pair(null, null)
    }

    /** Applies the current style to activity's window. Main thread. */
    fun apply(activity: Activity) {
        val style = current
        val window = activity.window
        val controller = WindowInsetsControllerCompat(window, window.decorView)
        if (style.statusBarHidden) {
            controller.hide(WindowInsetsCompat.Type.statusBars())
        } else {
            controller.show(WindowInsetsCompat.Type.statusBars())
        }
        // Light status bar = dark icons. "default" follows the system
        // theme, like iOS's .default.
        controller.isAppearanceLightStatusBars = when (style.statusBarStyle) {
            "dark" -> true
            "light" -> false
            else -> (activity.resources.configuration.uiMode and Configuration.UI_MODE_NIGHT_MASK) !=
                Configuration.UI_MODE_NIGHT_YES
        }
    }
}

// MARK: - Safe Area Handler

object SafeAreaHandler {
    fun sendInsetsUpdate() {
        val activity = PlatformChannelManager.currentActivity() ?: return
        val rootView = activity.window.decorView
        val insets = ViewCompat.getRootWindowInsets(rootView)
            ?.getInsets(WindowInsetsCompat.Type.systemBars()) ?: return
        val density = activity.resources.displayMetrics.density
        PlatformChannelManager.sendEvent("drift/safe_area/events", mapOf(
            "top" to (insets.top / density).toDouble(),
            "bottom" to (insets.bottom / density).toDouble(),
            "left" to (insets.left / density).toDouble(),
            "right" to (insets.right / density).toDouble()
        ))
    }
}

// MARK: - URL Launcher Handler

object URLLauncherHandler {
    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        return when (method) {
            "openURL" -> {
                val argsMap = args as? Map<*, *>
                val url = argsMap?.get("url") as? String
                    ?: return Pair(null, IllegalArgumentException("Missing url argument"))
                try {
                    val intent = Intent(Intent.ACTION_VIEW, android.net.Uri.parse(url)).apply {
                        addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                    }
                    context.startActivity(intent)
                    Pair(null, null)
                } catch (e: android.content.ActivityNotFoundException) {
                    Pair(null, IllegalArgumentException("No application can handle this URL: $url"))
                }
            }

            "canOpenURL" -> {
                val argsMap = args as? Map<*, *>
                val url = argsMap?.get("url") as? String
                    ?: return Pair(null, IllegalArgumentException("Missing url argument"))
                val intent = Intent(Intent.ACTION_VIEW, android.net.Uri.parse(url))
                val canOpen = intent.resolveActivity(context.packageManager) != null
                Pair(mapOf("canOpen" to canOpen), null)
            }

            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }
}

// MARK: - JSON Implementation

/**
 * Simple JSON codec for basic types.
 */
object JsonCodec {
    fun encode(value: Any?): ByteArray {
        val jsonValue = toJson(value)
        val jsonString = when (jsonValue) {
            JSONObject.NULL -> "null"
            is JSONObject, is JSONArray -> jsonValue.toString()
            is String -> JSONObject.quote(jsonValue)
            is Number, is Boolean -> jsonValue.toString()
            else -> "null"
        }
        return jsonString.toByteArray(Charsets.UTF_8)
    }

    fun decode(data: ByteArray): Any? {
        if (data.isEmpty()) return null
        val jsonString = String(data, Charsets.UTF_8)
        val parsed = JSONTokener(jsonString).nextValue()
        return fromJson(parsed)
    }

    private fun toJson(value: Any?): Any? {
        return when (value) {
            null -> JSONObject.NULL
            is JSONObject, is JSONArray -> value
            is Boolean, is Number, is String -> value
            is Map<*, *> -> {
                val obj = JSONObject()
                for ((key, item) in value) {
                    if (key != null) {
                        obj.put(key.toString(), toJson(item))
                    }
                }
                obj
            }
            is Iterable<*> -> JSONArray().also { arr -> value.forEach { arr.put(toJson(it)) } }
            is Array<*> -> JSONArray().also { arr -> value.forEach { arr.put(toJson(it)) } }
            else -> JSONObject.NULL
        }
    }

    private fun fromJson(value: Any?): Any? {
        return when (value) {
            JSONObject.NULL -> null
            is JSONObject -> {
                val map = mutableMapOf<String, Any?>()
                for (key in value.keys()) {
                    map[key] = fromJson(value.get(key))
                }
                map
            }
            is JSONArray -> (0 until value.length()).map { fromJson(value.get(it)) }
            else -> value
        }
    }
}
