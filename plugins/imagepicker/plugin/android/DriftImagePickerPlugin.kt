/**
 * DriftImagePickerPlugin.kt
 *
 * The image picker plugin's native half on Android, on the main thread:
 * the system camera app (TakePicture) and the system photo picker
 * (PickVisualMedia), neither of which needs a permission, started through
 * the Activity's activityResultRegistry. The camera writes through Drift's
 * FileProvider (`<package>.fileprovider`, the cache directory). Picked
 * images are copied to the cache directory off the main thread. One picker
 * at a time.
 *
 * If any plugin declares CAMERA, Android refuses TakePicture until it is
 * granted, so capture asks for it first.
 *
 * Errors Go maps to its sentinels (runtime package): `busy`,
 * `camera_unavailable`, `permission_denied`, `no_activity`.
 */
package com.drift.plugin.imagepicker

import android.Manifest
import android.content.ActivityNotFoundException
import android.content.Context
import android.content.pm.PackageManager
import android.graphics.BitmapFactory
import android.net.Uri
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.core.content.ContextCompat
import androidx.core.content.FileProvider
import androidx.fragment.app.FragmentActivity
import com.drift.runner.DriftActivityBinding
import com.drift.runner.DriftCallError
import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost
import com.drift.runner.DriftResult
import com.drift.runner.DriftRuntimePermission
import com.drift.runner.launchForResult
import java.io.File
import java.util.UUID
import kotlin.concurrent.thread

class DriftImagePickerPlugin : DriftPlugin {
    private lateinit var context: Context
    private var activity: FragmentActivity? = null
    /** The open picker's reply, null when none is open. */
    private var pending: DriftResult? = null

    override fun onRegister(host: DriftPluginHost) {
        context = host.context
        host.registerChannel(CHANNEL) {
            asyncMethod("capture") { args, result ->
                val front = (args as? Map<*, *>)?.get("frontCamera") as? Boolean ?: false
                begin(result) { activity -> capture(activity, front) }
            }
            asyncMethod("pick") { args, result ->
                val multiple = (args as? Map<*, *>)?.get("multiple") as? Boolean ?: false
                begin(result) { activity -> pick(activity, multiple) }
            }
        }
    }

    override fun onAttach(binding: DriftActivityBinding) {
        activity = binding.activity
    }

    override fun onDetach() {
        activity = null
    }

    /** Opens a picker for result, or fails it. */
    private fun begin(result: DriftResult, open: (FragmentActivity) -> Unit) {
        if (pending != null) throw DriftCallError("busy", "an image picker is already open")
        val activity = activity ?: throw DriftCallError("no_activity", "no Activity to show the picker over")
        pending = result
        try {
            open(activity)
        } catch (e: Exception) {
            pending = null
            throw e
        }
    }

    private fun finish(reply: (DriftResult) -> Unit) {
        val result = pending ?: return
        pending = null
        reply(result)
    }

    private fun capture(activity: FragmentActivity, front: Boolean) {
        if (!context.packageManager.hasSystemFeature(PackageManager.FEATURE_CAMERA_ANY)) {
            throw DriftCallError("camera_unavailable", "this device has no camera")
        }
        val needsGrant = DriftRuntimePermission.isDeclared(context, Manifest.permission.CAMERA) &&
            ContextCompat.checkSelfPermission(context, Manifest.permission.CAMERA) != PackageManager.PERMISSION_GRANTED
        if (needsGrant) {
            launchForResult(activity, ActivityResultContracts.RequestPermission(), Manifest.permission.CAMERA) { granted ->
                if (granted == true) {
                    takePicture(activity, front)
                } else {
                    finish { it.error(DriftCallError("permission_denied", "camera access denied")) }
                }
            }
        } else {
            takePicture(activity, front)
        }
    }

    private fun takePicture(activity: FragmentActivity, front: Boolean) {
        val file = newCacheFile("jpg")
        val uri = FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", file)
        val contract = object : ActivityResultContracts.TakePicture() {
            override fun createIntent(context: Context, input: Uri) = super.createIntent(context, input).apply {
                // Hints; not every camera app honours them.
                if (front) {
                    putExtra("android.intent.extras.CAMERA_FACING", 1)
                    putExtra("android.intent.extras.LENS_FACING_FRONT", 1)
                    putExtra("android.intent.extra.USE_FRONT_CAMERA", true)
                }
            }
        }
        try {
            launchForResult(activity, contract, uri) { saved ->
                finish { result ->
                    if (saved == true && file.length() > 0) {
                        result.success(mapOf("image" to describe(file, "image/jpeg")))
                    } else {
                        file.delete()
                        result.success(mapOf("image" to null))
                    }
                }
            }
        } catch (_: ActivityNotFoundException) {
            finish { it.error(DriftCallError("camera_unavailable", "no camera app is installed")) }
        }
    }

    private fun pick(activity: FragmentActivity, multiple: Boolean) {
        val request = PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)
        if (multiple) {
            launchForResult(activity, ActivityResultContracts.PickMultipleVisualMedia(), request) { uris ->
                copyPicked(uris.orEmpty())
            }
        } else {
            launchForResult(activity, ActivityResultContracts.PickVisualMedia(), request) { uri ->
                copyPicked(listOfNotNull(uri))
            }
        }
    }

    /** Copies the picked images to the cache, off the main thread, and replies. */
    private fun copyPicked(uris: List<Uri>) {
        val result = pending ?: return
        pending = null
        thread(name = "DriftImagePicker") {
            try {
                val images = uris.map { uri ->
                    val type = context.contentResolver.getType(uri) ?: "image/jpeg"
                    val file = newCacheFile(type.substringAfter('/', "jpg"))
                    val input = context.contentResolver.openInputStream(uri)
                        ?: throw IllegalStateException("could not open $uri")
                    input.use { stream -> file.outputStream().use { stream.copyTo(it) } }
                    describe(file, type)
                }
                result.success(mapOf("images" to images))
            } catch (e: Exception) {
                result.error(e)
            }
        }
    }

    private fun newCacheFile(extension: String): File {
        val dir = File(context.cacheDir, "drift_images").apply { mkdirs() }
        return File(dir, "${UUID.randomUUID()}.$extension")
    }

    private fun describe(file: File, mimeType: String): Map<String, Any> {
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeFile(file.absolutePath, bounds)
        return mapOf(
            "path" to file.absolutePath,
            "mimeType" to mimeType,
            "width" to bounds.outWidth,
            "height" to bounds.outHeight,
            "size" to file.length(),
        )
    }

    private companion object {
        const val CHANNEL = "drift/imagepicker"
    }
}
