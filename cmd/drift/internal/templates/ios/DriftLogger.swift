/// DriftLogger.swift
/// Centralized logging for Drift iOS apps using os_log.
/// Uses the app's bundle ID as subsystem for precise filtering.

import Foundation
import os

/// Centralized logging for Drift iOS apps.
/// Uses the app's bundle ID as subsystem for precise filtering with `drift log ios`.
enum DriftLog {
    private static let subsystem = Bundle.main.bundleIdentifier ?? "com.drift"

    static let general = Logger(subsystem: subsystem, category: "general")
    static let deeplink = Logger(subsystem: subsystem, category: "deeplink")
    static let background = Logger(subsystem: subsystem, category: "background")
    static let platform = Logger(subsystem: subsystem, category: "platform")
}
