/**
 * DriftSecureStoragePlugin.kt
 *
 * The secure storage plugin's native half on Android: values in
 * EncryptedSharedPreferences, one file per service, and a second file for
 * values stored with requireBiometric. Storage work runs on one background
 * thread (the keystore is slow); reading a biometric value first passes
 * Biometrics.authenticate on the main thread. Biometric protection is a
 * gate the plugin enforces (BiometricPrompt without a CryptoObject); the
 * values are encrypted either way.
 *
 * Errors Go maps to its sentinels (runtime package): `auth_failed`,
 * `auth_canceled`, `biometrics_unavailable`, `biometrics_not_configured`,
 * `no_activity`.
 */
package com.drift.plugin.securestorage

import android.content.Context
import android.content.SharedPreferences
import android.os.Handler
import android.os.Looper
import androidx.fragment.app.FragmentActivity
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import com.drift.runner.DriftActivityBinding
import com.drift.runner.DriftCallError
import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost
import com.drift.runner.DriftResult
import java.util.concurrent.Executors

class DriftSecureStoragePlugin : DriftPlugin {
    private lateinit var context: Context
    private var activity: FragmentActivity? = null
    private val worker = Executors.newSingleThreadExecutor { Thread(it, "DriftSecureStorage") }
    private val main = Handler(Looper.getMainLooper())
    /** Opened preference files by name; worker thread only. */
    private val opened = HashMap<String, SharedPreferences>()

    override fun onRegister(host: DriftPluginHost) {
        context = host.context
        host.registerChannel(CHANNEL) {
            asyncMethod("set") { args, result ->
                val a = Args(args)
                val key = a.string("key")
                val value = a.string("value")
                if (a.requireBiometric) {
                    Biometrics.requireConfigured()
                    if (Biometrics.available(context) == "none") {
                        throw DriftCallError("biometrics_unavailable", "no biometrics enrolled")
                    }
                }
                work(result) {
                    val (target, other) = if (a.requireBiometric) {
                        prefs(a.biometricFile) to prefs(a.file)
                    } else {
                        prefs(a.file) to prefs(a.biometricFile)
                    }
                    other.edit().remove(key).commit()
                    target.edit().putString(key, value).commit()
                    null
                }
            }
            asyncMethod("get") { args, result ->
                val a = Args(args)
                val key = a.string("key")
                worker.execute {
                    try {
                        when {
                            prefs(a.file).contains(key) -> result.success(mapOf("value" to prefs(a.file).getString(key, null)))
                            prefs(a.biometricFile).contains(key) -> main.post { readBiometric(a, key, result) }
                            else -> result.success(mapOf("value" to null))
                        }
                    } catch (e: Exception) {
                        result.error(e)
                    }
                }
            }
            asyncMethod("delete") { args, result ->
                val a = Args(args)
                val key = a.string("key")
                work(result) {
                    prefs(a.file).edit().remove(key).commit()
                    prefs(a.biometricFile).edit().remove(key).commit()
                    null
                }
            }
            asyncMethod("contains") { args, result ->
                val a = Args(args)
                val key = a.string("key")
                work(result) { mapOf("exists" to (prefs(a.file).contains(key) || prefs(a.biometricFile).contains(key))) }
            }
            asyncMethod("keys") { args, result ->
                val a = Args(args)
                work(result) { mapOf("keys" to (prefs(a.file).all.keys + prefs(a.biometricFile).all.keys).toList()) }
            }
            asyncMethod("deleteAll") { args, result ->
                val a = Args(args)
                work(result) {
                    prefs(a.file).edit().clear().commit()
                    prefs(a.biometricFile).edit().clear().commit()
                    null
                }
            }
            asyncMethod("biometric") { _, result ->
                result.success(mapOf("type" to Biometrics.available(context)))
            }
        }
    }

    override fun onAttach(binding: DriftActivityBinding) {
        activity = binding.activity
    }

    override fun onDetach() {
        activity = null
    }

    /** Prompts for biometrics (main thread), then reads key on the worker. */
    private fun readBiometric(a: Args, key: String, result: DriftResult) {
        val activity = activity
            ?: return result.error(DriftCallError("no_activity", "no Activity to show the biometric prompt over"))
        Biometrics.authenticate(activity, a.biometricPrompt) { error ->
            if (error != null) {
                result.error(error)
            } else {
                work(result) { mapOf("value" to prefs(a.biometricFile).getString(key, null)) }
            }
        }
    }

    /** Runs body on the worker and replies with its value or its exception. */
    private fun work(result: DriftResult, body: () -> Any?) {
        worker.execute {
            try {
                result.success(body())
            } catch (e: Exception) {
                result.error(e)
            }
        }
    }

    private fun prefs(name: String): SharedPreferences = opened.getOrPut(name) {
        val key = MasterKey.Builder(context).setKeyScheme(MasterKey.KeyScheme.AES256_GCM).build()
        EncryptedSharedPreferences.create(
            context, name, key,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        )
    }

    /** A call's arguments. */
    private class Args(args: Any?) {
        private val m = args as? Map<*, *> ?: emptyMap<String, Any?>()
        private val service = (m["service"] as? String)?.takeIf { it.isNotEmpty() } ?: DEFAULT_FILE
        val file = service
        val biometricFile = "${service}_biometric"
        val requireBiometric = m["requireBiometric"] as? Boolean ?: false
        val biometricPrompt = m["biometricPrompt"] as? String ?: "Authenticate to read secure data"

        fun string(name: String): String =
            m[name] as? String ?: throw DriftCallError("invalid_arguments", "missing $name")
    }

    private companion object {
        const val CHANNEL = "drift/securestorage"
        const val DEFAULT_FILE = "drift_secure_storage"
    }
}
