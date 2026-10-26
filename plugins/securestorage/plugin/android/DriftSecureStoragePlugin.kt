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
 * A file Auto Backup or device transfer restored does not decrypt (the
 * master key stays behind) and opens empty: see prefs.
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
import android.util.Log
import androidx.fragment.app.FragmentActivity
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import com.drift.runner.DriftActivityBinding
import com.drift.runner.DriftCallError
import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost
import com.drift.runner.DriftResult
import java.io.IOException
import java.security.GeneralSecurityException
import java.security.KeyStore
import java.security.KeyStoreException
import java.util.concurrent.Executors
import javax.crypto.Cipher
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec

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

    /**
     * The preference file name, opened. The file holds its values and the
     * Tink keysets that encrypt them, the keysets wrapped by the master key
     * in the Android Keystore. Auto Backup and device transfer restore the
     * file but not the key, which never leaves its device, so a restored
     * file cannot open again and its values are lost for good: it is reset
     * to empty. Keystore errors, which can be transient, fail the call and
     * leave the file alone.
     */
    private fun prefs(name: String): SharedPreferences = opened.getOrPut(name) {
        val key = MasterKey.Builder(context, MASTER_KEY_ALIAS)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build()
        try {
            open(name, key)
        } catch (e: GeneralSecurityException) {
            reset(name, key, e)
        } catch (e: IOException) {
            reset(name, key, e)
        }
    }

    private fun open(name: String, key: MasterKey): SharedPreferences = EncryptedSharedPreferences.create(
        context, name, key,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
    )

    /**
     * Opening name failed with cause. Tink fails the same way for keysets
     * the master key does not decrypt and for keystore errors, so name is
     * emptied and opened afresh only when the master key works and name
     * still fails to open; otherwise this throws cause.
     */
    private fun reset(name: String, key: MasterKey, cause: Exception): SharedPreferences {
        try {
            checkMasterKey()
        } catch (e: Exception) {
            cause.addSuppressed(e)
            throw cause
        }
        try {
            return open(name, key) // cause was transient
        } catch (e: GeneralSecurityException) {
            cause.addSuppressed(e)
        } catch (e: IOException) {
            cause.addSuppressed(e)
        }
        Log.w(TAG, "\"$name\" does not decrypt with this install's key (restored from a backup or another device?); its values are lost, starting it empty: $cause")
        // The keysets are entries of the file itself: clearing it drops them.
        context.getSharedPreferences(name, Context.MODE_PRIVATE).edit().clear().commit()
        return open(name, key)
    }

    /** Throws unless the master key encrypts and decrypts a probe. */
    private fun checkMasterKey() {
        val keyStore = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        val key = keyStore.getKey(MASTER_KEY_ALIAS, null) as? SecretKey
            ?: throw KeyStoreException("no master key $MASTER_KEY_ALIAS")
        val probe = CHANNEL.toByteArray()
        val seal = Cipher.getInstance(MASTER_KEY_CIPHER).apply { init(Cipher.ENCRYPT_MODE, key) }
        val sealed = seal.doFinal(probe)
        val unseal = Cipher.getInstance(MASTER_KEY_CIPHER).apply {
            init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(128, seal.iv))
        }
        if (!unseal.doFinal(sealed).contentEquals(probe)) {
            throw KeyStoreException("master key $MASTER_KEY_ALIAS does not round-trip")
        }
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
        const val TAG = "DriftSecureStorage"
        const val MASTER_KEY_ALIAS = MasterKey.DEFAULT_MASTER_KEY_ALIAS
        const val MASTER_KEY_CIPHER = "AES/GCM/NoPadding"
    }
}
