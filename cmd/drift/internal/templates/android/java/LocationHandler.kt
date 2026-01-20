/**
 * LocationHandler.kt
 * Handles location services for the Drift platform channel.
 */
package {{.PackageName}}

import android.annotation.SuppressLint
import android.content.Context
import android.location.Location
import android.location.LocationManager
import android.os.Looper
import com.google.android.gms.location.*

object LocationHandler {
    private var fusedLocationClient: FusedLocationProviderClient? = null
    private var locationCallback: LocationCallback? = null
    private var isUpdating = false

    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        ensureClient(context)
        return when (method) {
            "getCurrentLocation" -> getCurrentLocation(context, args)
            "startUpdates" -> startUpdates(context, args)
            "stopUpdates" -> stopUpdates()
            "isEnabled" -> isEnabled(context)
            "getLastKnown" -> getLastKnown()
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun ensureClient(context: Context) {
        if (fusedLocationClient == null) {
            fusedLocationClient = LocationServices.getFusedLocationProviderClient(context)
        }
    }

    @SuppressLint("MissingPermission")
    private fun getCurrentLocation(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *> ?: emptyMap<String, Any>()
        val highAccuracy = argsMap["highAccuracy"] as? Boolean ?: true

        val client = fusedLocationClient ?: return Pair(null, IllegalStateException("Location client not initialized"))

        var result: Map<String, Any?>? = null
        var error: Exception? = null
        val latch = java.util.concurrent.CountDownLatch(1)

        val priority = if (highAccuracy) {
            Priority.PRIORITY_HIGH_ACCURACY
        } else {
            Priority.PRIORITY_BALANCED_POWER_ACCURACY
        }

        val request = CurrentLocationRequest.Builder()
            .setPriority(priority)
            .setMaxUpdateAgeMillis(10000)
            .build()

        client.getCurrentLocation(request, null)
            .addOnSuccessListener { location ->
                result = location?.let { locationToMap(it) }
                latch.countDown()
            }
            .addOnFailureListener { e ->
                error = e
                latch.countDown()
            }

        try {
            latch.await(30, java.util.concurrent.TimeUnit.SECONDS)
        } catch (e: InterruptedException) {
            return Pair(null, e)
        }

        return if (error != null) {
            Pair(null, error)
        } else {
            Pair(result, null)
        }
    }

    @SuppressLint("MissingPermission")
    private fun startUpdates(context: Context, args: Any?): Pair<Any?, Exception?> {
        if (isUpdating) {
            return Pair(null, null)
        }

        val argsMap = args as? Map<*, *> ?: emptyMap<String, Any>()
        val highAccuracy = argsMap["highAccuracy"] as? Boolean ?: true
        val distanceFilter = (argsMap["distanceFilter"] as? Number)?.toFloat() ?: 0f
        val intervalMs = (argsMap["intervalMs"] as? Number)?.toLong() ?: 10000L
        val fastestIntervalMs = (argsMap["fastestIntervalMs"] as? Number)?.toLong() ?: 5000L

        val client = fusedLocationClient ?: return Pair(null, IllegalStateException("Location client not initialized"))

        val priority = if (highAccuracy) {
            Priority.PRIORITY_HIGH_ACCURACY
        } else {
            Priority.PRIORITY_BALANCED_POWER_ACCURACY
        }

        val request = LocationRequest.Builder(priority, intervalMs)
            .setMinUpdateIntervalMillis(fastestIntervalMs)
            .setMinUpdateDistanceMeters(distanceFilter)
            .build()

        locationCallback = object : LocationCallback() {
            override fun onLocationResult(result: LocationResult) {
                result.lastLocation?.let { location ->
                    PlatformChannelManager.sendEvent("drift/location/updates", locationToMap(location))
                }
            }
        }

        client.requestLocationUpdates(request, locationCallback!!, Looper.getMainLooper())
        isUpdating = true

        return Pair(null, null)
    }

    private fun stopUpdates(): Pair<Any?, Exception?> {
        if (!isUpdating) {
            return Pair(null, null)
        }

        locationCallback?.let { callback ->
            fusedLocationClient?.removeLocationUpdates(callback)
        }
        locationCallback = null
        isUpdating = false

        return Pair(null, null)
    }

    private fun isEnabled(context: Context): Pair<Any?, Exception?> {
        val locationManager = context.getSystemService(Context.LOCATION_SERVICE) as LocationManager
        val gpsEnabled = locationManager.isProviderEnabled(LocationManager.GPS_PROVIDER)
        val networkEnabled = locationManager.isProviderEnabled(LocationManager.NETWORK_PROVIDER)
        return Pair(mapOf("enabled" to (gpsEnabled || networkEnabled)), null)
    }

    @SuppressLint("MissingPermission")
    private fun getLastKnown(): Pair<Any?, Exception?> {
        val client = fusedLocationClient ?: return Pair(null, null)

        var result: Map<String, Any?>? = null
        val latch = java.util.concurrent.CountDownLatch(1)

        client.lastLocation
            .addOnSuccessListener { location ->
                result = location?.let { locationToMap(it) }
                latch.countDown()
            }
            .addOnFailureListener {
                latch.countDown()
            }

        try {
            latch.await(5, java.util.concurrent.TimeUnit.SECONDS)
        } catch (e: InterruptedException) {
            // Ignore
        }

        return Pair(result, null)
    }

    private fun locationToMap(location: Location): Map<String, Any?> {
        return mapOf(
            "latitude" to location.latitude,
            "longitude" to location.longitude,
            "altitude" to location.altitude,
            "accuracy" to location.accuracy.toDouble(),
            "heading" to location.bearing.toDouble(),
            "speed" to location.speed.toDouble(),
            "timestamp" to location.time,
            "isMocked" to location.isFromMockProvider
        )
    }
}
