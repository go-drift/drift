/**
 * PreferencesHandler.kt
 * Handles simple key-value storage using SharedPreferences.
 */
package {{.PackageName}}

import android.content.Context

object PreferencesHandler {
    // A dedicated SharedPreferences file isolates drift keys from the rest of
    // the app, so no per-key prefix is needed (unlike iOS, which shares
    // UserDefaults.standard app-wide and therefore prefixes every key).
    private const val PREFS_NAME = "drift_preferences"

    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        return when (method) {
            "set" -> set(context, args)
            "get" -> get(context, args)
            "delete" -> delete(context, args)
            "contains" -> contains(context, args)
            "getAllKeys" -> getAllKeys(context)
            "deleteAll" -> deleteAll(context)
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun set(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val key = argsMap["key"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing key"))
        val value = argsMap["value"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing value"))

        getPrefs(context).edit().putString(key, value).apply()
        return Pair(null, null)
    }

    private fun get(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val key = argsMap["key"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing key"))

        val value = getPrefs(context).getString(key, null)
        return Pair(mapOf("value" to value), null)
    }

    private fun delete(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val key = argsMap["key"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing key"))

        getPrefs(context).edit().remove(key).apply()
        return Pair(null, null)
    }

    private fun contains(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val key = argsMap["key"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing key"))

        val exists = getPrefs(context).contains(key)
        return Pair(mapOf("exists" to exists), null)
    }

    private fun getAllKeys(context: Context): Pair<Any?, Exception?> {
        val keys = getPrefs(context).all.keys.toList()
        return Pair(mapOf("keys" to keys), null)
    }

    private fun deleteAll(context: Context): Pair<Any?, Exception?> {
        getPrefs(context).edit().clear().apply()
        return Pair(null, null)
    }

    private fun getPrefs(context: Context) =
        context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
}
