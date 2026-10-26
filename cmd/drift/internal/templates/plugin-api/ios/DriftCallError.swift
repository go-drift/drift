/// DriftCallError.swift
///
/// The error a plugin fails a call with when Go should see a specific
/// code. Mirrors Android's com.drift.runner.DriftCallError.

import Foundation

/// A failure Go receives as a `*platform.ChannelError` with `code`,
/// `message` and `details`. Throw it from a method, or pass it to
/// `DriftResult.error(_:)` or `DriftPluginHost.sendEventError(_:_:)`.
///
/// Go matches `code` against the sentinels the channel declares
/// (`platform.ErrorCodes`) under `errors.Is`. Any other `Error` reaches Go
/// as `native_error`, with its NSError domain and code in `details`.
public struct DriftCallError: Error, CustomStringConvertible {
    /// The wire code, such as `permission_denied`.
    public let code: String
    public let message: String
    /// JSON-like extra data for Go (`ChannelError.Details`), or nil. A
    /// value that is not JSON-like fails the call with `not_encodable`.
    public let details: Any?

    public init(_ code: String, _ message: String, details: Any? = nil) {
        self.code = code
        self.message = message
        self.details = details
    }

    /// A call whose arguments are missing or malformed
    /// (`platform.ErrInvalidArguments`), as Kotlin's
    /// IllegalArgumentException is on Android.
    public static func invalidArguments(_ message: String) -> DriftCallError {
        DriftCallError("invalid_arguments", message)
    }

    public var description: String { "\(code): \(message)" }
}

extension DriftCallError: LocalizedError {
    public var errorDescription: String? { message }
}
