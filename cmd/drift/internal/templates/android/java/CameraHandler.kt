/**
 * CameraHandler.kt
 * Handles camera capture and gallery picking for the Drift platform channel.
 */
package {{.PackageName}}

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.graphics.BitmapFactory
import android.media.MediaMetadataRetriever
import android.net.Uri
import android.os.Environment
import android.provider.MediaStore
import android.provider.OpenableColumns
import androidx.core.content.FileProvider
import java.io.File
import java.text.SimpleDateFormat
import java.util.*

object CameraHandler {
    private const val CAPTURE_PHOTO_REQUEST = 9010
    private const val CAPTURE_VIDEO_REQUEST = 9011
    private const val PICK_GALLERY_REQUEST = 9012

    private var pendingPhotoUri: Uri? = null
    private var pendingPhotoPath: String? = null

    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        return when (method) {
            "capturePhoto" -> capturePhoto(context, args)
            "captureVideo" -> captureVideo(context, args)
            "pickFromGallery" -> pickFromGallery(context, args)
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun capturePhoto(context: Context, args: Any?): Pair<Any?, Exception?> {
        val activity = PlatformChannelManager.currentActivity()
            ?: return Pair(null, IllegalStateException("No activity available"))

        val argsMap = args as? Map<*, *> ?: emptyMap<String, Any>()
        val useFrontCamera = argsMap["useFrontCamera"] as? Boolean ?: false

        val intent = Intent(MediaStore.ACTION_IMAGE_CAPTURE)
        if (intent.resolveActivity(context.packageManager) == null) {
            return Pair(null, IllegalStateException("No camera app available"))
        }

        // Create file for photo
        val photoFile = createMediaFile(context, "IMG", ".jpg")
            ?: return Pair(null, IllegalStateException("Could not create photo file"))

        pendingPhotoPath = photoFile.absolutePath
        pendingPhotoUri = FileProvider.getUriForFile(
            context,
            "${context.packageName}.fileprovider",
            photoFile
        )

        intent.putExtra(MediaStore.EXTRA_OUTPUT, pendingPhotoUri)
        if (useFrontCamera) {
            intent.putExtra("android.intent.extras.CAMERA_FACING", 1)
            intent.putExtra("android.intent.extra.USE_FRONT_CAMERA", true)
        }

        activity.startActivityForResult(intent, CAPTURE_PHOTO_REQUEST)
        // Result will be delivered via drift/camera/result event channel
        return Pair(mapOf("pending" to true), null)
    }

    private fun captureVideo(context: Context, args: Any?): Pair<Any?, Exception?> {
        val activity = PlatformChannelManager.currentActivity()
            ?: return Pair(null, IllegalStateException("No activity available"))

        val argsMap = args as? Map<*, *> ?: emptyMap<String, Any>()
        val quality = argsMap["quality"] as? Number ?: 1
        val maxDurationMs = argsMap["maxDurationMs"] as? Number
        val useFrontCamera = argsMap["useFrontCamera"] as? Boolean ?: false

        val intent = Intent(MediaStore.ACTION_VIDEO_CAPTURE)
        if (intent.resolveActivity(context.packageManager) == null) {
            return Pair(null, IllegalStateException("No camera app available"))
        }

        val videoFile = createMediaFile(context, "VID", ".mp4")
            ?: return Pair(null, IllegalStateException("Could not create video file"))

        pendingPhotoPath = videoFile.absolutePath
        pendingPhotoUri = FileProvider.getUriForFile(
            context,
            "${context.packageName}.fileprovider",
            videoFile
        )

        intent.putExtra(MediaStore.EXTRA_OUTPUT, pendingPhotoUri)
        intent.putExtra(MediaStore.EXTRA_VIDEO_QUALITY, quality.toInt())
        maxDurationMs?.let {
            intent.putExtra(MediaStore.EXTRA_DURATION_LIMIT, (it.toLong() / 1000).toInt())
        }
        if (useFrontCamera) {
            intent.putExtra("android.intent.extras.CAMERA_FACING", 1)
        }

        activity.startActivityForResult(intent, CAPTURE_VIDEO_REQUEST)
        // Result will be delivered via drift/camera/result event channel
        return Pair(mapOf("pending" to true), null)
    }

    private fun pickFromGallery(context: Context, args: Any?): Pair<Any?, Exception?> {
        val activity = PlatformChannelManager.currentActivity()
            ?: return Pair(null, IllegalStateException("No activity available"))

        val argsMap = args as? Map<*, *> ?: emptyMap<String, Any>()
        val allowMultiple = argsMap["allowMultiple"] as? Boolean ?: false
        val mediaType = argsMap["mediaType"] as? String ?: "all"

        val mimeType = when (mediaType) {
            "image" -> "image/*"
            "video" -> "video/*"
            else -> "*/*"
        }

        val intent = Intent(Intent.ACTION_PICK).apply {
            type = mimeType
            if (allowMultiple) {
                putExtra(Intent.EXTRA_ALLOW_MULTIPLE, true)
            }
        }

        // Use GET_CONTENT as fallback
        if (intent.resolveActivity(context.packageManager) == null) {
            intent.action = Intent.ACTION_GET_CONTENT
            intent.addCategory(Intent.CATEGORY_OPENABLE)
        }

        activity.startActivityForResult(intent, PICK_GALLERY_REQUEST)
        // Result will be delivered via drift/camera/result event channel
        return Pair(mapOf("pending" to true), null)
    }

    private fun createMediaFile(context: Context, prefix: String, extension: String): File? {
        val timeStamp = SimpleDateFormat("yyyyMMdd_HHmmss", Locale.US).format(Date())
        val fileName = "${prefix}_$timeStamp"
        val storageDir = context.getExternalFilesDir(Environment.DIRECTORY_PICTURES)
        return File.createTempFile(fileName, extension, storageDir)
    }

    fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?, context: Context) {
        when (requestCode) {
            CAPTURE_PHOTO_REQUEST -> {
                if (resultCode == Activity.RESULT_OK) {
                    pendingPhotoPath?.let { path ->
                        val file = File(path)
                        if (file.exists()) {
                            val options = BitmapFactory.Options().apply {
                                inJustDecodeBounds = true
                            }
                            BitmapFactory.decodeFile(path, options)

                            val result = mapOf(
                                "type" to "capture",
                                "path" to path,
                                "mimeType" to "image/jpeg",
                                "width" to options.outWidth,
                                "height" to options.outHeight,
                                "size" to file.length(),
                                "durationMs" to 0L
                            )
                            sendResult(result)
                        } else {
                            sendCancelled("capture")
                        }
                    } ?: sendCancelled("capture")
                } else {
                    sendCancelled("capture")
                }
                pendingPhotoUri = null
                pendingPhotoPath = null
            }
            CAPTURE_VIDEO_REQUEST -> {
                if (resultCode == Activity.RESULT_OK) {
                    pendingPhotoPath?.let { path ->
                        val file = File(path)
                        if (file.exists()) {
                            val retriever = MediaMetadataRetriever()
                            try {
                                retriever.setDataSource(path)
                                val width = retriever.extractMetadata(MediaMetadataRetriever.METADATA_KEY_VIDEO_WIDTH)?.toIntOrNull() ?: 0
                                val height = retriever.extractMetadata(MediaMetadataRetriever.METADATA_KEY_VIDEO_HEIGHT)?.toIntOrNull() ?: 0
                                val duration = retriever.extractMetadata(MediaMetadataRetriever.METADATA_KEY_DURATION)?.toLongOrNull() ?: 0

                                val result = mapOf(
                                    "type" to "capture",
                                    "path" to path,
                                    "mimeType" to "video/mp4",
                                    "width" to width,
                                    "height" to height,
                                    "size" to file.length(),
                                    "durationMs" to duration
                                )
                                sendResult(result)
                            } finally {
                                retriever.release()
                            }
                        } else {
                            sendCancelled("capture")
                        }
                    } ?: sendCancelled("capture")
                } else {
                    sendCancelled("capture")
                }
                pendingPhotoUri = null
                pendingPhotoPath = null
            }
            PICK_GALLERY_REQUEST -> {
                if (resultCode == Activity.RESULT_OK) {
                    val mediaList = mutableListOf<Map<String, Any?>>()

                    data?.clipData?.let { clipData ->
                        for (i in 0 until clipData.itemCount) {
                            clipData.getItemAt(i).uri?.let { uri ->
                                getMediaInfo(context, uri)?.let { mediaList.add(it) }
                            }
                        }
                    } ?: data?.data?.let { uri ->
                        getMediaInfo(context, uri)?.let { mediaList.add(it) }
                    }

                    sendGalleryResult(mediaList)
                } else {
                    sendCancelled("gallery")
                }
            }
        }
    }

    private fun sendResult(result: Map<String, Any?>) {
        PlatformChannelManager.sendEvent("drift/camera/result", result)
    }

    private fun sendGalleryResult(mediaList: List<Map<String, Any?>>) {
        PlatformChannelManager.sendEvent("drift/camera/result", mapOf(
            "type" to "gallery",
            "media" to mediaList
        ))
    }

    private fun sendCancelled(requestType: String) {
        PlatformChannelManager.sendEvent("drift/camera/result", mapOf(
            "type" to requestType,
            "cancelled" to true
        ))
    }

    private fun getMediaInfo(context: Context, uri: Uri): Map<String, Any?>? {
        val mimeType = context.contentResolver.getType(uri) ?: return null

        return try {
            // Get size using ContentResolver query (reliable for content URIs)
            var size = 0L
            context.contentResolver.query(uri, arrayOf(OpenableColumns.SIZE), null, null, null)?.use { cursor ->
                if (cursor.moveToFirst()) {
                    val sizeIndex = cursor.getColumnIndex(OpenableColumns.SIZE)
                    if (sizeIndex >= 0 && !cursor.isNull(sizeIndex)) {
                        size = cursor.getLong(sizeIndex)
                    }
                }
            }

            val retriever = MediaMetadataRetriever()
            retriever.setDataSource(context, uri)

            val width = retriever.extractMetadata(MediaMetadataRetriever.METADATA_KEY_VIDEO_WIDTH)?.toIntOrNull()
                ?: retriever.extractMetadata(MediaMetadataRetriever.METADATA_KEY_IMAGE_WIDTH)?.toIntOrNull()
                ?: 0
            val height = retriever.extractMetadata(MediaMetadataRetriever.METADATA_KEY_VIDEO_HEIGHT)?.toIntOrNull()
                ?: retriever.extractMetadata(MediaMetadataRetriever.METADATA_KEY_IMAGE_HEIGHT)?.toIntOrNull()
                ?: 0
            val duration = retriever.extractMetadata(MediaMetadataRetriever.METADATA_KEY_DURATION)?.toLongOrNull() ?: 0

            retriever.release()

            mapOf(
                "path" to uri.toString(),
                "mimeType" to mimeType,
                "width" to width,
                "height" to height,
                "size" to size,
                "durationMs" to duration
            )
        } catch (e: Exception) {
            null
        }
    }
}
