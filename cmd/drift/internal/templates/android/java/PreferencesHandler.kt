/**
 * PreferencesHandler.kt
 * Handles simple key-value storage using SharedPreferences.
 */
package {{.PackageName}}

import android.content.Context
import com.drift.runner.DriftPluginHost

object PreferencesHandler {
    // A dedicated SharedPreferences file isolates drift keys from the rest of
    // the app, so no per-key prefix is needed (unlike iOS, which shares
    // UserDefaults.standard app-wide and therefore prefixes every key).
    private const val PREFS_NAME = "drift_preferences"

    fun start(host: DriftPluginHost) {
        val prefs = host.context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        host.registerChannel("drift/preferences") {
            method("set") { args ->
                val key = key(args)
                val value = (args as Map<*, *>)["value"] as? String
                    ?: throw IllegalArgumentException("Missing value")
                prefs.edit().putString(key, value).apply()
                null
            }
            method("get") { args -> mapOf("value" to prefs.getString(key(args), null)) }
            method("delete") { args ->
                prefs.edit().remove(key(args)).apply()
                null
            }
            method("contains") { args -> mapOf("exists" to prefs.contains(key(args))) }
            method("getAllKeys") { mapOf("keys" to prefs.all.keys.toList()) }
            method("deleteAll") {
                prefs.edit().clear().apply()
                null
            }
        }
    }

    private fun key(args: Any?): String {
        val argsMap = args as? Map<*, *> ?: throw IllegalArgumentException("Invalid arguments")
        return argsMap["key"] as? String ?: throw IllegalArgumentException("Missing key")
    }
}
