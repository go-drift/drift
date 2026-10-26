/// DatePickerChannel.swift
/// Provides native date picker modal for Drift.

import DriftPluginAPI
import UIKit

// MARK: - Date Picker Handler

/// The drift/date_picker channel: `show` replies when the user picks a
/// date ({"year", "month" 1-12, "day"}) or cancels (nil). Dates cross the
/// channel as calendar days, so no time zone shifts them.
enum DatePickerHandler {
    static func start(host: DriftPluginHost) {
        host.registerChannel("drift/date_picker") { channel in
            channel.asyncMethod("show") { args, result in
                let params = args as? [String: Any] ?? [:]
                let shown = showDatePickerModal(
                    initialDate: calendarDay(params["initialDate"]) ?? Date(),
                    minDate: calendarDay(params["minDate"]),
                    maxDate: calendarDay(params["maxDate"])
                ) { selectedDate in
                    result.success(selectedDate.map { date -> [String: Int] in
                        let day = Calendar.current.dateComponents([.year, .month, .day], from: date)
                        return ["year": day.year ?? 0, "month": day.month ?? 0, "day": day.day ?? 0]
                    })
                }
                if !shown {
                    result.error(DriftCallError("platform_unavailable", "no screen to show the date picker over"))
                }
            }
        }
    }

    /// Local midnight of a wire calendar day ({year, month 1-12, day}).
    private static func calendarDay(_ value: Any?) -> Date? {
        guard let day = value as? [String: Any],
              let year = (day["year"] as? NSNumber)?.intValue,
              let month = (day["month"] as? NSNumber)?.intValue,
              let dayOfMonth = (day["day"] as? NSNumber)?.intValue else {
            return nil
        }
        return Calendar.current.date(from: DateComponents(year: year, month: month, day: dayOfMonth))
    }

    private static func showDatePickerModal(
        initialDate: Date,
        minDate: Date?,
        maxDate: Date?,
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
        let datePicker = UIDatePicker()
        datePicker.datePickerMode = .date
        datePicker.date = initialDate
        datePicker.preferredDatePickerStyle = .inline
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
        return true
    }
}
