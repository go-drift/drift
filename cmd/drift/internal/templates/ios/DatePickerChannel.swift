/// DatePickerChannel.swift
/// Provides native date picker modal for Drift.

import UIKit

// MARK: - Date Picker Handler

/// Handles date picker channel methods from Go.
enum DatePickerHandler {
    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        guard method == "show" else {
            return (nil, NSError(domain: "DatePicker", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }

        guard let params = args as? [String: Any] else {
            return (nil, NSError(domain: "DatePicker", code: 400, userInfo: [NSLocalizedDescriptionKey: "Invalid arguments"]))
        }

        // Parse initial date
        let initialTimestamp = (params["initialDate"] as? NSNumber)?.int64Value ?? Int64(Date().timeIntervalSince1970)
        let initialDate = Date(timeIntervalSince1970: TimeInterval(initialTimestamp))

        // Parse min/max dates
        var minDate: Date? = nil
        var maxDate: Date? = nil
        if let minTimestamp = (params["minDate"] as? NSNumber)?.int64Value {
            minDate = Date(timeIntervalSince1970: TimeInterval(minTimestamp))
        }
        if let maxTimestamp = (params["maxDate"] as? NSNumber)?.int64Value {
            maxDate = Date(timeIntervalSince1970: TimeInterval(maxTimestamp))
        }

        // Show picker on main thread and wait for result
        var result: Int64? = nil
        let semaphore = DispatchSemaphore(value: 0)

        DispatchQueue.main.async {
            showDatePickerModal(
                initialDate: initialDate,
                minDate: minDate,
                maxDate: maxDate
            ) { selectedDate in
                if let date = selectedDate {
                    result = Int64(date.timeIntervalSince1970)
                }
                semaphore.signal()
            }
        }

        // Wait for user selection (with timeout)
        let waitResult = semaphore.wait(timeout: .now() + .seconds(300))
        if waitResult == .timedOut {
            return (nil, NSError(domain: "DatePicker", code: 408, userInfo: [NSLocalizedDescriptionKey: "Picker timeout"]))
        }

        // Return result (nil means cancelled)
        if let timestamp = result {
            return (["timestamp": timestamp], nil)
        }
        return (nil, nil)
    }

    private static func showDatePickerModal(
        initialDate: Date,
        minDate: Date?,
        maxDate: Date?,
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
        let datePicker = UIDatePicker()
        datePicker.datePickerMode = .date
        datePicker.date = initialDate
        if #available(iOS 13.4, *) {
            datePicker.preferredDatePickerStyle = .inline
        }
        if let min = minDate {
            datePicker.minimumDate = min
        }
        if let max = maxDate {
            datePicker.maximumDate = max
        }

        // Create alert controller with picker
        let alertController = UIAlertController(title: "Select Date", message: nil, preferredStyle: .actionSheet)

        // Add date picker to alert
        datePicker.translatesAutoresizingMaskIntoConstraints = false
        alertController.view.addSubview(datePicker)

        // Calculate picker height - inline style needs more space
        // Extra padding to prevent selection circle clipping the separator
        let pickerHeight: CGFloat = 300

        NSLayoutConstraint.activate([
            datePicker.leadingAnchor.constraint(equalTo: alertController.view.leadingAnchor, constant: 8),
            datePicker.trailingAnchor.constraint(equalTo: alertController.view.trailingAnchor, constant: -8),
            datePicker.topAnchor.constraint(equalTo: alertController.view.topAnchor, constant: 45),
            datePicker.heightAnchor.constraint(equalToConstant: pickerHeight)
        ])

        // Set alert controller height to accommodate picker + actions
        // Title (~45) + picker + spacing for action buttons (~100)
        let totalHeight = 45 + pickerHeight + 120
        alertController.view.heightAnchor.constraint(equalToConstant: totalHeight).isActive = true

        // Add actions
        alertController.addAction(UIAlertAction(title: "Cancel", style: .cancel) { _ in
            completion(nil)
        })
        alertController.addAction(UIAlertAction(title: "Done", style: .default) { _ in
            completion(datePicker.date)
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
