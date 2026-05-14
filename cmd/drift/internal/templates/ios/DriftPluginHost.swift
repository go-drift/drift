/// DriftPluginHost.swift
///
/// Stable host API consumed by Drift plugin Swift sources. Plugin sources
/// reference this protocol; the templated PlatformChannelManager adopts it.
/// MethodHandler is declared here so both the protocol and concrete plugins
/// can refer to it without depending on PlatformChannel.swift internals.

import Foundation

typealias MethodHandler = (String, Any?) -> (Any?, Error?)

protocol DriftPluginHost: AnyObject {
    func registerChannel(_ name: String, handler: @escaping MethodHandler)
    func sendEvent(_ channel: String, data: Any?)
    func sendEventError(_ channel: String, code: String, message: String)
    func sendEventDone(_ channel: String)
}
