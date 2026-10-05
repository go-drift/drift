/**
 * StorageHandler.kt
 * The drift/storage channel: document pickers and file system access.
 *
 * The pickers reply when the user finishes ({"files": [...]} or
 * {"path": ...}) or cancels (null); one picker at a time, a second fails
 * with `storage_busy`. Paths may be content URIs. File I/O runs off the
 * main thread. File contents cross the channel as base64.
 */
package {{.PackageName}}

import android.content.Context
import android.net.Uri
import android.os.Environment
import android.provider.DocumentsContract
import android.provider.OpenableColumns
import android.util.Base64
import android.webkit.MimeTypeMap
import androidx.activity.result.contract.ActivityResultContract
import androidx.activity.result.contract.ActivityResultContracts
import androidx.fragment.app.FragmentActivity
import com.drift.runner.DriftCallError
import com.drift.runner.DriftPluginHost
import com.drift.runner.DriftResult
import com.drift.runner.launchForResult
import java.io.File
import java.util.concurrent.Executors

object StorageHandler {
    private val io = Executors.newSingleThreadExecutor { Thread(it, "drift-storage-io") }

    /** Whether a picker is open. Main thread only. */
    private var picking = false

    fun start(host: DriftPluginHost) {
        val context = host.context
        host.registerChannel("drift/storage") {
            asyncMethod("pickFile") { args, result -> pickFile(context, args, result) }
            asyncMethod("pickDirectory") { _, result ->
                pick(result, ActivityResultContracts.OpenDocumentTree(), null) { uri ->
                    uri?.let { mapOf("path" to it.toString()) }
                }
            }
            asyncMethod("saveFile") { args, result -> saveFile(context, args, result) }
            asyncMethod("readFile") { args, result ->
                background(result) {
                    val path = path(args)
                    val bytes = if (isContentUri(path)) {
                        context.contentResolver.openInputStream(Uri.parse(path))?.use { it.readBytes() }
                            ?: throw DriftCallError("io_error", "cannot open $path")
                    } else {
                        File(path).readBytes()
                    }
                    mapOf("data" to Base64.encodeToString(bytes, Base64.NO_WRAP))
                }
            }
            asyncMethod("writeFile") { args, result ->
                background(result) {
                    val path = path(args)
                    val bytes = data(args)
                    if (isContentUri(path)) {
                        context.contentResolver.openOutputStream(Uri.parse(path), "wt")?.use { it.write(bytes) }
                            ?: throw DriftCallError("io_error", "cannot open $path")
                    } else {
                        File(path).writeBytes(bytes)
                    }
                    null
                }
            }
            asyncMethod("deleteFile") { args, result ->
                background(result) {
                    val path = path(args)
                    val deleted = if (isContentUri(path)) {
                        DocumentsContract.deleteDocument(context.contentResolver, Uri.parse(path))
                    } else {
                        File(path).delete()
                    }
                    if (!deleted) throw DriftCallError("io_error", "cannot delete $path")
                    null
                }
            }
            asyncMethod("getFileInfo") { args, result ->
                background(result) {
                    val path = path(args)
                    if (isContentUri(path)) contentInfo(context, Uri.parse(path)) else fileInfo(path)
                }
            }
            method("getAppDirectory") { args ->
                val path = when ((args as? Map<*, *>)?.get("directory")) {
                    "documents" -> context.getExternalFilesDir(Environment.DIRECTORY_DOCUMENTS)?.absolutePath
                    "cache" -> context.cacheDir.absolutePath
                    "temp" -> File(context.cacheDir, "temp").apply { mkdirs() }.absolutePath
                    "support" -> context.filesDir.absolutePath
                    else -> throw IllegalArgumentException("drift/storage: missing or unknown directory")
                }
                mapOf("path" to path)
            }
        }
    }

    private fun pickFile(context: Context, args: Any?, result: DriftResult) {
        val params = args as? Map<*, *> ?: emptyMap<String, Any?>()
        val types = (params["allowedTypes"] as? List<*>)?.filterIsInstance<String>()
            ?.takeIf { it.isNotEmpty() }?.toTypedArray() ?: arrayOf("*/*")
        val reply = { uris: List<Uri>? ->
            uris?.let { picked -> mapOf("files" to picked.map { contentInfo(context, it) }) }
        }
        if (params["allowMultiple"] == true) {
            pick(result, ActivityResultContracts.OpenMultipleDocuments(), types) { uris ->
                reply(uris?.takeIf { it.isNotEmpty() })
            }
        } else {
            pick(result, ActivityResultContracts.OpenDocument(), types) { uri -> reply(uri?.let { listOf(it) }) }
        }
    }

    private fun saveFile(context: Context, args: Any?, result: DriftResult) {
        val params = args as? Map<*, *> ?: emptyMap<String, Any?>()
        val bytes = data(args)
        val mimeType = (params["mimeType"] as? String)?.takeIf { it.isNotEmpty() } ?: "application/octet-stream"
        val name = (params["suggestedName"] as? String)?.takeIf { it.isNotEmpty() } ?: "file"
        pick(result, ActivityResultContracts.CreateDocument(mimeType), name, offMain = true) { uri ->
            uri?.let {
                context.contentResolver.openOutputStream(it, "wt")?.use { out -> out.write(bytes) }
                    ?: throw DriftCallError("io_error", "cannot open $it")
                mapOf("path" to it.toString())
            }
        }
    }

    /**
     * Launches a picker, unless one is open, and replies with [reply]'s
     * value for its output (null when the user canceled), computed on the
     * I/O thread when [offMain].
     */
    private fun <I, O> pick(
        result: DriftResult,
        contract: ActivityResultContract<I, O>,
        input: I,
        offMain: Boolean = false,
        reply: (O?) -> Any?,
    ) {
        if (picking) throw DriftCallError("storage_busy", "a storage picker is already open")
        val activity = PlatformChannelManager.currentActivity() as? FragmentActivity
            ?: throw DriftCallError("platform_unavailable", "no screen to show the picker over")
        picking = true
        try {
            launchForResult(activity, contract, input) { output ->
                picking = false
                if (offMain) background(result) { reply(output) } else respond(result) { reply(output) }
            }
        } catch (e: Exception) {
            picking = false
            throw e
        }
    }

    private fun background(result: DriftResult, work: () -> Any?) {
        io.execute { respond(result, work) }
    }

    private fun respond(result: DriftResult, work: () -> Any?) {
        try {
            result.success(work())
        } catch (e: Exception) {
            result.error(e)
        }
    }

    private fun contentInfo(context: Context, uri: Uri): Map<String, Any?> {
        var name = ""
        var size = 0L
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
            "mimeType" to (context.contentResolver.getType(uri) ?: ""),
            "isDirectory" to false,
            "lastModified" to 0L,
        )
    }

    private fun fileInfo(path: String): Map<String, Any?> {
        val file = File(path)
        if (!file.exists()) throw DriftCallError("io_error", "no file at $path")
        return mapOf(
            "name" to file.name,
            "path" to file.absolutePath,
            "size" to file.length(),
            "mimeType" to (MimeTypeMap.getSingleton().getMimeTypeFromExtension(file.extension) ?: ""),
            "isDirectory" to file.isDirectory,
            "lastModified" to file.lastModified(),
        )
    }

    private fun isContentUri(path: String) = path.startsWith("content://")

    private fun path(args: Any?): String =
        (args as? Map<*, *>)?.get("path") as? String
            ?: throw IllegalArgumentException("drift/storage: missing path")

    /** The base64 `data` argument. */
    private fun data(args: Any?): ByteArray {
        val encoded = (args as? Map<*, *>)?.get("data") as? String
            ?: throw IllegalArgumentException("drift/storage: missing data")
        return Base64.decode(encoded, Base64.DEFAULT)
    }
}
