/**
 * BackgroundHandler.kt
 * Handles background task scheduling using WorkManager for the Drift platform channel.
 */
package {{.PackageName}}

import android.content.Context
import android.util.Log
import androidx.work.*
import java.util.concurrent.TimeUnit

object BackgroundHandler {
    private const val TAG = "DriftBackground"
    private const val WORK_TAG_PREFIX = "drift_task_"
    private const val DRIFT_ALL_TASKS_TAG = "drift_all_tasks"

    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        return when (method) {
            "scheduleTask" -> scheduleTask(context, args)
            "cancelTask" -> cancelTask(context, args)
            "cancelAllTasks" -> cancelAllTasks(context)
            "cancelTasksByTag" -> cancelTasksByTag(context, args)
            "completeTask" -> completeTask(args)
            "isBackgroundRefreshAvailable" -> isBackgroundRefreshAvailable()
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun scheduleTask(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))

        val id = argsMap["id"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing id"))
        val taskType = argsMap["taskType"] as? String ?: "one_time"
        val tag = argsMap["tag"] as? String
        val initialDelayMs = (argsMap["initialDelayMs"] as? Number)?.toLong() ?: 0
        val repeatIntervalMs = (argsMap["repeatIntervalMs"] as? Number)?.toLong() ?: 0
        @Suppress("UNCHECKED_CAST")
        val data = argsMap["data"] as? Map<String, Any?> ?: emptyMap()
        @Suppress("UNCHECKED_CAST")
        val constraints = argsMap["constraints"] as? Map<String, Any?> ?: emptyMap()

        val workConstraints = Constraints.Builder().apply {
            if (constraints["requiresNetwork"] == true) {
                setRequiredNetworkType(NetworkType.CONNECTED)
            }
            if (constraints["requiresUnmeteredNetwork"] == true) {
                setRequiredNetworkType(NetworkType.UNMETERED)
            }
            if (constraints["requiresCharging"] == true) {
                setRequiresCharging(true)
            }
            if (constraints["requiresIdle"] == true) {
                setRequiresDeviceIdle(true)
            }
            if (constraints["requiresStorageNotLow"] == true) {
                setRequiresStorageNotLow(true)
            }
            if (constraints["requiresBatteryNotLow"] == true) {
                setRequiresBatteryNotLow(true)
            }
        }.build()

        val inputData = Data.Builder()
            .putString("task_id", id)
            .putString("task_data", JsonCodec.encode(data).toString(Charsets.UTF_8))
            .build()

        val workManager = WorkManager.getInstance(context)

        when (taskType) {
            "periodic" -> {
                if (repeatIntervalMs < 15 * 60 * 1000) {
                    return Pair(null, IllegalArgumentException("Periodic interval must be at least 15 minutes"))
                }

                val request = PeriodicWorkRequestBuilder<DriftBackgroundWorker>(
                    repeatIntervalMs, TimeUnit.MILLISECONDS
                )
                    .setConstraints(workConstraints)
                    .setInitialDelay(initialDelayMs, TimeUnit.MILLISECONDS)
                    .setInputData(inputData)
                    .addTag(DRIFT_ALL_TASKS_TAG)
                    .addTag(WORK_TAG_PREFIX + id)
                    .apply { tag?.let { addTag(it) } }
                    .build()

                workManager.enqueueUniquePeriodicWork(
                    id,
                    ExistingPeriodicWorkPolicy.UPDATE,
                    request
                )
            }
            else -> {
                val request = OneTimeWorkRequestBuilder<DriftBackgroundWorker>()
                    .setConstraints(workConstraints)
                    .setInitialDelay(initialDelayMs, TimeUnit.MILLISECONDS)
                    .setInputData(inputData)
                    .addTag(DRIFT_ALL_TASKS_TAG)
                    .addTag(WORK_TAG_PREFIX + id)
                    .apply { tag?.let { addTag(it) } }
                    .build()

                workManager.enqueueUniqueWork(
                    id,
                    ExistingWorkPolicy.REPLACE,
                    request
                )
            }
        }

        Log.d(TAG, "Scheduled background task: $id (type=$taskType)")
        return Pair(null, null)
    }

    private fun cancelTask(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val id = argsMap["id"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing id"))

        WorkManager.getInstance(context).cancelUniqueWork(id)
        Log.d(TAG, "Cancelled background task: $id")
        return Pair(null, null)
    }

    private fun cancelAllTasks(context: Context): Pair<Any?, Exception?> {
        WorkManager.getInstance(context).cancelAllWorkByTag(DRIFT_ALL_TASKS_TAG)
        Log.d(TAG, "Cancelled all background tasks")
        return Pair(null, null)
    }

    private fun cancelTasksByTag(context: Context, args: Any?): Pair<Any?, Exception?> {
        val argsMap = args as? Map<*, *>
            ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val tag = argsMap["tag"] as? String
            ?: return Pair(null, IllegalArgumentException("Missing tag"))

        WorkManager.getInstance(context).cancelAllWorkByTag(tag)
        Log.d(TAG, "Cancelled background tasks with tag: $tag")
        return Pair(null, null)
    }

    private fun completeTask(args: Any?): Pair<Any?, Exception?> {
        // WorkManager handles completion automatically
        // This is mainly for iOS compatibility
        return Pair(null, null)
    }

    private fun isBackgroundRefreshAvailable(): Pair<Any?, Exception?> {
        // Background refresh is always available on Android with WorkManager
        return Pair(mapOf("available" to true), null)
    }

    fun sendTaskEvent(taskId: String, eventType: String, data: Map<String, Any?> = emptyMap()) {
        PlatformChannelManager.sendEvent("drift/background/events", mapOf(
            "taskId" to taskId,
            "eventType" to eventType,
            "data" to data,
            "timestamp" to System.currentTimeMillis()
        ))
    }
}

/**
 * Worker class for executing background tasks.
 */
class DriftBackgroundWorker(
    context: Context,
    params: WorkerParameters
) : Worker(context, params) {

    override fun doWork(): Result {
        val taskId = inputData.getString("task_id") ?: return Result.failure()
        val taskDataJson = inputData.getString("task_data")

        Log.d("DriftBackground", "Executing background task: $taskId")

        // Notify Go that task is starting
        BackgroundHandler.sendTaskEvent(taskId, "started")

        return try {
            // The actual work would be handled by Go code listening to the event
            // For now, we just signal that the task ran
            BackgroundHandler.sendTaskEvent(taskId, "completed", mapOf(
                "success" to true
            ))
            Result.success()
        } catch (e: Exception) {
            Log.e("DriftBackground", "Background task failed: $taskId", e)
            BackgroundHandler.sendTaskEvent(taskId, "failed", mapOf(
                "error" to e.message
            ))
            Result.failure()
        }
    }
}
