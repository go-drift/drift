/// TimePickerChannel.swift
/// Provides native time picker modal for Drift.

import UIKit

// MARK: - Time Picker Handler

/// Handles time picker channel methods from Go.
enum TimePickerHandler {
    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        guard method == "show" else {
            return (nil, NSError(domain: "TimePicker", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }

        guard let params = args as? [String: Any] else {
            return (nil, NSError(domain: "TimePicker", code: 400, userInfo: [NSLocalizedDescriptionKey: "Invalid arguments"]))
        }

        // Parse initial time
        let hour = (params["hour"] as? NSNumber)?.intValue ?? 0
        let minute = (params["minute"] as? NSNumber)?.intValue ?? 0

        // Parse is24Hour preference (nil = system default)
        let is24Hour: Bool? = params["is24Hour"] as? Bool

        // Create date from hour/minute using today as base
        let initialDate = Calendar.current.date(
            bySettingHour: hour,
            minute: minute,
            second: 0,
            of: Date()
        ) ?? Date()

        // Show picker on main thread and wait for result
        var resultHour: Int? = nil
        var resultMinute: Int? = nil
        let semaphore = DispatchSemaphore(value: 0)

        DispatchQueue.main.async {
            showTimePickerModal(initialDate: initialDate, is24Hour: is24Hour) { selectedDate in
                if let date = selectedDate {
                    let components = Calendar.current.dateComponents([.hour, .minute], from: date)
                    resultHour = components.hour
                    resultMinute = components.minute
                }
                semaphore.signal()
            }
        }

        // Wait for user selection (with timeout)
        let waitResult = semaphore.wait(timeout: .now() + .seconds(300))
        if waitResult == .timedOut {
            return (nil, NSError(domain: "TimePicker", code: 408, userInfo: [NSLocalizedDescriptionKey: "Picker timeout"]))
        }

        // Return result (nil means cancelled)
        if let hour = resultHour, let minute = resultMinute {
            return (["hour": hour, "minute": minute], nil)
        }
        return (nil, nil)
    }

    private static func showTimePickerModal(
        initialDate: Date,
        is24Hour: Bool?,
        completion: @escaping (Date?) -> Void
    ) {
        guard let windowScene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
              let rootVC = windowScene.windows.first?.rootViewController else {
            completion(nil)
            return
        }

        // Find topmost presented controller
        var topVC = rootVC
        while let presented = topVC.presentedViewController {
            topVC = presented
        }

        // Create date picker
        let timePicker = UIDatePicker()
        timePicker.datePickerMode = .time
        timePicker.date = initialDate
        if #available(iOS 13.4, *) {
            timePicker.preferredDatePickerStyle = .wheels
        }

        // Apply 12/24 hour format preference
        // nil = system default, true = 24-hour, false = 12-hour
        if let is24Hour = is24Hour {
            // Use locale to control time format display
            // en_GB uses 24-hour, en_US uses 12-hour
            timePicker.locale = Locale(identifier: is24Hour ? "en_GB" : "en_US")
        }

        // Create alert controller with picker
        let alertController = UIAlertController(title: "Select Time", message: nil, preferredStyle: .actionSheet)

        // Add time picker to alert
        timePicker.translatesAutoresizingMaskIntoConstraints = false
        alertController.view.addSubview(timePicker)

        // Wheels style picker height
        let pickerHeight: CGFloat = 200

        NSLayoutConstraint.activate([
            timePicker.leadingAnchor.constraint(equalTo: alertController.view.leadingAnchor, constant: 8),
            timePicker.trailingAnchor.constraint(equalTo: alertController.view.trailingAnchor, constant: -8),
            timePicker.topAnchor.constraint(equalTo: alertController.view.topAnchor, constant: 45),
            timePicker.heightAnchor.constraint(equalToConstant: pickerHeight)
        ])

        // Set alert controller height to accommodate picker + actions
        // Title (~45) + picker + spacing for action buttons (~100)
        let totalHeight = 45 + pickerHeight + 100
        alertController.view.heightAnchor.constraint(equalToConstant: totalHeight).isActive = true

        // Add actions
        alertController.addAction(UIAlertAction(title: "Cancel", style: .cancel) { _ in
            completion(nil)
        })
        alertController.addAction(UIAlertAction(title: "Done", style: .default) { _ in
            completion(timePicker.date)
        })

        // For iPad, configure popover
        if let popover = alertController.popoverPresentationController {
            popover.sourceView = topVC.view
            popover.sourceRect = CGRect(x: topVC.view.bounds.midX, y: topVC.view.bounds.midY, width: 0, height: 0)
            popover.permittedArrowDirections = []
        }

        topVC.present(alertController, animated: true)
    }
}
