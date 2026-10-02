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
        host.registerChannel("example/hello") { channel in
            channel.method("greeting") { _ in
                Bundle.main.object(forInfoDictionaryKey: "HelloGreeting") as? String
            }
        }
    }
}
