/// HelloPlugin.swift
///
/// The hello plugin's native half on iOS. It compiles into its own module
/// (DriftPlugin_hello), so the class and its initializer are public: the
/// app's generated registrant creates it from outside the module.

import DriftPluginAPI
import Foundation

public final class HelloPlugin: DriftPlugin {
    public init() {}

    public func register(host: DriftPluginHost) {
        host.registerChannel("example/hello") { method, _, result in
            switch method {
            case "greeting":
                result.success(Bundle.main.object(forInfoDictionaryKey: "HelloGreeting") as? String)
            default:
                result.error(NSError(domain: "example.hello", code: 1, userInfo: [
                    NSLocalizedDescriptionKey: "unknown method \(method)",
                ]))
            }
        }
    }
}
