/// PlatformView.swift
/// Provides platform view management for embedding native views in Drift UI.

@_spi(DriftHost) import DriftPluginAPI
import UIKit

/// FFI declaration for DriftHitTestPlatformView.
/// Returns 1 if the platform view is the topmost target, 0 if obscured.
@_silgen_name("DriftHitTestPlatformView")
func DriftHitTestPlatformView(_ viewID: Int64, _ x: Double, _ y: Double) -> Int32

// MARK: - Touch Interceptor View

/// Wraps each platform view to intercept touches when the view is obscured by
/// Drift widgets (modal barriers, dropdowns, etc.).
///
/// Two modes of operation:
///
/// **Obscured**: hitTest returns self. The interceptor receives all touches and
/// forwards them to the Drift engine.
///
/// **Topmost**: hitTest returns super.hitTest() so the native child view receives
/// real UITouch objects through UIKit's standard responder chain. All native
/// gesture recognizers (long press, double tap, magnifying loupe, etc.) work.
///
/// For unfocused text inputs inside Drift scroll views, a ScrollForwardingRecognizer
/// (attached to the interceptor) monitors touches targeting the child. If the user
/// scrolls (movement exceeds slop), the recognizer claims the gesture, UIKit cancels
/// the text field's touch, and the scroll forwards to the engine. If the user taps,
/// the recognizer fails silently and the text field handles everything natively.
class TouchInterceptorView: UIView {
    let viewId: Int
    var enableUnfocusedTextScrollForwarding: Bool = true

    // When true, the interceptor received the touch (obscured case).
    private var blocked: Bool = false

    // UIKit calls hitTest multiple times per touch event as it walks the
    // view hierarchy. Cache the engine query result for the current runloop
    // iteration to avoid redundant CGo round-trips and frameLock acquisitions.
    // Note: the cache is invalidated via DispatchQueue.main.async at the end
    // of the current runloop iteration. If the view hierarchy changes between
    // two consecutive ticks (e.g., an animation repositions an overlay), the
    // second touch could read the stale value from the previous tick. The
    // window is a single runloop iteration, and the consequence is one touch
    // routed to the wrong target.
    private var cachedIsTopmost: Bool = true
    private var cacheValid: Bool = false

    // Scroll detection for unfocused text inputs inside Drift scroll views.
    // Attached to the interceptor so it sees touches targeting child views
    // (UIKit delivers touches to recognizers on all ancestor views).
    // Enabled only when the touch targets an unfocused text input.
    private var scrollRecognizer: ScrollForwardingRecognizer!

    init(viewId: Int) {
        self.viewId = viewId
        super.init(frame: .zero)
        scrollRecognizer = ScrollForwardingRecognizer()
        scrollRecognizer.cancelsTouchesInView = true
        scrollRecognizer.isEnabled = false
        addGestureRecognizer(scrollRecognizer)
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) is not supported")
    }

    override func hitTest(_ point: CGPoint, with event: UIEvent?) -> UIView? {
        guard bounds.contains(point) else { return nil }

        let isTopmost: Bool
        if cacheValid {
            isTopmost = cachedIsTopmost
        } else {
            // Query Go engine: is this platform view the topmost target?
            let scale = window?.screen.scale ?? UIScreen.main.scale
            let globalPoint = convert(point, to: superview)
            let result = DriftHitTestPlatformView(
                Int64(viewId),
                Double(globalPoint.x * scale),
                Double(globalPoint.y * scale)
            )
            isTopmost = result == 1
            cachedIsTopmost = isTopmost
            cacheValid = true

            // Invalidate at the end of the current runloop iteration
            DispatchQueue.main.async { [weak self] in
                self?.cacheValid = false
            }
        }

        if !isTopmost {
            // Obscured: return self so we receive touches for engine forwarding.
            blocked = true
            scrollRecognizer.isEnabled = false
            return self
        }

        // Topmost: return the child so it receives real UITouch objects.
        // Enable scroll detection when an unfocused text input is the target,
        // so scrolls starting on the field forward to the Drift engine rather
        // than being consumed by the native view.
        blocked = false
        scrollRecognizer.isEnabled = enableUnfocusedTextScrollForwarding && findUnfocusedTextInput(at: point) != nil
        return super.hitTest(point, with: event)
    }

    // MARK: - Touch Handling (blocked/obscured mode only)

    override func touchesBegan(_ touches: Set<UITouch>, with event: UIEvent?) {
        if blocked { forwardTouches(touches, phase: 0) }
    }

    override func touchesMoved(_ touches: Set<UITouch>, with event: UIEvent?) {
        if blocked { forwardTouches(touches, phase: 1) }
    }

    override func touchesEnded(_ touches: Set<UITouch>, with event: UIEvent?) {
        if blocked {
            forwardTouches(touches, phase: 2)
            cleanupPointerIDs(touches)
        }
    }

    override func touchesCancelled(_ touches: Set<UITouch>, with event: UIEvent?) {
        if blocked {
            forwardTouches(touches, phase: 3)
            cleanupPointerIDs(touches)
        }
    }

    // MARK: - Touch Forwarding

    fileprivate func forwardTouches(_ touches: Set<UITouch>, phase: Int32) {
        guard let hostView = superview else { return }
        let scale = hostView.contentScaleFactor
        for touch in touches {
            let point = touch.location(in: hostView)
            let pointerID = TouchPointerIDManager.shared.getID(for: touch)
            DriftPointerEvent(pointerID, phase, Double(point.x * scale), Double(point.y * scale))
        }
        // Render immediately so the engine processes the touch and updates
        // the UI (e.g. dismissing an overlay) without waiting for a scheduled
        // display link tick. Matches DriftMetalView.handleTouch's pattern.
        DriftRequestFrame()
    }

    /// Forward a specific local point to the engine (used by ScrollForwardingRecognizer).
    fileprivate func forwardPoint(_ localPoint: CGPoint, phase: Int32, touch: UITouch) {
        guard let hostView = superview else { return }
        let scale = hostView.contentScaleFactor
        let point = convert(localPoint, to: hostView)
        let pointerID = TouchPointerIDManager.shared.getID(for: touch)
        DriftPointerEvent(pointerID, phase, Double(point.x * scale), Double(point.y * scale))
        DriftRequestFrame()
    }

    fileprivate func cleanupPointerID(for touch: UITouch) {
        TouchPointerIDManager.shared.releaseID(for: touch)
    }

    private func cleanupPointerIDs(_ touches: Set<UITouch>) {
        for touch in touches {
            TouchPointerIDManager.shared.releaseID(for: touch)
        }
    }

    // MARK: - Unfocused Text Input Detection

    /// Walks the view hierarchy to find an unfocused UITextField or UITextView
    /// at the given point (in the receiver's coordinate space).
    private func findUnfocusedTextInput(at point: CGPoint) -> UIView? {
        return findUnfocusedTextInput(in: self, at: point)
    }

    private func findUnfocusedTextInput(in parent: UIView, at point: CGPoint) -> UIView? {
        if parent !== self && (parent is UITextField || parent is UITextView) && !parent.isFirstResponder {
            return parent
        }
        for i in stride(from: parent.subviews.count - 1, through: 0, by: -1) {
            let child = parent.subviews[i]
            guard !child.isHidden else { continue }
            let childPoint = parent.convert(point, to: child)
            guard child.bounds.contains(childPoint) else { continue }
            if let found = findUnfocusedTextInput(in: child, at: childPoint) {
                return found
            }
        }
        return nil
    }
}

/// Detects scroll gestures starting on unfocused text inputs and forwards them
/// to the Drift engine. Attached to TouchInterceptorView (parent), so it sees
/// touches targeting child views via UIKit's gesture recognizer participation.
///
/// The text field receives real UITouch objects simultaneously (delaysTouchesBegan
/// is false by default). On scroll detection, this recognizer claims the gesture
/// and UIKit cancels the text field's touch (cancelsTouchesInView = true). On tap,
/// this recognizer fails and the text field handles everything natively, preserving
/// cursor placement, magnifying loupe, selection handles, etc.
private class ScrollForwardingRecognizer: UIGestureRecognizer {
    private var startPoint: CGPoint = .zero

    // Matches PaddedTextField/PaddedTextView's old slop value (12pt).
    private static let touchSlop: CGFloat = 12.0

    private var interceptor: TouchInterceptorView? {
        return view as? TouchInterceptorView
    }

    override func touchesBegan(_ touches: Set<UITouch>, with event: UIEvent) {
        super.touchesBegan(touches, with: event)
        if let touch = touches.first, let v = view {
            startPoint = touch.location(in: v)
        }
    }

    override func touchesMoved(_ touches: Set<UITouch>, with event: UIEvent) {
        super.touchesMoved(touches, with: event)
        guard let touch = touches.first, let v = view else { return }

        switch state {
        case .possible:
            let current = touch.location(in: v)
            let dx = abs(current.x - startPoint.x)
            let dy = abs(current.y - startPoint.y)
            if dx > Self.touchSlop || dy > Self.touchSlop {
                // Movement exceeded slop: claim the gesture.
                // UIKit cancels the text field's touch sequence.
                state = .began
                interceptor?.forwardPoint(startPoint, phase: 0, touch: touch)
                interceptor?.forwardPoint(current, phase: 1, touch: touch)
            }
        case .began, .changed:
            state = .changed
            interceptor?.forwardPoint(touch.location(in: v), phase: 1, touch: touch)
        default:
            break
        }
    }

    override func touchesEnded(_ touches: Set<UITouch>, with event: UIEvent) {
        super.touchesEnded(touches, with: event)
        switch state {
        case .began, .changed:
            if let touch = touches.first, let v = view {
                interceptor?.forwardPoint(touch.location(in: v), phase: 2, touch: touch)
                interceptor?.cleanupPointerID(for: touch)
            }
            state = .ended
        default:
            // No scroll detected: fail so the text field handles the tap.
            state = .failed
        }
    }

    override func touchesCancelled(_ touches: Set<UITouch>, with event: UIEvent) {
        super.touchesCancelled(touches, with: event)
        if state == .began || state == .changed {
            if let touch = touches.first, let v = view {
                interceptor?.forwardPoint(touch.location(in: v), phase: 3, touch: touch)
                interceptor?.cleanupPointerID(for: touch)
            }
        }
        state = .cancelled
    }

    override func reset() {
        super.reset()
        startPoint = .zero
    }
}

// MARK: - Platform View Handler

/// Handles platform view channel methods from Go.
enum PlatformViewHandler {
    /// Whether any platform views are currently active.
    /// Used by DriftMetalView to enable synchronized presentation.
    static var hasPlatformViews: Bool { !views.isEmpty }

    /// View factories by type, registered at launch (Drift's and plugins').
    private static var factories: [String: DriftPlatformViewFactory] = [:]
    private static let factoriesLock = NSLock()

    // Main thread only: every operation on a view runs there, in the order
    // Go made the calls.
    private static var views: [Int: DriftPlatformView] = [:]
    private static var viewTypes: [Int: String] = [:]
    private static var interceptors: [Int: TouchInterceptorView] = [:]
    private static var maskLayers: [Int: CAShapeLayer] = [:]
    private static weak var hostView: UIView?
    /// A subview of hostView that platform views stay below (plugin overlays).
    private static weak var overlayView: UIView?

    /// Sets the host view where platform views go, below overlayView, one of
    /// its subviews. Views outlive their host: those created earlier (for
    /// a scene that disconnected, or before any host) move into this one,
    /// hidden until the next frame positions them.
    static func setHostView(_ view: UIView, below overlayView: UIView) {
        dispatchPrecondition(condition: .onQueue(.main))
        precondition(overlayView.superview === view, "drift: platform view overlay must be a subview of the host view")
        hostView = view
        self.overlayView = overlayView
        for (viewId, interceptor) in interceptors {
            interceptor.removeFromSuperview()
            interceptor.isHidden = true
            view.insertSubview(interceptor, belowSubview: overlayView)
            views[viewId]?.onHostChanged()
        }
    }

    /// Takes the platform views out of view, a host whose scene
    /// disconnected, so they do not stay in its dead hierarchy; the next
    /// host takes them in.
    static func removeHostView(_ view: UIView) {
        dispatchPrecondition(condition: .onQueue(.main))
        guard hostView === view else { return }
        for (viewId, interceptor) in interceptors {
            interceptor.removeFromSuperview()
            views[viewId]?.onHostChanged()
        }
        hostView = nil
        overlayView = nil
    }

    /// Pre-warms expensive platform view classes by creating throwaway
    /// instances, forcing WebKit process spawning early so the cost is
    /// absorbed before the user navigates to pages using them. Plugins warm
    /// their own views on `driftPlatformViewWarmUpEvent`.
    ///
    /// Must be called on the main thread.
    static func warmUp() {
        _ = WKWebView(frame: .zero)
        _ = UITextField()
        PlatformChannelManager.shared.notifyEventObservers(channel: driftPlatformViewWarmUpEvent, data: nil)
        DriftLog.platform.log("Platform view warmup complete")
    }

    /// Applies platform view geometry from a frame snapshot synchronously.
    /// Called on the main thread between StepAndSnapshot and RenderSync to
    /// ensure native views are positioned before the GPU frame is composited.
    static func applySnapshot(_ views: [ViewSnapshot]) {
        CATransaction.setDisableActions(true)

        for snap in views {
            guard let container = self.views[snap.viewId] else { continue }
            let targetView = interceptors[snap.viewId] ?? container.view

            if !snap.visible {
                targetView.isHidden = true
                continue
            }

            let x = CGFloat(snap.x)
            let y = CGFloat(snap.y)
            let w = CGFloat(snap.width)
            let h = CGFloat(snap.height)

            targetView.frame = CGRect(x: x, y: y, width: w, height: h)
            // Show the view before applying clip bounds, since applyClipBounds
            // may hide it again if the clip area is zero.
            targetView.isHidden = false
            applyClipBounds(
                viewId: snap.viewId,
                view: targetView,
                viewX: x, viewY: y,
                viewWidth: w, viewHeight: h,
                visibleLeft: CGFloat(snap.visibleLeft),
                visibleTop: CGFloat(snap.visibleTop),
                visibleRight: CGFloat(snap.visibleRight),
                visibleBottom: CGFloat(snap.visibleBottom),
                occlusionPaths: snap.occlusionPaths
            )
            container.onGeometryChanged()
        }
    }

    /// Registers the view type `type`; a second registration traps.
    static func register(_ type: String, _ factory: @escaping DriftPlatformViewFactory) {
        factoriesLock.lock()
        defer { factoriesLock.unlock() }
        if factories[type] != nil {
            fatalError("drift: platform view type \"\(type)\" is already registered")
        }
        factories[type] = factory
    }

    /// Registers Drift's own view types.
    static func registerBuiltIns(host: DriftPluginHost) {
        host.registerPlatformView("native_webview") { id, params, _ in NativeWebViewContainer(viewId: id, params: params) }
        host.registerPlatformView("textinput") { id, params, _ in NativeTextInputContainer(viewId: id, params: params) }
        host.registerPlatformView("switch") { id, params, _ in NativeSwitchContainer(viewId: id, params: params) }
        host.registerPlatformView("activity_indicator") { id, params, _ in NativeActivityIndicatorContainer(viewId: id, params: params) }
    }

    /// Registers the drift/platform_views channel. `create` runs the
    /// factory on the main thread and replies with its outcome, so a
    /// factory's error reaches Go's Create. Every later operation on a view
    /// runs asynchronously on the main thread, after earlier ones, and Go
    /// does not wait for it.
    static func start(host: DriftPluginHost) {
        host.registerChannel("drift/platform_views") { channel in
            channel.method("create") { args in
                try create(args: viewArgs(args))
            }
            channel.method("dispose") { args in
                try onView(viewArgs(args)) { viewId, container in
                    let interceptor = interceptors.removeValue(forKey: viewId)
                    maskLayers.removeValue(forKey: viewId)
                    views.removeValue(forKey: viewId)
                    viewTypes.removeValue(forKey: viewId)
                    container.dispose()
                    interceptor?.removeFromSuperview()
                }
            }
            channel.method("setVisible") { args in
                let dict = try viewArgs(args)
                let visible = dict["visible"] as? Bool ?? true
                return try onView(dict) { viewId, container in
                    (interceptors[viewId] ?? container.view).isHidden = !visible
                }
            }
            channel.method("setEnabled") { args in
                let dict = try viewArgs(args)
                let enabled = dict["enabled"] as? Bool ?? true
                return try onView(dict) { _, container in
                    container.view.isUserInteractionEnabled = enabled
                    container.view.alpha = enabled ? 1.0 : 0.5
                }
            }
            channel.method("invokeViewMethod") { args in
                let dict = try viewArgs(args)
                guard let viewMethod = dict["method"] as? String else {
                    throw DriftCallError.invalidArguments("Missing method")
                }
                return try onView(dict) { viewId, container in
                    do {
                        try container.handle(viewMethod, dict)
                    } catch {
                        DriftLog.platform.error("platform view \(viewId) (\(viewTypes[viewId] ?? "?")): \(viewMethod): \(error.localizedDescription)")
                    }
                    if viewTypes[viewId] == "textinput", viewMethod == "updateConfig" {
                        let multiline = dict["multiline"] as? Bool ?? false
                        interceptors[viewId]?.enableUnfocusedTextScrollForwarding = !multiline
                    }
                }
            }
        }
    }

    private static func viewArgs(_ args: Any?) throws -> [String: Any] {
        guard let dict = args as? [String: Any] else {
            throw DriftCallError.invalidArguments("Invalid arguments")
        }
        return dict
    }

    /// Runs body on the main thread, after earlier calls, with the view
    /// args' viewId names; a view already disposed is skipped (Go may call
    /// it after disposing).
    private static func onView(_ args: [String: Any], _ body: @escaping (Int, DriftPlatformView) -> Void) throws -> Any? {
        guard let viewId = args["viewId"] as? Int else {
            throw DriftCallError.invalidArguments("Missing viewId")
        }
        DispatchQueue.main.async {
            guard let container = views[viewId] else { return }
            body(viewId, container)
        }
        return nil
    }

    /// Creates the view on the main thread, failing the call if the
    /// factory throws, and puts it in the host (or the next host, if none
    /// is set). Go is notified once the view is in the hierarchy, so
    /// resendGeometry targets it.
    private static func create(args: [String: Any]) throws -> Any? {
        guard let viewId = args["viewId"] as? Int,
              let viewType = args["viewType"] as? String else {
            throw DriftCallError.invalidArguments("Missing viewId or viewType")
        }
        let params = args["params"] as? [String: Any] ?? [:]

        factoriesLock.lock()
        let factory = factories[viewType]
        factoriesLock.unlock()
        guard let factory else {
            throw DriftCallError("view_type_not_found", "no native platform view type \"\(viewType)\"; add the plugin that provides it to drift.yaml")
        }

        let events = DriftPlatformViewEvents(viewId: viewId) { event in
            PlatformChannelManager.shared.sendEvent(channel: "drift/platform_views", data: event)
        }
        let container = try factory(viewId, params, events)
        views[viewId] = container
        viewTypes[viewId] = viewType
        let interceptor = TouchInterceptorView(viewId: viewId)
        if viewType == "textinput" {
            let multiline = params["multiline"] as? Bool ?? false
            interceptor.enableUnfocusedTextScrollForwarding = !multiline
        }
        interceptor.addSubview(container.view)
        // Child fills interceptor
        container.view.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            container.view.topAnchor.constraint(equalTo: interceptor.topAnchor),
            container.view.leadingAnchor.constraint(equalTo: interceptor.leadingAnchor),
            container.view.trailingAnchor.constraint(equalTo: interceptor.trailingAnchor),
            container.view.bottomAnchor.constraint(equalTo: interceptor.bottomAnchor)
        ])
        interceptor.isHidden = true // Hidden until positioned
        interceptors[viewId] = interceptor
        if let host = hostView, let overlay = overlayView {
            host.insertSubview(interceptor, belowSubview: overlay)
        }
        events.send("onViewCreated")
        return nil
    }

    /// Apply visibility masking to a platform view using visibleRect and occlusion masks.
    /// Uses CAShapeLayer with even-odd fill to cut holes for occluding overlays.
    /// Clip bounds are in logical points (no density conversion needed on iOS).
    /// Disables implicit CALayer animations internally.
    private static func applyClipBounds(
        viewId: Int,
        view: UIView,
        viewX: CGFloat, viewY: CGFloat,
        viewWidth: CGFloat, viewHeight: CGFloat,
        visibleLeft: CGFloat, visibleTop: CGFloat,
        visibleRight: CGFloat, visibleBottom: CGFloat,
        occlusionPaths: [CGPath]
    ) {
        CATransaction.setDisableActions(true)

        // Convert visible rect from global to local view coordinates.
        let localLeft = max(0, min(visibleLeft - viewX, viewWidth))
        let localTop = max(0, min(visibleTop - viewY, viewHeight))
        let localRight = max(0, min(visibleRight - viewX, viewWidth))
        let localBottom = max(0, min(visibleBottom - viewY, viewHeight))

        // Visible rect is empty: hide view entirely.
        if localLeft >= localRight || localTop >= localBottom {
            view.isHidden = true
            view.layer.mask = nil
            return
        }

        let visibleCGRect = CGRect(
            x: localLeft, y: localTop,
            width: localRight - localLeft, height: localBottom - localTop
        )

        // Fully visible (covers entire view) and no occlusion: clear mask.
        let coversFullView = localLeft <= 0 && localTop <= 0 &&
            localRight >= viewWidth && localBottom >= viewHeight
        if coversFullView && occlusionPaths.isEmpty {
            view.layer.mask = nil
            view.isHidden = false
            return
        }

        // Build mask path: outer visible rect, with occlusion paths subtracted.
        // Occlusion paths are already in view-local coordinates (transformed during decode).
        let maskPath = UIBezierPath(rect: visibleCGRect)

        for holePath in occlusionPaths {
            maskPath.append(UIBezierPath(cgPath: holePath))
        }

        // Reuse cached mask layer to avoid per-frame allocation.
        let maskLayer: CAShapeLayer
        if let cached = maskLayers[viewId] {
            maskLayer = cached
        } else {
            maskLayer = CAShapeLayer()
            maskLayers[viewId] = maskLayer
        }
        maskLayer.fillRule = occlusionPaths.isEmpty ? .nonZero : .evenOdd
        maskLayer.path = maskPath.cgPath
        view.layer.mask = maskLayer
        view.isHidden = false
    }
}

/// The error a view's handle(_:_:) throws for a method it does not know.
func unknownViewMethod(_ method: String) -> DriftCallError {
    DriftCallError("method_not_found", "unknown view method \(method)")
}

// MARK: - Native Web View Container

import WebKit

class NativeWebViewContainer: NSObject, DriftPlatformView, WKNavigationDelegate {
    let viewId: Int
    let view: UIView
    private let webView: WKWebView

    init(viewId: Int, params: [String: Any]) {
        self.viewId = viewId

        let config = WKWebViewConfiguration()
        let web = WKWebView(frame: .zero, configuration: config)
        web.backgroundColor = .white

        self.webView = web
        self.view = web

        super.init()

        web.navigationDelegate = self

        // Load initial URL if provided
        if let urlString = params["initialUrl"] as? String,
           let url = URL(string: urlString) {
            web.load(URLRequest(url: url))
        }
    }

    func dispose() {
        webView.stopLoading()
        view.removeFromSuperview()
    }

    func handle(_ method: String, _ args: [String: Any]) throws {
        switch method {
        case "load":
            if let urlString = args["url"] as? String, let url = URL(string: urlString) {
                loadURL(url)
            }
        case "goBack": goBack()
        case "goForward": goForward()
        case "reload": reload()
        default: throw unknownViewMethod(method)
        }
    }

    // MARK: - Navigation Methods

    func loadURL(_ url: URL) {
        webView.load(URLRequest(url: url))
    }

    func goBack() {
        webView.goBack()
    }

    func goForward() {
        webView.goForward()
    }

    func reload() {
        webView.reload()
    }

    // MARK: - WKNavigationDelegate

    func webView(_ webView: WKWebView, didStartProvisionalNavigation navigation: WKNavigation!) {
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/platform_views",
            data: [
                "method": "onPageStarted",
                "viewId": viewId,
                "url": webView.url?.absoluteString ?? ""
            ]
        )
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/platform_views",
            data: [
                "method": "onPageFinished",
                "viewId": viewId,
                "url": webView.url?.absoluteString ?? ""
            ]
        )
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        sendLoadError(error)
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        sendLoadError(error)
    }

    /// Reports a navigation's failure, unless a newer navigation cancelled
    /// it (that one reports its own start and end).
    private func sendLoadError(_ error: Error) {
        let nsError = error as NSError
        if nsError.domain == NSURLErrorDomain && nsError.code == NSURLErrorCancelled {
            return
        }
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/platform_views",
            data: [
                "method": "onWebViewError",
                "viewId": viewId,
                "code": webViewErrorCode(for: error),
                "message": error.localizedDescription
            ]
        )
    }

    private func webViewErrorCode(for error: Error) -> String {
        let nsError = error as NSError
        if nsError.domain == NSURLErrorDomain {
            switch nsError.code {
            case NSURLErrorServerCertificateHasBadDate,
                 NSURLErrorServerCertificateUntrusted,
                 NSURLErrorServerCertificateHasUnknownRoot,
                 NSURLErrorServerCertificateNotYetValid,
                 NSURLErrorClientCertificateRejected,
                 NSURLErrorClientCertificateRequired:
                return "ssl_error"
            case NSURLErrorTimedOut,
                 NSURLErrorCannotFindHost,
                 NSURLErrorCannotConnectToHost,
                 NSURLErrorNetworkConnectionLost,
                 NSURLErrorNotConnectedToInternet:
                return "network_error"
            default:
                return "load_failed"
            }
        }
        return "load_failed"
    }
}
