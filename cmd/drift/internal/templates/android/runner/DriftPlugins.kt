/**
 * DriftPlugins.kt
 *
 * Owns the app's plugin instances and drives their lifecycle.
 * DriftApplication registers them and MainActivity attaches them; the
 * generated DriftPluginRegistrant only lists the plugins.
 * Drift's own handling of the same Activity events (deep links,
 * notifications) stays in MainActivity.
 *
 * Main thread only.
 *
 * IMPORTANT: shipped verbatim by Drift's scaffold; must not depend on the
 * user's app package.
 */
package com.drift.runner

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.os.Looper
import android.view.ViewGroup
import androidx.fragment.app.FragmentActivity
import java.io.FileNotFoundException

object DriftPlugins {
    private var plugins: List<DriftPlugin>? = null
    private var binding: DriftActivityBinding? = null

    /**
     * Creates the plugins and registers each with its own view of host (its
     * config, and the drift/ channel names closed to third-party plugins),
     * once per process, from Application.onCreate. A second call is a
     * wiring bug.
     */
    fun register(host: DriftPluginHost) {
        checkMainThread()
        check(plugins == null) { "drift: DriftPlugins.register called twice; call it once, from DriftApplication.onCreate" }
        val entries = DriftPluginRegistrant.makePlugins()
        plugins = entries.map { it.plugin }
        entries.forEach { it.plugin.onRegister(PluginHost(host, it)) }
    }

    /**
     * Attaches every plugin to activity, detaching from a previous Activity
     * first. overlayView hosts plugin overlays; isNewLaunch says whether
     * activity.intent is a new launch (DriftActivityBinding).
     */
    fun attach(activity: FragmentActivity, overlayView: ViewGroup, isNewLaunch: Boolean) {
        val all = requirePlugins()
        binding?.let { detachAll(it) }
        val b = DriftActivityBinding(activity, overlayView, isNewLaunch)
        binding = b
        all.forEach { it.onAttach(b) }
    }

    /** Detaches every plugin if they are attached to activity. */
    fun detach(activity: Activity) {
        checkMainThread()
        val b = binding ?: return
        if (b.activity !== activity) return
        detachAll(b)
    }

    /** Offers intent to attached plugins; true if one claimed it. */
    fun onNewIntent(intent: Intent): Boolean {
        checkMainThread()
        return binding?.dispatchNewIntent(intent) ?: false
    }

    private fun detachAll(b: DriftActivityBinding) {
        binding = null
        requirePlugins().asReversed().forEach { it.onDetach() }
        b.clear()
    }

    private fun requirePlugins(): List<DriftPlugin> {
        checkMainThread()
        return checkNotNull(plugins) {
            "drift: DriftPlugins.register must run first (DriftApplication.onCreate)"
        }
    }

    private fun checkMainThread() {
        check(Looper.myLooper() == Looper.getMainLooper()) {
            "drift: DriftPlugins must be used on the main thread"
        }
    }
}

/** One plugin the registrant lists: its Name(), whether it ships in the drift module, and its instance. */
internal class DriftPluginEntry(val name: String, val firstParty: Boolean, val plugin: DriftPlugin)

/**
 * The host one plugin registers with: Drift's, with the plugin's own
 * config, and Drift's reserved drift/ channel names closed unless the
 * plugin is first-party.
 */
private class PluginHost(private val host: DriftPluginHost, private val entry: DriftPluginEntry) : DriftPluginHost by host {
    override val config: Map<String, Any?> = readConfig(host.context, entry.name)

    override fun registerChannel(name: String, declare: DriftChannel.() -> Unit) {
        if (name.startsWith("drift/") && !entry.firstParty) {
            driftCrash("drift: plugin ${entry.name} registers channel \"$name\"; drift/ is reserved for Drift, so name it <vendor>/<feature>")
        }
        host.registerChannel(name, declare)
    }

    private companion object {
        /** The config the build shipped for plugin [name] (protocol.AndroidPluginConfigAsset), or none. */
        fun readConfig(context: Context, name: String): Map<String, Any?> {
            val path = "drift/plugins/$name.json"
            val bytes = try {
                context.assets.open(path).use { it.readBytes() }
            } catch (e: FileNotFoundException) {
                return emptyMap()
            }
            @Suppress("UNCHECKED_CAST")
            return JsonCodec.decode(bytes) as? Map<String, Any?>
                ?: driftCrash("drift: plugin $name's config $path is not a JSON object")
        }
    }
}
