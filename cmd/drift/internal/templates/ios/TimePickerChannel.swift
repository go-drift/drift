/// TimePickerChannel.swift
/// Provides native time picker modal for Drift.

import DriftPluginAPI
import UIKit

// MARK: - Time Picker Handler

/// The drift/time_picker channel: `show` replies when the user picks a
/// time ({"hour", "minute"}) or cancels (nil).
enum TimePickerHandler {
    static func start(host: DriftPluginHost) {
        host.registerChannel("drift/time_picker") { channel in
            channel.asyncMethod("show") { args, result in
                let params = args as? [String: Any] ?? [:]
                let hour = (params["hour"] as? NSNumber)?.intValue ?? 0
                let minute = (params["minute"] as? NSNumber)?.intValue ?? 0
                // nil: the system's 12/24-hour preference
                let is24Hour = params["is24Hour"] as? Bool
                let initialDate = Calendar.current.date(bySettingHour: hour, minute: minute, second: 0, of: Date()) ?? Date()
                let shown = showTimePickerModal(initialDate: initialDate, is24Hour: is24Hour) { selectedDate in
                    result.success(selectedDate.map { date -> [String: Int] in
                        let components = Calendar.current.dateComponents([.hour, .minute], from: date)
                        return ["hour": components.hour ?? 0, "minute": components.minute ?? 0]
                    })
                }
                if !shown {
                    result.error(DriftCallError("platform_unavailable", "no screen to show the time picker over"))
                }
            }
        }
    }

    private static func showTimePickerModal(
        initialDate: Date,
        is24Hour: Bool?,
        completion: @escaping (Date?) -> Void
    ) -> Bool {
        guard let windowScene = UIApplication.shared.connectedScenes.first as? UIWindowScene,
              let rootVC = windowScene.windows.first?.rootViewController else {
            return false
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
        timePicker.preferredDatePickerStyle = .wheels

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
        return true
    }
}
