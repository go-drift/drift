/**
 * PermissionHandler.kt
 *
 * The drift/permissions channel: Drift's permission request mechanism.
 * Permission kinds are registered by plugins (DriftPluginHost
 * registerPermission) and by Drift itself (notifications, below); Go's
 * platform.Permission reaches a kind by name. "request" is an async method
 * that replies with the status once the user has answered, and reports it
 * on drift/permissions/changes.
 */
package {{.PackageName}}

import android.Manifest
import android.app.Activity
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import androidx.core.app.NotificationManagerCompat
import androidx.fragment.app.FragmentActivity
import com.drift.runner.DriftCallError
import com.drift.runner.DriftPermission
import com.drift.runner.DriftPermissionStatus
import com.drift.runner.DriftPluginHost
import com.drift.runner.DriftRuntimePermission
import com.drift.runner.driftCrash
import java.util.concurrent.ConcurrentHashMap

object PermissionHandler {
    private val kinds = ConcurrentHashMap<String, DriftPermission>()

    /** Registers the permission [kind]; a second registration crashes. */
    fun register(kind: String, permission: DriftPermission) {
        if (kinds.putIfAbsent(kind, permission) != null) {
            driftCrash("drift: permission kind \"$kind\" is already registered")
        }
    }

    /** Registers the drift/permissions channel and Drift's own kinds. */
    fun start(host: DriftPluginHost) {
        val context = host.context
        host.registerChannel("drift/permissions") {
            method("check") { args ->
                mapOf("status" to lookup(args).second.status(context, foreground()).wire)
            }
            asyncMethod("request") { args, result ->
                val (kind, permission) = lookup(args)
                val current = permission.status(context, foreground())
                when (current) {
                    // No prompt can change these.
                    DriftPermissionStatus.GRANTED,
                    DriftPermissionStatus.PERMANENTLY_DENIED,
                    DriftPermissionStatus.RESTRICTED -> {
                        result.success(mapOf("status" to current.wire))
                        return@asyncMethod
                    }
                    else -> {}
                }
                val activity = foreground() as? FragmentActivity
                    ?: throw DriftCallError("platform_unavailable", "no foreground Activity to show the $kind permission prompt")
                @Suppress("UNCHECKED_CAST")
                val options = (args as Map<String, Any?>) - "permission"
                permission.request(activity, options) { status ->
                    host.sendEvent("drift/permissions/changes", mapOf("permission" to kind, "status" to status.wire))
                    result.success(mapOf("status" to status.wire))
                }
            }
            method("shouldShowRationale") { args ->
                val permission = lookup(args).second
                val activity = foreground()
                mapOf("shouldShow" to (activity != null && permission.shouldShowRationale(activity)))
            }
            method("openSettings") { _ ->
                context.startActivity(Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS).apply {
                    data = Uri.fromParts("package", context.packageName, null)
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                })
                null
            }
        }

        host.registerPermission("notifications", NotificationsPermission)
        // Moving to plugins (Phase 5); registered here until each moves.
        host.registerPermission("camera", DriftRuntimePermission(Manifest.permission.CAMERA))
        host.registerPermission("microphone", DriftRuntimePermission(Manifest.permission.RECORD_AUDIO))
        host.registerPermission("photos", DriftRuntimePermission(
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                Manifest.permission.READ_MEDIA_IMAGES
            } else {
                Manifest.permission.READ_EXTERNAL_STORAGE
            }
        ))
        host.registerPermission("location", DriftRuntimePermission(
            Manifest.permission.ACCESS_FINE_LOCATION,
            Manifest.permission.ACCESS_COARSE_LOCATION,
        ))
        host.registerPermission("location_always", DriftRuntimePermission(Manifest.permission.ACCESS_BACKGROUND_LOCATION))
        host.registerPermission("contacts", DriftRuntimePermission(Manifest.permission.READ_CONTACTS))
        host.registerPermission("calendar", DriftRuntimePermission(Manifest.permission.READ_CALENDAR))
    }

    private fun foreground(): Activity? = PlatformChannelManager.currentActivity()

    private fun lookup(args: Any?): Pair<String, DriftPermission> {
        val kind = (args as? Map<*, *>)?.get("permission") as? String
            ?: throw IllegalArgumentException("drift/permissions: missing permission kind")
        val permission = kinds[kind] ?: throw DriftCallError(
            "permission_not_registered",
            "no plugin registered the permission kind \"$kind\"; add the plugin that provides it to drift.yaml",
        )
        return kind to permission
    }
}

/**
 * Notification permission: POST_NOTIFICATIONS from Android 13; before
 * that, whether the user left notifications enabled. Drift owns the kind,
 * but plugins that show notifications declare the manifest permission.
 */
private object NotificationsPermission : DriftPermission {
    private val runtime = DriftRuntimePermission(
        Manifest.permission.POST_NOTIFICATIONS,
        whenUndeclared = "add a plugin that shows notifications (notifications or firebase) to drift.yaml",
    )

    override fun status(context: Context, activity: Activity?): DriftPermissionStatus {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            return runtime.status(context, activity)
        }
        // Checked for the declaration, so a missing plugin fails the same
        // way on every API level.
        runtime.status(context, activity)
        return if (NotificationManagerCompat.from(context).areNotificationsEnabled()) {
            DriftPermissionStatus.GRANTED
        } else {
            DriftPermissionStatus.PERMANENTLY_DENIED
        }
    }

    override fun request(
        activity: FragmentActivity,
        options: Map<String, Any?>,
        completion: (DriftPermissionStatus) -> Unit,
    ) {
        // Below Android 13, status() is never one that reaches here.
        runtime.request(activity, options, completion)
    }

    override fun shouldShowRationale(activity: Activity): Boolean =
        Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU && runtime.shouldShowRationale(activity)
}
