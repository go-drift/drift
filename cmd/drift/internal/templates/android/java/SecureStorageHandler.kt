/**
 * SecureStorageHandler.kt
 * Handles secure storage using EncryptedSharedPreferences with optional BiometricPrompt.
 *
 * Note on biometric protection: Android's BiometricPrompt without a CryptoObject provides
 * app-level authentication (UI gate) but not cryptographic per-operation verification.
 * Biometric-protected values are still encrypted at rest via EncryptedSharedPreferences,
 * but the biometric check is an app-enforced policy, not hardware-enforced.
 */
package {{.PackageName}}

import android.content.Context
import android.content.SharedPreferences
import androidx.biometric.BiometricManager
import androidx.biometric.BiometricPrompt
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import java.util.concurrent.Executor

object SecureStorageHandler {
    // Error codes matching Go constants
    private const val ERROR_ITEM_NOT_FOUND = "item_not_found"
    private const val ERROR_AUTH_FAILED = "auth_failed"
    private const val ERROR_AUTH_CANCELLED = "auth_cancelled"
    private const val ERROR_BIOMETRIC_NOT_AVAILABLE = "biometric_not_available"
    private const val ERROR_BIOMETRIC_NOT_ENROLLED = "biometric_not_enrolled"

    private const val DEFAULT_PREFS_NAME = "drift_secure_storage"
    private const val BIOMETRIC_PREFS_SUFFIX = "_biometric"

    // Cache for encrypted preferences
    private val prefsCache = mutableMapOf<String, SharedPreferences>()

    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        return when (method) {
            "set" -> set(context, args)
            "get" -> get(context, args)
            "delete" -> delete(context, args)
            "contains" -> contains(context, args)
            "getAllKeys" -> getAllKeys(context, args)
            "deleteAll" -> deleteAll(context, args)
            "isBiometricAvailable" -> isBiometricAvailable(context)
            "getBiometricType" -> getBiometricType(context)
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    // MARK: - CRUD Operations

    private fun set(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val key = argsMap["key"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing key"))
        val value = argsMap["value"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing value"))

        val service = argsMap["service"] as? String
        val requireBiometric = argsMap["requireBiometric"] as? Boolean ?: false
        val biometricPrompt = argsMap["biometricPrompt"] as? String

        return try {
            if (requireBiometric) {
                val activity = PlatformChannelManager.currentActivity() as? FragmentActivity
                if (activity == null) {
                    return Pair(null, IllegalStateException("No FragmentActivity available for biometric"))
                }

                // Authenticate first, then store in biometric-protected prefs
                authenticateAndExecute(
                    activity = activity,
                    promptMessage = biometricPrompt ?: "Authenticate to save securely",
                    onSuccess = {
                        try {
                            // Store in separate biometric prefs (still encrypted, but biometric is UI gate)
                            val prefs = getBiometricPrefs(context, service)
                            prefs.edit().putString(key, value).apply()
                            addBiometricKey(context, service, key)
                            sendAuthResult(success = true, key = key)
                        } catch (e: Exception) {
                            sendAuthResult(success = false, key = key, error = ERROR_AUTH_FAILED)
                        }
                    },
                    onError = { errorCode ->
                        sendAuthResult(success = false, key = key, error = errorCode)
                    }
                )
                // Return pending since this is async
                Pair(mapOf("pending" to true), null)
            } else {
                val prefs = getEncryptedPrefs(context, service)
                prefs.edit().putString(key, value).apply()
                Pair(null, null)
            }
        } catch (e: Exception) {
            Pair(null, e)
        }
    }

    private fun get(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val key = argsMap["key"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing key"))

        val service = argsMap["service"] as? String
        val biometricPrompt = argsMap["biometricPrompt"] as? String

        return try {
            // Check regular encrypted storage first (no auth required)
            val regularPrefs = getEncryptedPrefs(context, service)
            if (regularPrefs.contains(key)) {
                val value = regularPrefs.getString(key, null)
                return Pair(mapOf("value" to value), null)
            }

            // Check if key exists in biometric storage (metadata check only)
            if (biometricKeyExists(context, service, key)) {
                val activity = PlatformChannelManager.currentActivity() as? FragmentActivity
                if (activity == null) {
                    return Pair(null, IllegalStateException("No FragmentActivity available for biometric"))
                }

                authenticateAndExecute(
                    activity = activity,
                    promptMessage = biometricPrompt ?: "Authenticate to access secure data",
                    onSuccess = {
                        try {
                            val biometricPrefs = getBiometricPrefs(context, service)
                            val value = biometricPrefs.getString(key, null)
                            sendAuthResult(success = true, key = key, value = value)
                        } catch (e: Exception) {
                            sendAuthResult(success = false, key = key, error = ERROR_AUTH_FAILED)
                        }
                    },
                    onError = { errorCode ->
                        sendAuthResult(success = false, key = key, error = errorCode)
                    }
                )
                return Pair(mapOf("pending" to true), null)
            }

            // Key not found in either storage
            Pair(mapOf("value" to null), null)
        } catch (e: Exception) {
            Pair(null, e)
        }
    }

    private fun delete(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val key = argsMap["key"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing key"))

        val service = argsMap["service"] as? String
        val biometricPrompt = argsMap["biometricPrompt"] as? String

        return try {
            // Delete from regular storage (no auth required)
            getEncryptedPrefs(context, service).edit().remove(key).apply()

            // If biometric key exists, authenticate then delete
            if (biometricKeyExists(context, service, key)) {
                val activity = PlatformChannelManager.currentActivity() as? FragmentActivity
                if (activity == null) {
                    // Can't authenticate - just remove tracking, data remains orphaned
                    removeBiometricKey(context, service, key)
                    return Pair(null, null)
                }

                authenticateAndExecute(
                    activity = activity,
                    promptMessage = biometricPrompt ?: "Authenticate to delete secure data",
                    onSuccess = {
                        try {
                            getBiometricPrefs(context, service).edit().remove(key).apply()
                            removeBiometricKey(context, service, key)
                            sendAuthResult(success = true, key = key)
                        } catch (e: Exception) {
                            // Still remove tracking even if prefs delete fails
                            removeBiometricKey(context, service, key)
                            sendAuthResult(success = true, key = key)
                        }
                    },
                    onError = { errorCode ->
                        // On auth failure, still remove tracking (orphan the data)
                        removeBiometricKey(context, service, key)
                        sendAuthResult(success = false, key = key, error = errorCode)
                    }
                )
                return Pair(mapOf("pending" to true), null)
            }

            Pair(null, null)
        } catch (e: Exception) {
            Pair(null, e)
        }
    }

    private fun contains(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val key = argsMap["key"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing key"))

        val service = argsMap["service"] as? String

        return try {
            val regularExists = getEncryptedPrefs(context, service).contains(key)
            val biometricExists = biometricKeyExists(context, service, key)
            Pair(mapOf("exists" to (regularExists || biometricExists)), null)
        } catch (e: Exception) {
            Pair(null, e)
        }
    }

    private fun getAllKeys(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
        val service = argsMap?.get("service") as? String

        return try {
            val regularKeys = getEncryptedPrefs(context, service).all.keys
            val biometricKeys = getBiometricKeySet(context, service)
            val allKeys = (regularKeys + biometricKeys).toList()
            Pair(mapOf("keys" to allKeys), null)
        } catch (e: Exception) {
            Pair(null, e)
        }
    }

    private fun deleteAll(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
        val service = argsMap?.get("service") as? String

        return try {
            getEncryptedPrefs(context, service).edit().clear().apply()
            getBiometricPrefs(context, service).edit().clear().apply()
            clearBiometricKeys(context, service)
            Pair(null, null)
        } catch (e: Exception) {
            Pair(null, e)
        }
    }

    // MARK: - Biometric Methods

    private fun isBiometricAvailable(context: Context): Pair<Any?, Exception?> {
        val biometricManager = BiometricManager.from(context)
        val canAuthenticate = biometricManager.canAuthenticate(BiometricManager.Authenticators.BIOMETRIC_STRONG)
        val available = canAuthenticate == BiometricManager.BIOMETRIC_SUCCESS
        return Pair(mapOf("available" to available), null)
    }

    private fun getBiometricType(context: Context): Pair<Any?, Exception?> {
        val biometricManager = BiometricManager.from(context)
        val canAuthenticate = biometricManager.canAuthenticate(BiometricManager.Authenticators.BIOMETRIC_STRONG)

        return when (canAuthenticate) {
            BiometricManager.BIOMETRIC_SUCCESS -> {
                val type = if (context.packageManager.hasSystemFeature("android.hardware.biometrics.face")) {
                    "face"
                } else {
                    "fingerprint"
                }
                Pair(mapOf("type" to type), null)
            }
            BiometricManager.BIOMETRIC_ERROR_NO_HARDWARE -> {
                Pair(mapOf("type" to "none", "reason" to "no_hardware"), null)
            }
            BiometricManager.BIOMETRIC_ERROR_HW_UNAVAILABLE -> {
                Pair(mapOf("type" to "none", "reason" to "unavailable"), null)
            }
            BiometricManager.BIOMETRIC_ERROR_NONE_ENROLLED -> {
                Pair(mapOf("type" to "none", "reason" to "not_enrolled"), null)
            }
            else -> {
                Pair(mapOf("type" to "none"), null)
            }
        }
    }

    // MARK: - Encrypted Preferences Helpers

    private fun getEncryptedPrefs(context: Context, service: String?): SharedPreferences {
        val prefsName = service ?: DEFAULT_PREFS_NAME

        return prefsCache.getOrPut(prefsName) {
            val masterKey = MasterKey.Builder(context)
                .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
                .build()

            EncryptedSharedPreferences.create(
                context,
                prefsName,
                masterKey,
                EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
            )
        }
    }

    private fun getBiometricPrefs(context: Context, service: String?): SharedPreferences {
        val baseName = service ?: DEFAULT_PREFS_NAME
        val prefsName = baseName + BIOMETRIC_PREFS_SUFFIX

        // Biometric prefs use the same encryption as regular prefs.
        // The biometric check is an app-level UI gate, not hardware-enforced per-operation.
        // This is because BiometricPrompt without CryptoObject doesn't cryptographically
        // unlock keys - it only provides user verification.
        return prefsCache.getOrPut(prefsName) {
            val masterKey = MasterKey.Builder(context)
                .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
                .build()

            EncryptedSharedPreferences.create(
                context,
                prefsName,
                masterKey,
                EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
                EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
            )
        }
    }

    // MARK: - Biometric Key Tracking
    // Track which keys require biometric auth (app-level policy)

    private fun getBiometricKeyTrackingPrefs(context: Context): SharedPreferences {
        return context.getSharedPreferences("drift_biometric_keys", Context.MODE_PRIVATE)
    }

    private fun biometricKeyExists(context: Context, service: String?, key: String): Boolean {
        val trackingKey = (service ?: DEFAULT_PREFS_NAME) + ":" + key
        return getBiometricKeyTrackingPrefs(context).contains(trackingKey)
    }

    private fun addBiometricKey(context: Context, service: String?, key: String) {
        val trackingKey = (service ?: DEFAULT_PREFS_NAME) + ":" + key
        getBiometricKeyTrackingPrefs(context).edit().putBoolean(trackingKey, true).apply()
    }

    private fun removeBiometricKey(context: Context, service: String?, key: String) {
        val trackingKey = (service ?: DEFAULT_PREFS_NAME) + ":" + key
        getBiometricKeyTrackingPrefs(context).edit().remove(trackingKey).apply()
    }

    private fun getBiometricKeySet(context: Context, service: String?): Set<String> {
        val prefix = (service ?: DEFAULT_PREFS_NAME) + ":"
        return getBiometricKeyTrackingPrefs(context).all.keys
            .filter { it.startsWith(prefix) }
            .map { it.removePrefix(prefix) }
            .toSet()
    }

    private fun clearBiometricKeys(context: Context, service: String?) {
        val prefix = (service ?: DEFAULT_PREFS_NAME) + ":"
        val prefs = getBiometricKeyTrackingPrefs(context)
        val editor = prefs.edit()
        prefs.all.keys.filter { it.startsWith(prefix) }.forEach { editor.remove(it) }
        editor.apply()
    }

    // MARK: - Biometric Authentication

    private fun authenticateAndExecute(
        activity: FragmentActivity,
        promptMessage: String,
        onSuccess: () -> Unit,
        onError: (String) -> Unit
    ) {
        val executor: Executor = ContextCompat.getMainExecutor(activity)

        val callback = object : BiometricPrompt.AuthenticationCallback() {
            override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) {
                super.onAuthenticationSucceeded(result)
                onSuccess()
            }

            override fun onAuthenticationError(errorCode: Int, errString: CharSequence) {
                super.onAuthenticationError(errorCode, errString)
                val code = when (errorCode) {
                    BiometricPrompt.ERROR_USER_CANCELED,
                    BiometricPrompt.ERROR_NEGATIVE_BUTTON -> ERROR_AUTH_CANCELLED
                    BiometricPrompt.ERROR_NO_BIOMETRICS -> ERROR_BIOMETRIC_NOT_ENROLLED
                    BiometricPrompt.ERROR_HW_NOT_PRESENT,
                    BiometricPrompt.ERROR_HW_UNAVAILABLE -> ERROR_BIOMETRIC_NOT_AVAILABLE
                    else -> ERROR_AUTH_FAILED
                }
                onError(code)
            }

            override fun onAuthenticationFailed() {
                super.onAuthenticationFailed()
                // Don't call onError - system will show retry or eventually call onAuthenticationError
            }
        }

        val biometricPrompt = BiometricPrompt(activity, executor, callback)

        val promptInfo = BiometricPrompt.PromptInfo.Builder()
            .setTitle("Authentication Required")
            .setSubtitle(promptMessage)
            .setNegativeButtonText("Cancel")
            .setAllowedAuthenticators(BiometricManager.Authenticators.BIOMETRIC_STRONG)
            .build()

        activity.runOnUiThread {
            biometricPrompt.authenticate(promptInfo)
        }
    }

    // MARK: - Event Sending

    private fun sendAuthResult(success: Boolean, key: String, value: String? = null, error: String? = null) {
        val result = mutableMapOf<String, Any?>(
            "type" to "auth_result",
            "success" to success,
            "key" to key
        )
        if (value != null) {
            result["value"] = value
        }
        if (error != null) {
            result["error"] = error
        }
        PlatformChannelManager.sendEvent("drift/secure_storage/events", result)
    }
}
