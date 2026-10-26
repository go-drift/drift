/**
 * DriftLocationPlugin.kt
 *
 * The location plugin's native half on Android, on the main thread, over
 * Google Play services' fused location provider: one-shot fixes, and
 * updates streamed to Go on `drift/location` while Go listens, with the
 * options `setUpdateOptions` set. A background stream
 * also runs DriftLocationService, a foreground service that keeps the
 * process alive and its updates unthrottled once the app leaves the
 * screen. Registers the `location` permission (fine and coarse; coarse
 * only reads as limited), and `location_always` when the build declared
 * background location.
 *
 * Errors Go maps to its sentinels (runtime package): `permission_denied`,
 * `location_disabled`, `background_not_configured`, `always_denied` and
 * `background_start`.
 */
package com.drift.plugin.location

import android.Manifest
import android.annotation.SuppressLint
import android.app.ForegroundServiceStartNotAllowedException
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.location.Location
import android.location.LocationManager
import android.os.Looper
import androidx.core.content.ContextCompat
import com.drift.runner.DriftCallError
import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost
import com.drift.runner.DriftRuntimePermission
import com.google.android.gms.location.CurrentLocationRequest
import com.google.android.gms.location.FusedLocationProviderClient
import com.google.android.gms.location.LocationCallback
import com.google.android.gms.location.LocationRequest
import com.google.android.gms.location.LocationResult
import com.google.android.gms.location.LocationServices
import com.google.android.gms.location.Priority

class DriftLocationPlugin : DriftPlugin {
    private lateinit var host: DriftPluginHost
    private lateinit var client: FusedLocationProviderClient
    /** The running update stream's callback, null when none runs. */
    private var updates: LocationCallback? = null
    /** The update stream's options, kept across starts. */
    private var options = UpdateOptions()
    /** The background service's notification, from host.config. */
    private var notificationTitle: String? = null
    private var notificationText: String? = null
    /** Whether DriftLocationService runs, for a background stream. */
    private var serviceRunning = false

    override fun onRegister(host: DriftPluginHost) {
        this.host = host
        // Set with the always: config (plugin.go nativeConfig).
        notificationTitle = host.config["notificationTitle"] as? String
        notificationText = host.config["notificationText"] as? String
        val context = host.context
        client = LocationServices.getFusedLocationProviderClient(context)
        host.registerPermission("location", DriftRuntimePermission(
            Manifest.permission.ACCESS_FINE_LOCATION,
            Manifest.permission.ACCESS_COARSE_LOCATION,
        ))
        if (DriftRuntimePermission.isDeclared(context, Manifest.permission.ACCESS_BACKGROUND_LOCATION)) {
            host.registerPermission("location_always", DriftRuntimePermission(Manifest.permission.ACCESS_BACKGROUND_LOCATION))
        }
        host.registerChannel(CHANNEL) {
            asyncMethod("getCurrentLocation") { args, result ->
                requireUsable(context)
                val highAccuracy = (args as? Map<*, *>)?.get("highAccuracy") as? Boolean ?: true
                val request = CurrentLocationRequest.Builder()
                    .setPriority(priority(highAccuracy))
                    .setMaxUpdateAgeMillis(10_000)
                    .build()
                @SuppressLint("MissingPermission") // requireUsable checked it.
                val task = client.getCurrentLocation(request, null)
                task.addOnSuccessListener { location ->
                    if (location == null) {
                        result.error(DriftCallError("location_unavailable", "no location fix available"))
                    } else {
                        result.success(payload(location))
                    }
                }.addOnFailureListener { result.error(it) }
            }
            asyncMethod("getLastKnown") { _, result ->
                requireUsable(context)
                @SuppressLint("MissingPermission") // requireUsable checked it.
                val task = client.lastLocation
                task.addOnSuccessListener { location -> result.success(location?.let(::payload)) }
                    .addOnFailureListener { result.error(it) }
            }
            asyncMethod("isEnabled") { _, result ->
                result.success(mapOf("enabled" to servicesEnabled(context)))
            }
            method("setUpdateOptions") { args ->
                val next = UpdateOptions.parse(args as? Map<*, *> ?: emptyMap<String, Any>())
                if (next.background) requireBackground(context)
                if (updates != null) {
                    requireUsable(context)
                    start(context, next)
                }
                options = next
                null
            }
            stream(
                onListen = {
                    requireUsable(context)
                    if (options.background) requireBackground(context)
                    start(context, options)
                },
                onCancel = {
                    removeUpdates()
                    stopService(context)
                },
            )
        }
    }

    /**
     * Streams updates with [opts], replacing a running stream. The service
     * a background stream needs starts before the running stream changes,
     * so a failed start leaves it as it was.
     */
    private fun start(context: Context, opts: UpdateOptions) {
        if (opts.background) startService(context)
        removeUpdates()
        if (!opts.background) stopService(context)
        val request = LocationRequest.Builder(priority(opts.highAccuracy), opts.intervalMs)
            .setMinUpdateIntervalMillis(opts.fastestIntervalMs)
            .setMinUpdateDistanceMeters(opts.distanceFilter)
            .build()
        val callback = object : LocationCallback() {
            override fun onLocationResult(result: LocationResult) {
                result.lastLocation?.let { host.sendEvent(CHANNEL, payload(it)) }
            }
        }
        updates = callback
        @SuppressLint("MissingPermission") // requireUsable checked it.
        val task = client.requestLocationUpdates(request, callback, Looper.getMainLooper())
        task.addOnFailureListener { e ->
            // The stream is over; no notification outlives it.
            if (updates === callback) {
                updates = null
                stopService(context)
            }
            host.sendEventError(CHANNEL, e)
        }
    }

    private fun removeUpdates() {
        updates?.let { client.removeLocationUpdates(it) }
        updates = null
    }

    /**
     * Fails the call unless the build configured background updates
     * (always:) and the user granted Always.
     */
    private fun requireBackground(context: Context) {
        if (!DriftRuntimePermission.isDeclared(context, Manifest.permission.ACCESS_BACKGROUND_LOCATION)) {
            throw DriftCallError("background_not_configured", "background location updates need the location plugin's always: config")
        }
        if (ContextCompat.checkSelfPermission(context, Manifest.permission.ACCESS_BACKGROUND_LOCATION) != PackageManager.PERMISSION_GRANTED) {
            throw DriftCallError("always_denied", "background location updates need the Always location permission; request it first")
        }
    }

    /**
     * Starts DriftLocationService unless it runs. Android 12+ refuses to
     * start a foreground service while the app is in the background.
     */
    private fun startService(context: Context) {
        if (serviceRunning) return
        try {
            context.startForegroundService(
                Intent(context, DriftLocationService::class.java)
                    .putExtra(DriftLocationService.EXTRA_TITLE, checkNotNull(notificationTitle) { "drift location: no notification title in the always: config" })
                    .putExtra(DriftLocationService.EXTRA_TEXT, checkNotNull(notificationText) { "drift location: no notification text in the always: config" }),
            )
        } catch (e: ForegroundServiceStartNotAllowedException) {
            throw DriftCallError("background_start", "background location updates must start while the app is in the foreground")
        }
        serviceRunning = true
    }

    private fun stopService(context: Context) {
        if (!serviceRunning) return
        context.stopService(Intent(context, DriftLocationService::class.java))
        serviceRunning = false
    }

    /** Fails the call unless location is permitted and turned on. */
    private fun requireUsable(context: Context) {
        val permitted = listOf(Manifest.permission.ACCESS_FINE_LOCATION, Manifest.permission.ACCESS_COARSE_LOCATION)
            .any { ContextCompat.checkSelfPermission(context, it) == PackageManager.PERMISSION_GRANTED }
        if (!permitted) {
            throw DriftCallError("permission_denied", "location permission not granted; request it first")
        }
        if (!servicesEnabled(context)) {
            throw DriftCallError("location_disabled", "location services are turned off")
        }
    }

    private fun servicesEnabled(context: Context): Boolean {
        val manager = context.getSystemService(Context.LOCATION_SERVICE) as LocationManager
        return manager.isLocationEnabled
    }

    private fun priority(highAccuracy: Boolean) =
        if (highAccuracy) Priority.PRIORITY_HIGH_ACCURACY else Priority.PRIORITY_BALANCED_POWER_ACCURACY

    /** The wire form of a location (see the runtime package). */
    private fun payload(location: Location): Map<String, Any?> = mapOf(
        "latitude" to location.latitude,
        "longitude" to location.longitude,
        "altitude" to location.altitude,
        "accuracy" to location.accuracy.toDouble(),
        "heading" to location.bearing.toDouble(),
        "speed" to location.speed.toDouble(),
        "timestamp" to location.time,
        "isMocked" to location.isMock,
    )

    /** The update stream's options (the runtime package's UpdateOptions). */
    private data class UpdateOptions(
        val highAccuracy: Boolean = true,
        val intervalMs: Long = 10_000L,
        val fastestIntervalMs: Long = 5_000L,
        val distanceFilter: Float = 0f,
        val background: Boolean = false,
    ) {
        companion object {
            fun parse(args: Map<*, *>) = UpdateOptions(
                highAccuracy = args["highAccuracy"] as? Boolean ?: true,
                intervalMs = (args["intervalMs"] as? Number)?.toLong()?.takeIf { it > 0 } ?: 10_000L,
                fastestIntervalMs = (args["fastestIntervalMs"] as? Number)?.toLong()?.takeIf { it > 0 } ?: 5_000L,
                distanceFilter = (args["distanceFilter"] as? Number)?.toFloat() ?: 0f,
                background = args["background"] as? Boolean ?: false,
            )
        }
    }

    private companion object {
        const val CHANNEL = "drift/location"
    }
}
