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
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.VibrationEffect
import android.os.VibratorManager
import android.util.Log
import android.view.HapticFeedbackConstants
import android.view.View
import androidx.core.content.FileProvider
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import com.drift.runner.DriftCallError
import com.drift.runner.DriftChannel
import com.drift.runner.DriftPluginHost
import com.drift.runner.DriftResult
import com.drift.runner.DriftSubscription
import java.io.File
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicBoolean
import org.json.JSONArray
import org.json.JSONObject
import org.json.JSONTokener

/**
 * Handler for a built-in channel: synchronous, run on the calling (Go)
 * thread. Plugins declare methods on a com.drift.runner.DriftChannel
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
     * A registered channel: a built-in's handler runs on the calling (Go)
     * thread; a plugin's declared methods run on the main thread.
     */
    private sealed interface Channel {
        class BuiltIn(val handler: MethodHandler) : Channel
        class Plugin(val channel: DriftChannel) : Channel
    }

    // Written on the main thread during registration, read from Go threads.
    private val channels = ConcurrentHashMap<String, Channel>()
    private val codec = JsonCodec
    private val mainHandler = Handler(Looper.getMainLooper())

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

    /** Registers a built-in channel handler. */
    fun register(channel: String, handler: MethodHandler) {
        add(channel, Channel.BuiltIn(handler))
    }

    /** DriftPluginHost: register a channel and its methods from a plugin source. */
    override fun registerChannel(name: String, declare: DriftChannel.() -> Unit) {
        val channel = DriftChannel(name)
        channel.declare()
        channel.seal()
        add(name, Channel.Plugin(channel))
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
     * JNI entry point for Go->Kotlin method calls, on the thread Go called
     * from. Replies exactly once through NativeBridge.platformReply: before
     * returning when the method replies in place, otherwise later from any
     * thread. Never throws.
     */
    @JvmStatic
    fun handleMethodCallNative(callId: Long, channel: String, method: String, argsData: ByteArray?) {
        // Guards the catch below, which cannot tell whether the throw came
        // before or after the reply. Plugin double replies never get here:
        // DriftResult crashes on them.
        val replied = AtomicBoolean(false)
        val reply = { value: Any?, error: Exception? ->
            if (replied.compareAndSet(false, true)) {
                if (error != null) {
                    Log.e("PlatformChannel", "Error handling $channel.$method: $error")
                    NativeBridge.platformReply(callId, null, errorPayload(error))
                } else {
                    NativeBridge.platformReply(callId, codec.encode(value), null)
                }
            }
        }
        try {
            handleMethodCall(channel, method, argsData, reply)
        } catch (e: Exception) {
            // A built-in handler or argument decoding threw before replying.
            reply(null, e)
        }
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
     * Dispatches a method call from Go, on the calling thread, and replies
     * through [reply] exactly once.
     *
     * A built-in runs in place. A plugin method runs on the main thread: in
     * place when Go called from it, otherwise posted there. Go never waits
     * on the main thread: a call made there is answered before this returns,
     * so an asynchronous plugin method called there fails at once with
     * blocks_ui_thread instead of running.
     */
    private fun handleMethodCall(
        channel: String,
        method: String,
        argsData: ByteArray?,
        reply: (Any?, Exception?) -> Unit,
    ) {
        val registered = channels[channel]
            ?: return reply(null, DriftCallError("channel_not_found", "Channel not found: $channel"))

        val args = if (argsData != null && argsData.isNotEmpty()) {
            JsonCodec.decode(argsData)
        } else {
            null
        }

        when (registered) {
            is Channel.BuiltIn -> {
                val (value, error) = registered.handler(method, args)
                reply(value, error)
            }
            is Channel.Plugin -> {
                val onMainThread = Looper.myLooper() == Looper.getMainLooper()
                when (val m = registered.channel.lookup(method)) {
                    null -> reply(null, DriftCallError("method_not_found", "$channel has no method $method"))
                    is DriftChannel.Method.Sync -> {
                        fun run() {
                            val value = try {
                                m.handler(args)
                            } catch (e: Exception) {
                                return reply(null, e)
                            }
                            reply(value, null)
                        }
                        if (onMainThread) run() else mainHandler.post { run() }
                    }
                    is DriftChannel.Method.Async -> {
                        if (onMainThread) {
                            return reply(null, DriftCallError(
                                "blocks_ui_thread",
                                "drift: $channel.$method replies asynchronously; Go must call it " +
                                    "from a goroutine, not the UI thread",
                            ))
                        }
                        mainHandler.post {
                            val result = DriftResult("$channel.$method", reply)
                            try {
                                m.handler(args, result)
                            } catch (e: Exception) {
                                result.errorIfPending(e)
                            }
                        }
                    }
                }
            }
        }
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

    /** Encodes a failed call's exception as the JSON error payload Go decodes. */
    private fun errorPayload(e: Exception): String {
        val code = when (e) {
            is DriftCallError -> e.code
            is IllegalArgumentException -> "invalid_arguments"
            else -> "native_error"
        }
        val details = if (e is DriftCallError) null else mapOf("exception" to e.javaClass.name)
        return errorPayload(code, e.message ?: "Unknown error", details)
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
                clipboard.clearPrimaryClip()
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
            "success" -> HapticFeedbackConstants.CONFIRM
            "warning" -> HapticFeedbackConstants.REJECT
            "error" -> HapticFeedbackConstants.REJECT
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
        val vibratorManager = context.getSystemService(Context.VIBRATOR_MANAGER_SERVICE) as VibratorManager
        vibratorManager.defaultVibrator.vibrate(
            VibrationEffect.createOneShot(durationMs, VibrationEffect.DEFAULT_AMPLITUDE)
        )
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
