/// PlatformView.swift
/// Provides platform view management for embedding native views in Drift UI.

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

    private static var views: [Int: PlatformViewContainer] = [:]
    private static var interceptors: [Int: TouchInterceptorView] = [:]
    private static var maskLayers: [Int: CAShapeLayer] = [:]
    private static weak var hostView: UIView?

    /// Sets the host view where platform views will be added.
    static func setHostView(_ view: UIView) {
        hostView = view
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

            targetView.frame = CGRect(
                x: snap.x, y: snap.y,
                width: snap.width, height: snap.height
            )
            // Show the view before applying clip bounds, since applyClipBounds
            // may hide it again if the clip area is zero.
            targetView.isHidden = false
            applyClipBounds(
                viewId: snap.viewId,
                view: targetView,
                viewX: CGFloat(snap.x), viewY: CGFloat(snap.y),
                viewWidth: CGFloat(snap.width), viewHeight: CGFloat(snap.height),
                clipLeft: snap.clipLeft.map { CGFloat($0) },
                clipTop: snap.clipTop.map { CGFloat($0) },
                clipRight: snap.clipRight.map { CGFloat($0) },
                clipBottom: snap.clipBottom.map { CGFloat($0) }
            )
            container.onGeometryChanged()
        }
    }

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        guard let dict = args as? [String: Any] else {
            return (nil, NSError(domain: "PlatformView", code: 400, userInfo: [NSLocalizedDescriptionKey: "Invalid arguments"]))
        }

        switch method {
        case "create":
            return create(args: dict)
        case "dispose":
            return dispose(args: dict)
        case "setVisible":
            return setVisible(args: dict)
        case "setEnabled":
            return setEnabled(args: dict)
        case "invokeViewMethod":
            return invokeViewMethod(args: dict)
        default:
            return (nil, NSError(domain: "PlatformView", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    private static func invokeViewMethod(args: [String: Any]) -> (Any?, Error?) {
        guard let viewId = args["viewId"] as? Int else {
            return (nil, NSError(domain: "PlatformView", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing viewId"]))
        }
        guard let method = args["method"] as? String else {
            return (nil, NSError(domain: "PlatformView", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing method"]))
        }
        guard let container = views[viewId] else {
            return (nil, NSError(domain: "PlatformView", code: 404, userInfo: [NSLocalizedDescriptionKey: "View not found: \(viewId)"]))
        }

        // Validate method is supported
        let supportedMethods: Set<String>
        if container is NativeWebViewContainer {
            supportedMethods = ["load", "goBack", "goForward", "reload"]
        } else if container is NativeTextInputContainer {
            supportedMethods = ["setText", "setSelection", "setValue", "focus", "blur", "updateConfig"]
        } else if container is NativeSwitchContainer {
            supportedMethods = ["setValue", "updateConfig"]
        } else if container is NativeActivityIndicatorContainer {
            supportedMethods = ["setAnimating", "updateConfig"]
        } else if container is NativeVideoPlayerContainer {
            supportedMethods = ["play", "pause", "stop", "seekTo", "setVolume", "setLooping", "setPlaybackSpeed", "load"]
        } else {
            supportedMethods = []
        }

        guard supportedMethods.contains(method) else {
            return (nil, NSError(domain: "PlatformView", code: 400, userInfo: [NSLocalizedDescriptionKey: "Unknown method '\(method)' for view type"]))
        }

        if let webViewContainer = container as? NativeWebViewContainer {
            DispatchQueue.main.async {
                switch method {
                case "load":
                    if let urlString = args["url"] as? String,
                       let url = URL(string: urlString) {
                        webViewContainer.loadURL(url)
                    }
                case "goBack":
                    webViewContainer.goBack()
                case "goForward":
                    webViewContainer.goForward()
                case "reload":
                    webViewContainer.reload()
                default:
                    break
                }
            }
        } else if let textInputContainer = container as? NativeTextInputContainer {
            DispatchQueue.main.async {
                switch method {
                case "setText":
                    if let text = args["text"] as? String {
                        textInputContainer.setText(text)
                    }
                case "setSelection":
                    let base = args["selectionBase"] as? Int ?? 0
                    let extent = args["selectionExtent"] as? Int ?? 0
                    textInputContainer.setSelection(base: base, extent: extent)
                case "setValue":
                    let text = args["text"] as? String ?? ""
                    let base = args["selectionBase"] as? Int ?? text.count
                    let extent = args["selectionExtent"] as? Int ?? text.count
                    textInputContainer.setValue(text: text, selectionBase: base, selectionExtent: extent)
                case "focus":
                    textInputContainer.focus()
                case "blur":
                    textInputContainer.blur()
                case "updateConfig":
                    textInputContainer.updateConfig(args)
                    let multiline = args["multiline"] as? Bool ?? false
                    interceptors[viewId]?.enableUnfocusedTextScrollForwarding = !multiline
                default:
                    break
                }
            }
        } else if let switchContainer = container as? NativeSwitchContainer {
            DispatchQueue.main.async {
                switch method {
                case "setValue":
                    if let value = args["value"] as? Bool {
                        switchContainer.setValue(value)
                    }
                case "updateConfig":
                    switchContainer.updateConfig(args)
                default:
                    break
                }
            }
        } else if let indicatorContainer = container as? NativeActivityIndicatorContainer {
            DispatchQueue.main.async {
                switch method {
                case "setAnimating":
                    if let animating = args["animating"] as? Bool {
                        indicatorContainer.setAnimating(animating)
                    }
                case "updateConfig":
                    indicatorContainer.updateConfig(args)
                default:
                    break
                }
            }
        } else if let videoContainer = container as? NativeVideoPlayerContainer {
            DispatchQueue.main.async {
                switch method {
                case "play":
                    videoContainer.play()
                case "pause":
                    videoContainer.pause()
                case "stop":
                    videoContainer.stop()
                case "seekTo":
                    if let positionMs = (args["positionMs"] as? NSNumber)?.int64Value {
                        videoContainer.seekTo(positionMs: positionMs)
                    }
                case "setVolume":
                    if let volume = (args["volume"] as? NSNumber)?.floatValue {
                        videoContainer.setVolume(volume)
                    }
                case "setLooping":
                    if let looping = args["looping"] as? Bool {
                        videoContainer.setLooping(looping)
                    }
                case "setPlaybackSpeed":
                    if let rate = (args["rate"] as? NSNumber)?.floatValue {
                        videoContainer.setPlaybackSpeed(rate)
                    }
                case "load":
                    if let urlString = args["url"] as? String {
                        videoContainer.load(urlString)
                    }
                default:
                    break
                }
            }
        }

        return (nil, nil)
    }

    private static func create(args: [String: Any]) -> (Any?, Error?) {
        guard let viewId = args["viewId"] as? Int,
              let viewType = args["viewType"] as? String else {
            return (nil, NSError(domain: "PlatformView", code: 400, userInfo: [NSLocalizedDescriptionKey: "Missing viewId or viewType"]))
        }

        let params = args["params"] as? [String: Any] ?? [:]

        // Create the platform view based on type
        let container: PlatformViewContainer?

        switch viewType {
        case "native_webview":
            container = createNativeWebView(viewId: viewId, params: params)
        case "textinput":
            container = NativeTextInputContainer(viewId: viewId, params: params)
        case "switch":
            container = NativeSwitchContainer(viewId: viewId, params: params)
        case "activity_indicator":
            container = NativeActivityIndicatorContainer(viewId: viewId, params: params)
        case "video_player":
            container = NativeVideoPlayerContainer(viewId: viewId, params: params)
        default:
            return (nil, NSError(domain: "PlatformView", code: 400, userInfo: [NSLocalizedDescriptionKey: "Unknown view type: \(viewType)"]))
        }

        guard let view = container else {
            return (nil, NSError(domain: "PlatformView", code: 500, userInfo: [NSLocalizedDescriptionKey: "Failed to create view"]))
        }

        views[viewId] = view

        // Add to host view on main thread, wrapped in a TouchInterceptorView.
        // Notify Go only after the interceptor is attached so resendGeometry
        // targets the actual host view (not the unattached child).
        DispatchQueue.main.async {
            if let host = hostView {
                let interceptor = TouchInterceptorView(viewId: viewId)
                if viewType == "textinput" {
                    let multiline = params["multiline"] as? Bool ?? false
                    interceptor.enableUnfocusedTextScrollForwarding = !multiline
                }
                interceptor.addSubview(view.view)
                // Child fills interceptor
                view.view.translatesAutoresizingMaskIntoConstraints = false
                NSLayoutConstraint.activate([
                    view.view.topAnchor.constraint(equalTo: interceptor.topAnchor),
                    view.view.leadingAnchor.constraint(equalTo: interceptor.leadingAnchor),
                    view.view.trailingAnchor.constraint(equalTo: interceptor.trailingAnchor),
                    view.view.bottomAnchor.constraint(equalTo: interceptor.bottomAnchor)
                ])
                interceptor.isHidden = true // Hidden until positioned
                interceptors[viewId] = interceptor
                host.addSubview(interceptor)
            }

            PlatformChannelManager.shared.sendEvent(
                channel: "drift/platform_views",
                data: [
                    "method": "onViewCreated",
                    "viewId": viewId
                ]
            )
        }

        return (["created": true], nil)
    }

    private static func dispose(args: [String: Any]) -> (Any?, Error?) {
        guard let viewId = args["viewId"] as? Int else {
            return (nil, nil)
        }

        if let container = views[viewId] {
            let interceptor = interceptors.removeValue(forKey: viewId)
            maskLayers.removeValue(forKey: viewId)
            DispatchQueue.main.async {
                container.dispose()
                interceptor?.removeFromSuperview()
            }
            views.removeValue(forKey: viewId)
        }

        return (nil, nil)
    }

    /// Apply clip bounds to a view using CALayer masking.
    /// Clip bounds are in logical points (no density conversion needed on iOS).
    /// Disables implicit CALayer animations internally.
    private static func applyClipBounds(
        viewId: Int,
        view: UIView,
        viewX: CGFloat, viewY: CGFloat,
        viewWidth: CGFloat, viewHeight: CGFloat,
        clipLeft: CGFloat?, clipTop: CGFloat?,
        clipRight: CGFloat?, clipBottom: CGFloat?
    ) {
        CATransaction.setDisableActions(true)

        // No clip provided - clear any existing mask, but don't change visibility
        // (visibility is controlled by SetVisible or by full clipping below)
        guard let clipLeft = clipLeft,
              let clipTop = clipTop,
              let clipRight = clipRight,
              let clipBottom = clipBottom else {
            view.layer.mask = nil
            return
        }

        // Convert global clip to local view coordinates
        let localClipLeft = clipLeft - viewX
        let localClipTop = clipTop - viewY
        let localClipRight = clipRight - viewX
        let localClipBottom = clipBottom - viewY

        // Clamp to view bounds
        let left = max(0, min(localClipLeft, viewWidth))
        let top = max(0, min(localClipTop, viewHeight))
        let right = max(0, min(localClipRight, viewWidth))
        let bottom = max(0, min(localClipBottom, viewHeight))

        // Completely clipped - hide view
        if left >= right || top >= bottom {
            view.isHidden = true
            view.layer.mask = nil
            return
        }

        // Fully visible (clip covers entire view) - no mask needed
        // Check local values directly to avoid sub-pixel edge exposure
        if localClipLeft <= 0 && localClipTop <= 0 &&
           localClipRight >= viewWidth && localClipBottom >= viewHeight {
            view.layer.mask = nil
            view.isHidden = false
            return
        }

        // Partial clip - reuse cached mask layer to avoid per-frame allocation
        let maskPath = UIBezierPath(rect: CGRect(x: left, y: top, width: right - left, height: bottom - top))
        let maskLayer: CAShapeLayer
        if let cached = maskLayers[viewId] {
            maskLayer = cached
        } else {
            maskLayer = CAShapeLayer()
            maskLayers[viewId] = maskLayer
        }
        maskLayer.path = maskPath.cgPath
        view.layer.mask = maskLayer
        view.isHidden = false
    }

    private static func setVisible(args: [String: Any]) -> (Any?, Error?) {
        guard let viewId = args["viewId"] as? Int,
              let visible = args["visible"] as? Bool,
              let container = views[viewId] else {
            return (nil, nil)
        }

        DispatchQueue.main.async {
            let targetView = interceptors[viewId] ?? container.view
            targetView.isHidden = !visible
        }

        return (nil, nil)
    }

    private static func setEnabled(args: [String: Any]) -> (Any?, Error?) {
        guard let viewId = args["viewId"] as? Int,
              let enabled = args["enabled"] as? Bool,
              let container = views[viewId] else {
            return (nil, nil)
        }

        DispatchQueue.main.async {
            container.view.isUserInteractionEnabled = enabled
            container.view.alpha = enabled ? 1.0 : 0.5
        }

        return (nil, nil)
    }

    // MARK: - View Factories

    private static func createNativeWebView(viewId: Int, params: [String: Any]) -> PlatformViewContainer? {
        return NativeWebViewContainer(viewId: viewId, params: params)
    }
}

// MARK: - Platform View Protocol

protocol PlatformViewContainer {
    var viewId: Int { get }
    var view: UIView { get }
    func dispose()
    func onGeometryChanged()
}

extension PlatformViewContainer {
    func onGeometryChanged() {}
}

// MARK: - Native Web View Container

import WebKit

class NativeWebViewContainer: NSObject, PlatformViewContainer, WKNavigationDelegate {
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

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
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
