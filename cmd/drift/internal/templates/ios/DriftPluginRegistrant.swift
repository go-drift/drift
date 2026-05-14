/// DriftPluginRegistrant.swift
///
/// Initial placeholder. The Drift plugin pipeline overwrites this file at each
/// build with one host.registerChannel(...) (or plugin.register(host: host))
/// call per configured plugin. With zero plugins the body stays empty so the
/// runtime call site in PlatformChannelManager always resolves.

enum DriftPluginRegistrant {
    static func registerAll(host: DriftPluginHost) {
    }
}
