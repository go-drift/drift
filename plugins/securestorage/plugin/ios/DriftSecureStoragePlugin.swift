/// DriftSecureStoragePlugin.swift
///
/// The secure storage plugin's native half on iOS: generic-password
/// Keychain items, one per key, in a service (namespace). Every call runs
/// on a serial background queue, since Keychain calls block (a Face ID
/// prompt blocks SecItemCopyMatching until the user answers). A value
/// stored with requireBiometric gets access control `.biometryCurrentSet`,
/// so the Keychain itself demands biometrics to read it.
///
/// Errors Go maps to its sentinels (runtime package): `auth_failed`,
/// `auth_canceled`, `biometrics_unavailable`,
/// `biometrics_not_configured`, `locked`.

import DriftPluginAPI
import Foundation
import LocalAuthentication
import Security

/// Public, with a public initializer: the app's generated registrant
/// creates it from outside this module (DriftPlugin_securestorage).
public final class DriftSecureStoragePlugin: DriftPlugin {
    private static let channel = "drift/securestorage"
    private let queue = DispatchQueue(label: "drift.securestorage")

    public init() {}

    public func register(host: DriftPluginHost) {
        host.registerChannel(Self.channel) { channel in
            for (name, body) in [
                ("set", Self.set),
                ("get", Self.get),
                ("delete", Self.delete),
                ("contains", Self.contains),
                ("keys", Self.keys),
                ("deleteAll", Self.deleteAll),
                ("biometric", Self.biometric),
            ] as [(String, ([String: Any]) throws -> Any?)] {
                channel.asyncMethod(name) { [queue] args, result in
                    let args = args as? [String: Any] ?? [:]
                    queue.async {
                        do {
                            result.success(try body(args))
                        } catch {
                            result.error(error)
                        }
                    }
                }
            }
        }
    }

    // MARK: - Methods

    private static func set(_ args: [String: Any]) throws -> Any? {
        let key = try string(args, "key")
        guard let data = try string(args, "value").data(using: .utf8) else {
            throw error("invalid_arguments", "value is not UTF-8")
        }
        let accessibility = parseAccessibility(args["accessibility"] as? String)
        var item = query(args, key: key)
        item[kSecValueData as String] = data
        if args["requireBiometric"] as? Bool ?? false {
            guard Bundle.main.object(forInfoDictionaryKey: "NSFaceIDUsageDescription") != nil else {
                throw error("biometrics_not_configured", "add biometrics: to the securestorage plugin config")
            }
            var laError: NSError?
            guard LAContext().canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, error: &laError) else {
                throw error("biometrics_unavailable", laError?.localizedDescription ?? "no biometrics enrolled")
            }
            var cfError: Unmanaged<CFError>?
            guard let access = SecAccessControlCreateWithFlags(nil, accessibility, .biometryCurrentSet, &cfError) else {
                throw cfError?.takeRetainedValue() ?? error("native_error", "could not create access control")
            }
            item[kSecAttrAccessControl as String] = access
        } else {
            item[kSecAttrAccessible as String] = accessibility
        }
        SecItemDelete(query(args, key: key) as CFDictionary)
        try check(SecItemAdd(item as CFDictionary, nil))
        return nil
    }

    private static func get(_ args: [String: Any]) throws -> Any? {
        var item = query(args, key: try string(args, "key"))
        item[kSecReturnData as String] = true
        item[kSecMatchLimit as String] = kSecMatchLimitOne
        let context = LAContext()
        context.localizedReason = args["biometricPrompt"] as? String ?? "Authenticate to read secure data"
        item[kSecUseAuthenticationContext as String] = context
        var result: AnyObject?
        let status = SecItemCopyMatching(item as CFDictionary, &result)
        if status == errSecItemNotFound {
            return ["value": NSNull()]
        }
        try check(status)
        guard let data = result as? Data, let value = String(data: data, encoding: .utf8) else {
            throw error("native_error", "stored value is not UTF-8")
        }
        return ["value": value]
    }

    private static func delete(_ args: [String: Any]) throws -> Any? {
        try checkDeleted(SecItemDelete(query(args, key: try string(args, "key")) as CFDictionary))
        return nil
    }

    private static func contains(_ args: [String: Any]) throws -> Any? {
        var item = query(args, key: try string(args, "key"))
        item[kSecMatchLimit as String] = kSecMatchLimitOne
        // Attributes only: no authentication, even for biometric items.
        item[kSecReturnAttributes as String] = true
        item[kSecUseAuthenticationContext as String] = noPrompt()
        var result: AnyObject?
        let status = SecItemCopyMatching(item as CFDictionary, &result)
        // A biometric item answers "interaction not allowed" when the UI is
        // skipped: it exists.
        return ["exists": status == errSecSuccess || status == errSecInteractionNotAllowed]
    }

    private static func keys(_ args: [String: Any]) throws -> Any? {
        var item = query(args, key: nil)
        item[kSecMatchLimit as String] = kSecMatchLimitAll
        item[kSecReturnAttributes as String] = true
        item[kSecUseAuthenticationContext as String] = noPrompt()
        var result: AnyObject?
        let status = SecItemCopyMatching(item as CFDictionary, &result)
        if status == errSecItemNotFound {
            return ["keys": [String]()]
        }
        try check(status)
        let items = result as? [[String: Any]] ?? []
        return ["keys": items.compactMap { $0[kSecAttrAccount as String] as? String }]
    }

    private static func deleteAll(_ args: [String: Any]) throws -> Any? {
        try checkDeleted(SecItemDelete(query(args, key: nil) as CFDictionary))
        return nil
    }

    private static func biometric(_ args: [String: Any]) throws -> Any? {
        let context = LAContext()
        guard context.canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, error: nil) else {
            return ["type": "none"]
        }
        switch context.biometryType {
        case .faceID: return ["type": "face_id"]
        case .touchID: return ["type": "touch_id"]
        case .opticID: return ["type": "optic_id"]
        default: return ["type": "none"]
        }
    }

    // MARK: - Helpers

    /// The query for key (all keys when nil) in args' service.
    private static func query(_ args: [String: Any], key: String?) -> [String: Any] {
        var query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: args["service"] as? String ?? Bundle.main.bundleIdentifier ?? "drift.securestorage",
        ]
        if let key {
            query[kSecAttrAccount as String] = key
        }
        return query
    }

    /// A context that fails instead of prompting.
    private static func noPrompt() -> LAContext {
        let context = LAContext()
        context.interactionNotAllowed = true
        return context
    }

    private static func string(_ args: [String: Any], _ name: String) throws -> String {
        guard let value = args[name] as? String else {
            throw error("invalid_arguments", "missing \(name)")
        }
        return value
    }

    private static func parseAccessibility(_ value: String?) -> CFString {
        switch value {
        case "after_first_unlock": return kSecAttrAccessibleAfterFirstUnlock
        case "when_unlocked_this_device_only": return kSecAttrAccessibleWhenUnlockedThisDeviceOnly
        case "after_first_unlock_this_device_only": return kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        default: return kSecAttrAccessibleWhenUnlocked
        }
    }

    private static func checkDeleted(_ status: OSStatus) throws {
        if status != errSecItemNotFound {
            try check(status)
        }
    }

    private static func check(_ status: OSStatus) throws {
        switch status {
        case errSecSuccess: return
        case errSecUserCanceled: throw error("auth_canceled", "authentication canceled")
        case errSecAuthFailed: throw error("auth_failed", "authentication failed")
        case errSecInteractionNotAllowed: throw error("locked", "the device is locked")
        default:
            let message = SecCopyErrorMessageString(status, nil) as String? ?? "Keychain error \(status)"
            throw error("native_error", message)
        }
    }

    private static func error(_ code: String, _ message: String) -> NSError {
        NSError(domain: code, code: 0, userInfo: [NSLocalizedDescriptionKey: message])
    }
}
