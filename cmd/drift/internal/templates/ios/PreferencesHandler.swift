/// PreferencesHandler.swift
/// Handles simple key-value storage using UserDefaults.

import Foundation

enum PreferencesHandler {
    // UserDefaults.standard is shared app-wide, so we prefix all keys to avoid
    // collisions with system defaults or other libraries. Android doesn't need
    // a prefix because it uses a dedicated SharedPreferences file instead.
    private static let keyPrefix = "drift_prefs_"

    // MARK: - Public Interface

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "set":
            return set(args: args)
        case "get":
            return get(args: args)
        case "delete":
            return delete(args: args)
        case "contains":
            return contains(args: args)
        case "getAllKeys":
            return getAllKeys()
        case "deleteAll":
            return deleteAll()
        default:
            return (nil, NSError(domain: "Preferences", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    // MARK: - CRUD Operations

    private static func set(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let key = dict["key"] as? String,
              let value = dict["value"] as? String else {
            return (nil, NSError(domain: "Preferences", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing key or value"]))
        }

        UserDefaults.standard.set(value, forKey: keyPrefix + key)
        return (nil, nil)
    }

    private static func get(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let key = dict["key"] as? String else {
            return (nil, NSError(domain: "Preferences", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing key"]))
        }

        let value = UserDefaults.standard.string(forKey: keyPrefix + key)
        return (["value": value as Any], nil)
    }

    private static func delete(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let key = dict["key"] as? String else {
            return (nil, NSError(domain: "Preferences", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing key"]))
        }

        UserDefaults.standard.removeObject(forKey: keyPrefix + key)
        return (nil, nil)
    }

    private static func contains(args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any],
              let key = dict["key"] as? String else {
            return (nil, NSError(domain: "Preferences", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing key"]))
        }

        let exists = UserDefaults.standard.object(forKey: keyPrefix + key) != nil
        return (["exists": exists], nil)
    }

    private static func getAllKeys() -> (Any?, Error?) {
        let allKeys = UserDefaults.standard.dictionaryRepresentation().keys
        let prefixedKeys = allKeys
            .filter { $0.hasPrefix(keyPrefix) }
            .map { String($0.dropFirst(keyPrefix.count)) }
        return (["keys": prefixedKeys], nil)
    }

    private static func deleteAll() -> (Any?, Error?) {
        let allKeys = UserDefaults.standard.dictionaryRepresentation().keys
        for key in allKeys where key.hasPrefix(keyPrefix) {
            UserDefaults.standard.removeObject(forKey: key)
        }
        return (nil, nil)
    }
}
