/// DriftPermission.swift
///
/// Runtime permissions provided by plugins. Drift owns the request
/// mechanism: Go's platform.Permission calls the drift/permissions channel,
/// which looks the kind up among those registered with
/// `DriftPluginHost.registerPermission`. A plugin registers each kind whose
/// usage string it adds to Info.plist; its Go runtime exposes
/// `platform.NewPermission(kind)`.
///
/// Threading: Drift calls `status()` and `request(options:completion:)` on
/// the main thread.

/// A permission status, as Go's platform.PermissionResult names it.
public enum DriftPermissionStatus: String {
    case granted
    case denied
    case permanentlyDenied = "permanently_denied"
    case restricted
    case limited
    case notDetermined = "not_determined"
    case provisional
    case unknown
}

/// One kind of runtime permission, such as location or contacts.
public protocol DriftPermission: AnyObject {
    /// The current status, without prompting.
    func status() -> DriftPermissionStatus

    /// Prompts the user if that can change the status, then calls
    /// `completion` exactly once, from any thread, with the resulting
    /// status. Drift calls it only when `status()` is not `granted`,
    /// `permanentlyDenied` or `restricted`. `options` are the kind-specific
    /// options Go passed (empty for most kinds).
    ///
    /// When iOS shows no prompt (it already asked) or the user leaves the
    /// status unchanged, the system may send no callback; the permission
    /// must still complete, with the current status.
    func request(options: [String: Any], completion: @escaping (DriftPermissionStatus) -> Void)
}
