/// PreferencesHandler.swift
/// Handles simple key-value storage using UserDefaults.

import DriftPluginAPI
import Foundation

enum PreferencesHandler {
    // UserDefaults.standard is shared app-wide, so we prefix all keys to avoid
    // collisions with system defaults or other libraries. Android doesn't need
    // a prefix because it uses a dedicated SharedPreferences file instead.
    private static let keyPrefix = "drift_prefs_"

    /// Registers the drift/preferences channel.
    static func start(host: DriftPluginHost) {
        host.registerChannel("drift/preferences") { channel in
            channel.method("set") { args in
                guard let dict = args as? [String: Any],
                      let key = dict["key"] as? String,
                      let value = dict["value"] as? String else {
                    throw DriftCallError.invalidArguments("Missing key or value")
                }
                UserDefaults.standard.set(value, forKey: keyPrefix + key)
                return nil
            }
            channel.method("get") { args in
                ["value": UserDefaults.standard.string(forKey: keyPrefix + (try key(args))) as Any]
            }
            channel.method("delete") { args in
                UserDefaults.standard.removeObject(forKey: keyPrefix + (try key(args)))
                return nil
            }
            channel.method("contains") { args in
                ["exists": UserDefaults.standard.object(forKey: keyPrefix + (try key(args))) != nil]
            }
            channel.method("getAllKeys") { _ in
                ["keys": prefixedKeys().map { String($0.dropFirst(keyPrefix.count)) }]
            }
            channel.method("deleteAll") { _ in
                for key in prefixedKeys() {
                    UserDefaults.standard.removeObject(forKey: key)
                }
                return nil
            }
        }
    }

    private static func key(_ args: Any?) throws -> String {
        guard let dict = args as? [String: Any], let key = dict["key"] as? String else {
            throw DriftCallError.invalidArguments("Missing key")
        }
        return key
    }

    /// The UserDefaults keys Drift's preferences use, prefix included.
    private static func prefixedKeys() -> [String] {
        UserDefaults.standard.dictionaryRepresentation().keys.filter { $0.hasPrefix(keyPrefix) }
    }
}
