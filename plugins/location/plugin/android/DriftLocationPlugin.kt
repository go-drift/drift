/**
 * DriftLocationPlugin.kt
 *
 * The location plugin's native half on Android, on the main thread, over
 * Google Play services' fused location provider: one-shot fixes, and
 * updates streamed to Go on `drift/location/updates`. Registers the
 * `location` permission (fine and coarse; coarse only reads as limited),
 * and `location_always` when the build declared background location.
 *
 * Errors Go maps to its sentinels (runtime package): `permission_denied`
 * and `location_disabled`.
 */
package com.drift.plugin.location

import android.Manifest
import android.annotation.SuppressLint
import android.content.Context
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
    private var updates: LocationCallback? = null

    override fun onRegister(host: DriftPluginHost) {
        this.host = host
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
            method("startUpdates") { args ->
                requireUsable(context)
                stopUpdates()
                val options = args as? Map<*, *> ?: emptyMap<String, Any>()
                val highAccuracy = options["highAccuracy"] as? Boolean ?: true
                val intervalMs = (options["intervalMs"] as? Number)?.toLong()?.takeIf { it > 0 } ?: 10_000L
                val fastestMs = (options["fastestIntervalMs"] as? Number)?.toLong()?.takeIf { it > 0 } ?: 5_000L
                val distance = (options["distanceFilter"] as? Number)?.toFloat() ?: 0f
                val request = LocationRequest.Builder(priority(highAccuracy), intervalMs)
                    .setMinUpdateIntervalMillis(fastestMs)
                    .setMinUpdateDistanceMeters(distance)
                    .build()
                val callback = object : LocationCallback() {
                    override fun onLocationResult(result: LocationResult) {
                        result.lastLocation?.let { host.sendEvent(UPDATES_CHANNEL, payload(it)) }
                    }
                }
                updates = callback
                @SuppressLint("MissingPermission") // requireUsable checked it.
                val task = client.requestLocationUpdates(request, callback, Looper.getMainLooper())
                task.addOnFailureListener { e ->
                    host.sendEventError(UPDATES_CHANNEL, "native_error", e.message ?: "location updates failed")
                }
                null
            }
            method("stopUpdates") { _ ->
                stopUpdates()
                null
            }
        }
    }

    private fun stopUpdates() {
        updates?.let { client.removeLocationUpdates(it) }
        updates = null
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

    private companion object {
        const val CHANNEL = "drift/location"
        const val UPDATES_CHANNEL = "drift/location/updates"
    }
}
