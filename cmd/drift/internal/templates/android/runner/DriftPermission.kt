/**
 * DriftPermission.kt
 *
 * Runtime permissions provided by plugins. Drift owns the request
 * mechanism: Go's platform.Permission calls the drift/permissions channel,
 * which looks the kind up among those registered with
 * DriftPluginHost.registerPermission. A plugin registers each kind whose
 * manifest permissions it declares; its Go runtime exposes
 * platform.NewPermission(kind).
 *
 * Threading: Drift calls every method on the main thread.
 *
 * IMPORTANT: shipped verbatim by Drift's scaffold; must not depend on the
 * user's app package.
 */
package com.drift.runner

import android.app.Activity
import android.content.Context
import android.content.pm.PackageManager
import androidx.activity.result.ActivityResultLauncher
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import java.util.concurrent.atomic.AtomicLong

/** A permission status, as Go's platform.PermissionResult names it. */
enum class DriftPermissionStatus(val wire: String) {
    GRANTED("granted"),
    DENIED("denied"),
    PERMANENTLY_DENIED("permanently_denied"),
    RESTRICTED("restricted"),
    LIMITED("limited"),
    NOT_DETERMINED("not_determined"),
    PROVISIONAL("provisional"),
}

/** One kind of runtime permission, such as location or contacts. */
interface DriftPermission {
    /**
     * The current status, without prompting. [activity] is the foreground
     * Activity, null while none is resumed.
     */
    fun status(context: Context, activity: Activity?): DriftPermissionStatus

    /**
     * Prompts the user if that can change the status, then calls
     * [completion] exactly once, on the main thread, with the resulting
     * status. Drift calls it only when [status] is not GRANTED,
     * PERMANENTLY_DENIED or RESTRICTED. [options] are the kind-specific
     * options Go passed (empty for most kinds).
     */
    fun request(
        activity: FragmentActivity,
        options: Map<String, Any?>,
        completion: (DriftPermissionStatus) -> Unit,
    )

    /** Whether the app should explain why it needs the permission first. */
    fun shouldShowRationale(activity: Activity): Boolean = false
}

/**
 * A kind backed by Android runtime permissions, requested together: GRANTED
 * when all are granted, LIMITED when only some are (approximate location,
 * say). Each must be declared in the manifest (by the plugin registering
 * the kind); otherwise every call fails with `permission_not_declared` and
 * [whenUndeclared] as the fix.
 */
open class DriftRuntimePermission(
    vararg manifestPermissions: String,
    private val whenUndeclared: String = "add the plugin that declares it to drift.yaml",
) : DriftPermission {
    private val permissions: Array<String> = arrayOf(*manifestPermissions)

    init {
        require(permissions.isNotEmpty()) { "DriftRuntimePermission needs at least one manifest permission" }
    }

    override fun status(context: Context, activity: Activity?): DriftPermissionStatus {
        requireDeclared(context)
        val granted = permissions.count {
            ContextCompat.checkSelfPermission(context, it) == PackageManager.PERMISSION_GRANTED
        }
        return when {
            granted == permissions.size -> DriftPermissionStatus.GRANTED
            granted > 0 -> DriftPermissionStatus.LIMITED
            // Android only says "don't ask again" through the rationale flag
            // after a request, so remember whether one was made.
            !asked(context) -> DriftPermissionStatus.NOT_DETERMINED
            activity == null || shouldShowRationale(activity) -> DriftPermissionStatus.DENIED
            else -> DriftPermissionStatus.PERMANENTLY_DENIED
        }
    }

    override fun request(
        activity: FragmentActivity,
        options: Map<String, Any?>,
        completion: (DriftPermissionStatus) -> Unit,
    ) {
        requireDeclared(activity)
        var finished = false
        lateinit var launcher: ActivityResultLauncher<Array<String>>
        lateinit var observer: LifecycleEventObserver
        fun finish() {
            if (finished) return
            finished = true
            launcher.unregister()
            activity.lifecycle.removeObserver(observer)
            markAsked(activity)
            completion(status(activity, activity))
        }
        // The result goes to the Activity that launched the prompt; if it is
        // destroyed first (recreated behind the dialog), answer with the
        // status as it stands rather than never.
        observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_DESTROY) finish()
        }
        launcher = activity.activityResultRegistry.register(
            "drift.permission.${requests.incrementAndGet()}",
            ActivityResultContracts.RequestMultiplePermissions(),
        ) { finish() }
        activity.lifecycle.addObserver(observer)
        launcher.launch(permissions)
    }

    override fun shouldShowRationale(activity: Activity): Boolean =
        permissions.any { ActivityCompat.shouldShowRequestPermissionRationale(activity, it) }

    private fun requireDeclared(context: Context) {
        val missing = permissions.filterNot { it in declaredPermissions(context) }
        if (missing.isNotEmpty()) {
            throw DriftCallError(
                "permission_not_declared",
                "AndroidManifest.xml does not declare ${missing.joinToString()}; $whenUndeclared",
            )
        }
    }

    private val askedKey = "asked:" + permissions.sorted().joinToString(",")

    private fun asked(context: Context): Boolean =
        prefs(context).getBoolean(askedKey, false)

    private fun markAsked(context: Context) {
        prefs(context).edit().putBoolean(askedKey, true).apply()
    }

    companion object {
        /** Whether the merged manifest declares [permission]. */
        fun isDeclared(context: Context, permission: String): Boolean =
            permission in declaredPermissions(context)

        private val requests = AtomicLong()

        @Volatile
        private var declared: Set<String>? = null

        private fun prefs(context: Context) =
            context.getSharedPreferences("drift_permissions", Context.MODE_PRIVATE)

        @Suppress("DEPRECATION") // PackageInfoFlags needs API 33; minSdk is 31.
        private fun declaredPermissions(context: Context): Set<String> =
            declared ?: context.packageManager
                .getPackageInfo(context.packageName, PackageManager.GET_PERMISSIONS)
                .requestedPermissions.orEmpty().toSet()
                .also { declared = it }
    }
}
