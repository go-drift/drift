/**
 * StorageHandler.kt
 * Handles file system access and document picking for the Drift platform channel.
 */
package {{.PackageName}}

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Environment
import android.provider.DocumentsContract
import android.provider.OpenableColumns
import android.webkit.MimeTypeMap
import java.io.File

object StorageHandler {
    private const val PICK_FILE_REQUEST = 9002
    private const val PICK_DIRECTORY_REQUEST = 9003
    private const val SAVE_FILE_REQUEST = 9004

    private var pendingSaveData: ByteArray? = null
    private var pendingRequestType: String? = null

    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        return when (method) {
            "pickFile" -> pickFile(context, args)
            "pickDirectory" -> pickDirectory(context)
            "saveFile" -> saveFile(context, args)
            "readFile" -> readFile(context, args)
            "writeFile" -> writeFile(context, args)
            "deleteFile" -> deleteFile(context, args)
            "getFileInfo" -> getFileInfo(context, args)
            "getAppDirectory" -> getAppDirectory(context, args)
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun pickFile(context: Context, args: Any?): Pair<Any?, Exception?> {
        val activity = PlatformChannelManager.currentActivity()
            ?: return Pair(null, IllegalStateException("No activity available"))

        val argsMap = args as? Map<*, *> ?: emptyMap<String, Any>()
        val allowMultiple = argsMap["allowMultiple"] as? Boolean ?: false
        @Suppress("UNCHECKED_CAST")
        val allowedTypes = argsMap["allowedTypes"] as? List<String> ?: listOf("*/*")

        pendingRequestType = "pickFile"

        val intent = Intent(Intent.ACTION_OPEN_DOCUMENT).apply {
            addCategory(Intent.CATEGORY_OPENABLE)
            type = if (allowedTypes.size == 1) allowedTypes[0] else "*/*"
            if (allowedTypes.size > 1) {
                putExtra(Intent.EXTRA_MIME_TYPES, allowedTypes.toTypedArray())
            }
            if (allowMultiple) {
                putExtra(Intent.EXTRA_ALLOW_MULTIPLE, true)
            }
        }

        activity.startActivityForResult(intent, PICK_FILE_REQUEST)
        // Result will be delivered via drift/storage/result event channel
        return Pair(mapOf("pending" to true), null)
    }

    private fun pickDirectory(context: Context): Pair<Any?, Exception?> {
        val activity = PlatformChannelManager.currentActivity()
            ?: return Pair(null, IllegalStateException("No activity available"))

        pendingRequestType = "pickDirectory"

        val intent = Intent(Intent.ACTION_OPEN_DOCUMENT_TREE)
        activity.startActivityForResult(intent, PICK_DIRECTORY_REQUEST)
        // Result will be delivered via drift/storage/result event channel
        return Pair(mapOf("pending" to true), null)
    }

    private fun saveFile(context: Context, args: Any?): Pair<Any?, Exception?> {
        val activity = PlatformChannelManager.currentActivity()
            ?: return Pair(null, IllegalStateException("No activity available"))

        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))

        val suggestedName = argsMap["suggestedName"] as? String ?: "file"
        val mimeType = argsMap["mimeType"] as? String ?: "*/*"

        @Suppress("UNCHECKED_CAST")
        val data = when (val dataArg = argsMap["data"]) {
            is ByteArray -> dataArg
            is List<*> -> (dataArg as List<Number>).map { it.toByte() }.toByteArray()
            else -> null
        }

        if (data == null) {
            return Pair(null, IllegalArgumentException("Missing data"))
        }

        pendingSaveData = data
        pendingRequestType = "saveFile"

        val intent = Intent(Intent.ACTION_CREATE_DOCUMENT).apply {
            addCategory(Intent.CATEGORY_OPENABLE)
            type = mimeType
            putExtra(Intent.EXTRA_TITLE, suggestedName)
        }

        activity.startActivityForResult(intent, SAVE_FILE_REQUEST)
        // Result will be delivered via drift/storage/result event channel
        return Pair(mapOf("pending" to true), null)
    }

    private fun readFile(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val path = argsMap["path"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing path"))

        return try {
            val data = if (path.startsWith("content://")) {
                val uri = Uri.parse(path)
                context.contentResolver.openInputStream(uri)?.use { it.readBytes() }
            } else {
                File(path).readBytes()
            }
            Pair(mapOf("data" to data), null)
        } catch (e: Exception) {
            Pair(null, e)
        }
    }

    private fun writeFile(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val path = argsMap["path"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing path"))

        @Suppress("UNCHECKED_CAST")
        val data = when (val dataArg = argsMap["data"]) {
            is ByteArray -> dataArg
            is List<*> -> (dataArg as List<Number>).map { it.toByte() }.toByteArray()
            else -> null
        } ?: return Pair(null, IllegalArgumentException("Missing data"))

        return try {
            if (path.startsWith("content://")) {
                val uri = Uri.parse(path)
                context.contentResolver.openOutputStream(uri)?.use { it.write(data) }
            } else {
                File(path).writeBytes(data)
            }
            Pair(null, null)
        } catch (e: Exception) {
            Pair(null, e)
        }
    }

    private fun deleteFile(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val path = argsMap["path"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing path"))

        return try {
            if (path.startsWith("content://")) {
                val uri = Uri.parse(path)
                DocumentsContract.deleteDocument(context.contentResolver, uri)
            } else {
                File(path).delete()
            }
            Pair(null, null)
        } catch (e: Exception) {
            Pair(null, e)
        }
    }

    private fun getFileInfo(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val path = argsMap["path"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing path"))

        return try {
            val info = if (path.startsWith("content://")) {
                getContentUriInfo(context, Uri.parse(path))
            } else {
                getFilePathInfo(path)
            }
            Pair(info, null)
        } catch (e: Exception) {
            Pair(null, e)
        }
    }

    private fun getAppDirectory(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val directory = argsMap["directory"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing directory"))

        val path = when (directory) {
            "documents" -> context.getExternalFilesDir(Environment.DIRECTORY_DOCUMENTS)?.absolutePath
            "cache" -> context.cacheDir.absolutePath
            "temp" -> context.cacheDir.absolutePath + "/temp"
            "support" -> context.filesDir.absolutePath
            else -> context.filesDir.absolutePath
        }

        return Pair(mapOf("path" to path), null)
    }

    private fun getContentUriInfo(context: Context, uri: Uri): Map<String, Any?> {
        var name = ""
        var size = 0L
        var mimeType = context.contentResolver.getType(uri) ?: ""

        context.contentResolver.query(uri, null, null, null, null)?.use { cursor ->
            if (cursor.moveToFirst()) {
                val nameIndex = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
                val sizeIndex = cursor.getColumnIndex(OpenableColumns.SIZE)
                if (nameIndex >= 0) name = cursor.getString(nameIndex) ?: ""
                if (sizeIndex >= 0 && !cursor.isNull(sizeIndex)) size = cursor.getLong(sizeIndex)
            }
        }

        return mapOf(
            "name" to name,
            "path" to uri.toString(),
            "uri" to uri.toString(),
            "size" to size,
            "mimeType" to mimeType,
            "isDirectory" to false,
            "lastModified" to 0L
        )
    }

    private fun getFilePathInfo(path: String): Map<String, Any?> {
        val file = File(path)
        val mimeType = MimeTypeMap.getSingleton()
            .getMimeTypeFromExtension(file.extension) ?: ""

        return mapOf(
            "name" to file.name,
            "path" to file.absolutePath,
            "size" to file.length(),
            "mimeType" to mimeType,
            "isDirectory" to file.isDirectory,
            "lastModified" to file.lastModified()
        )
    }

    fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?, context: Context) {
        when (requestCode) {
            PICK_FILE_REQUEST -> {
                if (resultCode == Activity.RESULT_OK) {
                    val files = mutableListOf<Map<String, Any?>>()
                    data?.clipData?.let { clipData ->
                        for (i in 0 until clipData.itemCount) {
                            clipData.getItemAt(i).uri?.let { uri ->
                                files.add(getContentUriInfo(context, uri))
                            }
                        }
                    } ?: data?.data?.let { uri ->
                        files.add(getContentUriInfo(context, uri))
                    }
                    sendPickFileResult(files)
                } else {
                    sendCancelled("pickFile")
                }
                pendingRequestType = null
            }
            PICK_DIRECTORY_REQUEST -> {
                if (resultCode == Activity.RESULT_OK) {
                    val path = data?.data?.toString()
                    sendPickDirectoryResult(path)
                } else {
                    sendCancelled("pickDirectory")
                }
                pendingRequestType = null
            }
            SAVE_FILE_REQUEST -> {
                if (resultCode == Activity.RESULT_OK) {
                    data?.data?.let { uri ->
                        pendingSaveData?.let { saveData ->
                            try {
                                context.contentResolver.openOutputStream(uri)?.use {
                                    it.write(saveData)
                                }
                                sendSaveFileResult(uri.toString())
                            } catch (e: Exception) {
                                sendError("saveFile", e.message ?: "Failed to save file")
                            }
                        } ?: sendError("saveFile", "No data to save")
                    } ?: sendCancelled("saveFile")
                } else {
                    sendCancelled("saveFile")
                }
                pendingSaveData = null
                pendingRequestType = null
            }
        }
    }

    private fun sendPickFileResult(files: List<Map<String, Any?>>) {
        PlatformChannelManager.sendEvent("drift/storage/result", mapOf(
            "type" to "pickFile",
            "files" to files
        ))
    }

    private fun sendPickDirectoryResult(path: String?) {
        PlatformChannelManager.sendEvent("drift/storage/result", mapOf(
            "type" to "pickDirectory",
            "path" to path
        ))
    }

    private fun sendSaveFileResult(path: String) {
        PlatformChannelManager.sendEvent("drift/storage/result", mapOf(
            "type" to "saveFile",
            "path" to path
        ))
    }

    private fun sendCancelled(requestType: String) {
        PlatformChannelManager.sendEvent("drift/storage/result", mapOf(
            "type" to requestType,
            "cancelled" to true
        ))
    }

    private fun sendError(requestType: String, message: String) {
        PlatformChannelManager.sendEvent("drift/storage/result", mapOf(
            "type" to requestType,
            "error" to message
        ))
    }
}
