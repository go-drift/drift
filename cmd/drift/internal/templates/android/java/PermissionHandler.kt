/**
 * PermissionHandler.kt
 * Handles runtime permission requests for the Drift platform channel.
 */
package {{.PackageName}}

import android.Manifest
import android.app.Activity
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.provider.Settings
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat

object PermissionHandler {
    private const val PERMISSION_REQUEST_CODE = 9001
    private var pendingCallback: ((Map<String, String>) -> Unit)? = null
    private var pendingPermissions: Array<String>? = null

    private val permissionMap = mapOf(
        "camera" to Manifest.permission.CAMERA,
        "microphone" to Manifest.permission.RECORD_AUDIO,
        "photos" to if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            Manifest.permission.READ_MEDIA_IMAGES
        } else {
            Manifest.permission.READ_EXTERNAL_STORAGE
        },
        "location" to Manifest.permission.ACCESS_FINE_LOCATION,
        "location_always" to Manifest.permission.ACCESS_BACKGROUND_LOCATION,
        "storage" to if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            Manifest.permission.READ_MEDIA_IMAGES
        } else {
            Manifest.permission.READ_EXTERNAL_STORAGE
        },
        "contacts" to Manifest.permission.READ_CONTACTS,
        "calendar" to Manifest.permission.READ_CALENDAR,
        "notifications" to if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            Manifest.permission.POST_NOTIFICATIONS
        } else {
            null
        }
    )

    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        return when (method) {
            "check" -> check(context, args)
            "request" -> request(context, args)
            "requestMultiple" -> requestMultiple(context, args)
            "openSettings" -> openSettings(context)
            "shouldShowRationale" -> shouldShowRationale(args)
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun check(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val permission = argsMap["permission"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing permission"))

        val status = checkPermissionStatus(context, permission)
        return Pair(mapOf("status" to status), null)
    }

    private fun request(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val permission = argsMap["permission"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing permission"))

        val activity = PlatformChannelManager.currentActivity()
            ?: return Pair(mapOf("status" to "denied"), null)

        val androidPermission = permissionMap[permission]
        if (androidPermission == null) {
            // Permission not required on this Android version
            return Pair(mapOf("status" to "granted"), null)
        }

        val currentStatus = checkPermissionStatus(context, permission)
        if (currentStatus == "granted") {
            return Pair(mapOf("status" to "granted"), null)
        }

        // Request permission synchronously by returning current state
        // The actual permission request happens asynchronously
        ActivityCompat.requestPermissions(activity, arrayOf(androidPermission), PERMISSION_REQUEST_CODE)

        // Return current status - the result will come through onRequestPermissionsResult
        return Pair(mapOf("status" to currentStatus), null)
    }

    private fun requestMultiple(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        @Suppress("UNCHECKED_CAST")
        val permissions = argsMap["permissions"] as? List<String>
            ?: return Pair(null, IllegalArgumentException("Missing permissions"))

        val activity = PlatformChannelManager.currentActivity()
            ?: return Pair(mapOf("results" to permissions.associateWith { "denied" }), null)

        val androidPermissions = permissions.mapNotNull { permissionMap[it] }.toTypedArray()

        if (androidPermissions.isEmpty()) {
            return Pair(mapOf("results" to permissions.associateWith { "granted" }), null)
        }

        val results = permissions.associateWith { checkPermissionStatus(context, it) }

        // Request any not-granted permissions
        val notGranted = androidPermissions.filter {
            ContextCompat.checkSelfPermission(context, it) != PackageManager.PERMISSION_GRANTED
        }

        if (notGranted.isNotEmpty()) {
            ActivityCompat.requestPermissions(activity, notGranted.toTypedArray(), PERMISSION_REQUEST_CODE)
        }

        return Pair(mapOf("results" to results), null)
    }

    private fun openSettings(context: Context): Pair<Any?, Exception?> {
        val intent = Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS).apply {
            data = Uri.fromParts("package", context.packageName, null)
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        context.startActivity(intent)
        return Pair(null, null)
    }

    private fun shouldShowRationale(args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val permission = argsMap["permission"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing permission"))

        val activity = PlatformChannelManager.currentActivity()
            ?: return Pair(mapOf("shouldShow" to false), null)

        val androidPermission = permissionMap[permission]
            ?: return Pair(mapOf("shouldShow" to false), null)

        val shouldShow = ActivityCompat.shouldShowRequestPermissionRationale(activity, androidPermission)
        return Pair(mapOf("shouldShow" to shouldShow), null)
    }

    private fun checkPermissionStatus(context: Context, permission: String): String {
        val androidPermission = permissionMap[permission]
            ?: return "granted" // Permission not required on this version

        return when {
            ContextCompat.checkSelfPermission(context, androidPermission) == PackageManager.PERMISSION_GRANTED -> "granted"
            Build.VERSION.SDK_INT >= Build.VERSION_CODES.M -> {
                val activity = PlatformChannelManager.currentActivity()
                if (activity != null && !ActivityCompat.shouldShowRequestPermissionRationale(activity, androidPermission)) {
                    // Either never asked or permanently denied
                    val prefs = context.getSharedPreferences("drift_permissions", Context.MODE_PRIVATE)
                    if (prefs.getBoolean("asked_$permission", false)) {
                        "permanently_denied"
                    } else {
                        "not_determined"
                    }
                } else {
                    "denied"
                }
            }
            else -> "denied"
        }
    }

    fun onRequestPermissionsResult(requestCode: Int, permissions: Array<out String>, grantResults: IntArray) {
        if (requestCode != PERMISSION_REQUEST_CODE) return

        val context = PlatformChannelManager.currentActivity() ?: return
        val prefs = context.getSharedPreferences("drift_permissions", Context.MODE_PRIVATE)
        val editor = prefs.edit()

        permissions.forEachIndexed { index, androidPermission ->
            // Find the drift permission name
            val driftPermission = permissionMap.entries.find { it.value == androidPermission }?.key
            if (driftPermission != null) {
                editor.putBoolean("asked_$driftPermission", true)

                val status = if (grantResults.getOrNull(index) == PackageManager.PERMISSION_GRANTED) {
                    "granted"
                } else if (!ActivityCompat.shouldShowRequestPermissionRationale(context, androidPermission)) {
                    "permanently_denied"
                } else {
                    "denied"
                }

                // Send permission change event
                PlatformChannelManager.sendEvent("drift/permissions/changes", mapOf(
                    "permission" to driftPermission,
                    "status" to status
                ))
            }
        }

        editor.apply()
    }
}
