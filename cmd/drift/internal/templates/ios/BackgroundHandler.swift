/// BackgroundHandler.swift
/// Handles background task scheduling for the Drift platform channel.

import UIKit
import BackgroundTasks

enum BackgroundHandler {
    private static var pendingTasks: [String: TaskInfo] = [:]
    private static var isRegistered = false
    private static var bundleID: String {
        Bundle.main.bundleIdentifier ?? "com.drift"
    }

    // Use bundle ID prefix for task identifiers to match Info.plist
    private static var refreshTaskIdentifier: String {
        "\(bundleID).background.refresh"
    }
    private static var processingTaskIdentifier: String {
        "\(bundleID).background.processing"
    }

    // Track which task ID is currently scheduled for each identifier type
    // (iOS only allows one pending request per identifier)
    private static var currentRefreshTaskId: String?
    private static var currentProcessingTaskId: String?

    private struct TaskInfo {
        let id: String
        let taskType: String
        let data: [String: Any]
        let identifier: String  // which BGTask identifier this uses
    }

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "scheduleTask":
            return scheduleTask(args: args)
        case "cancelTask":
            return cancelTask(args: args)
        case "cancelAllTasks":
            return cancelAllTasks()
        case "cancelTasksByTag":
            return cancelTasksByTag(args: args)
        case "completeTask":
            return completeTask(args: args)
        case "isBackgroundRefreshAvailable":
            return isBackgroundRefreshAvailable()
        default:
            return (nil, NSError(domain: "Background", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    private static func scheduleTask(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let id = dict["id"] as? String else {
            return (nil, NSError(domain: "Background", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing id"]))
        }

        // Ensure tasks are registered before scheduling
        registerBackgroundTasks()

        let taskType = dict["taskType"] as? String ?? "one_time"
        let initialDelayMs = dict["initialDelayMs"] as? Int64 ?? 0
        let data = dict["data"] as? [String: Any] ?? [:]

        // Determine which identifier to use
        let identifier = taskType == "fetch" ? refreshTaskIdentifier : processingTaskIdentifier

        // iOS only allows one pending request per identifier - reject if one is already scheduled
        if taskType == "fetch" && currentRefreshTaskId != nil {
            return (nil, NSError(domain: "Background", code: 409, userInfo: [
                NSLocalizedDescriptionKey: "A fetch task is already scheduled (id: \(currentRefreshTaskId!)). Cancel it first or wait for it to complete."
            ]))
        }
        if taskType != "fetch" && currentProcessingTaskId != nil {
            return (nil, NSError(domain: "Background", code: 409, userInfo: [
                NSLocalizedDescriptionKey: "A processing task is already scheduled (id: \(currentProcessingTaskId!)). Cancel it first or wait for it to complete."
            ]))
        }

        // Calculate earliest begin date
        let earliestBegin = Date(timeIntervalSinceNow: TimeInterval(initialDelayMs) / 1000.0)

        // Store task info for when it executes
        pendingTasks[id] = TaskInfo(id: id, taskType: taskType, data: data, identifier: identifier)

        // Schedule based on task type
        switch taskType {
        case "fetch":
            scheduleFetchTask(id: id, earliestBegin: earliestBegin)
        default:
            scheduleProcessingTask(id: id, earliestBegin: earliestBegin)
        }

        return (nil, nil)
    }

    private static func scheduleFetchTask(id: String, earliestBegin: Date) {
        let request = BGAppRefreshTaskRequest(identifier: refreshTaskIdentifier)
        request.earliestBeginDate = earliestBegin

        do {
            try BGTaskScheduler.shared.submit(request)
            // Track which task ID is currently scheduled for this identifier
            currentRefreshTaskId = id
            NSLog("Scheduled background fetch task: %@ using identifier %@", id, refreshTaskIdentifier)
        } catch {
            NSLog("Failed to schedule background fetch task: %@", error.localizedDescription)
            sendTaskEvent(taskId: id, eventType: "failed", data: ["error": error.localizedDescription])
        }
    }

    private static func scheduleProcessingTask(id: String, earliestBegin: Date) {
        let request = BGProcessingTaskRequest(identifier: processingTaskIdentifier)
        request.earliestBeginDate = earliestBegin
        request.requiresNetworkConnectivity = false
        request.requiresExternalPower = false

        do {
            try BGTaskScheduler.shared.submit(request)
            // Track which task ID is currently scheduled for this identifier
            currentProcessingTaskId = id
            NSLog("Scheduled background processing task: %@ using identifier %@", id, processingTaskIdentifier)
        } catch {
            NSLog("Failed to schedule background processing task: %@", error.localizedDescription)
            sendTaskEvent(taskId: id, eventType: "failed", data: ["error": error.localizedDescription])
        }
    }

    private static func cancelTask(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let id = dict["id"] as? String else {
            return (nil, NSError(domain: "Background", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing id"]))
        }

        // Only cancel if this task is currently scheduled for its identifier
        if let taskInfo = pendingTasks[id] {
            if taskInfo.identifier == refreshTaskIdentifier && currentRefreshTaskId == id {
                BGTaskScheduler.shared.cancel(taskRequestWithIdentifier: refreshTaskIdentifier)
                currentRefreshTaskId = nil
            } else if taskInfo.identifier == processingTaskIdentifier && currentProcessingTaskId == id {
                BGTaskScheduler.shared.cancel(taskRequestWithIdentifier: processingTaskIdentifier)
                currentProcessingTaskId = nil
            }
        }
        pendingTasks.removeValue(forKey: id)

        return (nil, nil)
    }

    private static func cancelAllTasks() -> (Any?, Error?) {
        BGTaskScheduler.shared.cancelAllTaskRequests()
        pendingTasks.removeAll()
        currentRefreshTaskId = nil
        currentProcessingTaskId = nil
        return (nil, nil)
    }

    private static func cancelTasksByTag(args: Any?) -> (Any?, Error?) {
        // iOS doesn't have native tag support
        // We'd need to track tags separately to implement this
        return (nil, nil)
    }

    private static func completeTask(args: Any?) -> (Any?, Error?) {
        // Task completion is handled in the task handler
        // This is called from Go to signal completion
        return (nil, nil)
    }

    private static func isBackgroundRefreshAvailable() -> (Any?, Error?) {
        let status = UIApplication.shared.backgroundRefreshStatus
        let available = status == .available
        return (["available": available], nil)
    }

    // MARK: - Task Registration

    static func registerBackgroundTasks() {
        guard !isRegistered else { return }
        isRegistered = true

        // Register refresh task
        BGTaskScheduler.shared.register(forTaskWithIdentifier: refreshTaskIdentifier, using: nil) { task in
            handleBackgroundTask(task as! BGAppRefreshTask)
        }

        // Register processing task
        BGTaskScheduler.shared.register(forTaskWithIdentifier: processingTaskIdentifier, using: nil) { task in
            handleBackgroundTask(task as! BGProcessingTask)
        }

        NSLog("Registered background task identifiers: %@, %@", refreshTaskIdentifier, processingTaskIdentifier)
    }

    private static func handleBackgroundTask(_ task: BGTask) {
        // Determine the task ID based on the BGTask identifier
        let taskId: String
        if task.identifier == refreshTaskIdentifier {
            taskId = currentRefreshTaskId ?? "unknown"
            currentRefreshTaskId = nil
        } else if task.identifier == processingTaskIdentifier {
            taskId = currentProcessingTaskId ?? "unknown"
            currentProcessingTaskId = nil
        } else {
            taskId = "unknown"
        }

        NSLog("Executing background task: %@ (identifier: %@)", taskId, task.identifier)

        // Notify Go that task is starting
        sendTaskEvent(taskId: taskId, eventType: "started")

        // Set expiration handler
        task.expirationHandler = {
            sendTaskEvent(taskId: taskId, eventType: "expired")
            task.setTaskCompleted(success: false)
        }

        // Get stored task info
        let taskInfo = pendingTasks[taskId]

        // Signal completion after a short delay
        // In a real implementation, Go code would handle the work and call completeTask
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.0) {
            sendTaskEvent(taskId: taskId, eventType: "completed", data: [
                "success": true,
                "taskData": taskInfo?.data ?? [:]
            ])
            task.setTaskCompleted(success: true)
            pendingTasks.removeValue(forKey: taskId)
        }
    }

    // MARK: - Event Sending

    static func sendTaskEvent(taskId: String, eventType: String, data: [String: Any] = [:]) {
        PlatformChannelManager.shared.sendEvent(channel: "drift/background/events", data: [
            "taskId": taskId,
            "eventType": eventType,
            "data": data,
            "timestamp": Int64(Date().timeIntervalSince1970 * 1000)
        ])
    }
}
