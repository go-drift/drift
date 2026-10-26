/**
 * DriftJsonCodec.kt
 *
 * The JSON codec of the values crossing a platform channel, shared by the
 * host (PlatformChannelManager) and DriftPlugins, which reads plugin configs.
 *
 * IMPORTANT: shipped verbatim by Drift's scaffold; must not depend on the
 * user's app package.
 */
package com.drift.runner

import org.json.JSONArray
import org.json.JSONObject
import org.json.JSONTokener

/**
 * JSON codec for the values crossing a platform channel: null, booleans,
 * finite numbers, strings, lists and string-keyed maps. Mirrors the iOS
 * JsonCodec.
 */
internal object JsonCodec {
    /** The outcome of [encode]. */
    sealed interface Encoded {
        class Bytes(val data: ByteArray) : Encoded

        /** Some part of the value is not JSON-like; [reason] locates it, as in `value.items[2]`. */
        class Unencodable(val reason: String) : Encoded
    }

    /** Thrown inside [toJson] only, carrying [Encoded.Unencodable.reason]. */
    private class NotJsonLike(val reason: String) : Exception(reason)

    /** Encodes a JSON-like value, or fails naming the first part that is not. */
    fun encode(value: Any?): Encoded {
        val jsonValue = try {
            toJson(value, "value")
        } catch (e: NotJsonLike) {
            return Encoded.Unencodable(e.reason)
        }
        // toJson admits only finite numbers, so org.json cannot refuse these.
        val jsonString = when (jsonValue) {
            JSONObject.NULL -> "null"
            is JSONObject, is JSONArray -> jsonValue.toString()
            is String -> JSONObject.quote(jsonValue)
            is Number, is Boolean -> jsonValue.toString()
            else -> error("drift: JsonCodec.toJson returned a ${jsonValue.javaClass.name}")
        }
        return Encoded.Bytes(jsonString.toByteArray(Charsets.UTF_8))
    }

    fun decode(data: ByteArray): Any? {
        if (data.isEmpty()) return null
        val jsonString = String(data, Charsets.UTF_8)
        val parsed = JSONTokener(jsonString).nextValue()
        return fromJson(parsed)
    }

    /** The org.json form of [value], found at [path]. */
    private fun toJson(value: Any?, path: String): Any {
        return when (value) {
            null, Unit, JSONObject.NULL -> JSONObject.NULL
            // Rebuilt to check its numbers: a JSONArray accepts NaN.
            is JSONObject, is JSONArray -> toJson(fromJson(value), path)
            is Double, is Float -> {
                if (!(value as Number).toDouble().isFinite()) {
                    throw NotJsonLike("$path is $value, not a finite number")
                }
                value
            }
            is Boolean, is Number, is String -> value
            is Map<*, *> -> {
                val obj = JSONObject()
                for ((key, item) in value) {
                    if (key != null) {
                        obj.put(key.toString(), toJson(item, "$path.$key"))
                    }
                }
                obj
            }
            is Iterable<*> -> JSONArray().also { arr ->
                value.forEachIndexed { i, item -> arr.put(toJson(item, "$path[$i]")) }
            }
            is Array<*> -> JSONArray().also { arr ->
                value.forEachIndexed { i, item -> arr.put(toJson(item, "$path[$i]")) }
            }
            else -> throw NotJsonLike(
                "$path is a ${value.javaClass.name}; values must be null, booleans, finite numbers, strings, lists or string-keyed maps"
            )
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
