/// DriftPlatformView.swift
///
/// Native views embedded in Drift UI, provided by plugins (and by Drift for
/// its own text inputs, switches and web views). A plugin registers a view
/// type with `DriftPluginHost.registerPlatformView`; its Go runtime
/// registers a `platform.PlatformViewFactory` of the same type and embeds
/// the view in a widget. Drift positions, clips and shows the view each
/// frame.
///
/// Threading: Drift creates views and calls them on the main thread, in
/// the order Go made the calls.
///
/// A view outlives the view controller that hosts it: when the scene
/// reconnects, Drift moves it into the new Drift view. Do not keep the
/// hosting view controller; reach the current one through `view.window`.

import UIKit

/// One native view instance.
public protocol DriftPlatformView: AnyObject {
    /// The native view Drift positions over its content.
    var view: UIView { get }

    /// Handles a method Go called (`PlatformViewRegistry.InvokeViewMethod`);
    /// `args` holds the call's arguments. Go does not wait for view methods:
    /// a thrown error (an unknown method, a bad argument) is logged. Report
    /// results to Go as events.
    func handle(_ method: String, _ args: [String: Any]) throws

    /// Releases the view's resources; Drift removes the view afterwards.
    func dispose()

    /// Called after Drift moved or resized the view.
    func onGeometryChanged()
}

public extension DriftPlatformView {
    func onGeometryChanged() {}
}

/// Sends a view's events to its Go view (`PlatformView.HandleEvent`).
public final class DriftPlatformViewEvents {
    /// The view's ID, shared with Go.
    public let viewId: Int
    private let deliver: ([String: Any]) -> Void

    @_spi(DriftHost) public init(viewId: Int, deliver: @escaping ([String: Any]) -> Void) {
        self.viewId = viewId
        self.deliver = deliver
    }

    /// Sends `method` with `data` to the Go view, from any thread.
    public func send(_ method: String, _ data: [String: Any] = [:]) {
        var event = data
        event["method"] = method
        event["viewId"] = viewId
        deliver(event)
    }
}

/// The `observeEvent` channel Drift posts on once at launch, on the main
/// thread, unless the app called `engine.DisableViewWarmUp`. A plugin whose
/// view loads a heavy framework creates and drops a throwaway instance so
/// the first real view opens without a stall. Native only: Go never sees
/// it.
public let driftPlatformViewWarmUpEvent = "drift/platform_views/warm_up"

/// Creates a view of one type: its ID, the parameters Go created it with,
/// and its event sender. An error it throws fails Go's
/// `PlatformViewRegistry.Create` (a `DriftCallError` with its code, any
/// other error as `native_error`), and no view exists.
public typealias DriftPlatformViewFactory = (_ viewId: Int, _ params: [String: Any], _ events: DriftPlatformViewEvents) throws -> DriftPlatformView
