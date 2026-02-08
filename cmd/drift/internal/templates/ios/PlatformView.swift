/// PlatformView.swift
/// Provides platform view management for embedding native views in Drift UI.

import UIKit

// MARK: - Geometry Applied Signal

/// FFI declaration for the Go-exported DriftGeometryApplied function.
/// Called after applying platform view geometry to signal the render thread.
@_silgen_name("DriftGeometryApplied")
func DriftGeometryApplied()

// MARK: - Platform View Handler

/// Handles platform view channel methods from Go.
enum PlatformViewHandler {
    private static var views: [Int: PlatformViewContainer] = [:]
    private static weak var hostView: UIView?

    /// Frame sequence tracking for geometry batches
    private static var lastAppliedSeq: UInt64 = 0

    /// Sets the host view where platform views will be added.
    static func setHostView(_ view: UIView) {
        hostView = view
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
        case "setGeometry":
            return setGeometry(args: dict)
        case "batchSetGeometry":
            return batchSetGeometry(args: dict)
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

        // Add to host view on main thread
        DispatchQueue.main.async {
            if let host = hostView {
                host.addSubview(view.view)
                view.view.isHidden = true // Hidden until positioned
            }
        }

        // Notify Go that view is created
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/platform_views",
            data: [
                "method": "onViewCreated",
                "viewId": viewId
            ]
        )

        return (["created": true], nil)
    }

    private static func dispose(args: [String: Any]) -> (Any?, Error?) {
        guard let viewId = args["viewId"] as? Int else {
            return (nil, nil)
        }

        if let container = views[viewId] {
            DispatchQueue.main.async {
                container.dispose()
            }
            views.removeValue(forKey: viewId)
        }

        return (nil, nil)
    }

    private static func setGeometry(args: [String: Any]) -> (Any?, Error?) {
        guard let viewId = args["viewId"] as? Int,
              let container = views[viewId] else {
            return (nil, nil)
        }

        let x = args["x"] as? Double ?? 0
        let y = args["y"] as? Double ?? 0
        let width = args["width"] as? Double ?? 0
        let height = args["height"] as? Double ?? 0
        let clipLeft = args["clipLeft"] as? Double
        let clipTop = args["clipTop"] as? Double
        let clipRight = args["clipRight"] as? Double
        let clipBottom = args["clipBottom"] as? Double

        DispatchQueue.main.async {
            container.view.frame = CGRect(x: x, y: y, width: width, height: height)
            applyClipBounds(
                view: container.view,
                viewX: CGFloat(x), viewY: CGFloat(y),
                viewWidth: CGFloat(width), viewHeight: CGFloat(height),
                clipLeft: clipLeft.map { CGFloat($0) },
                clipTop: clipTop.map { CGFloat($0) },
                clipRight: clipRight.map { CGFloat($0) },
                clipBottom: clipBottom.map { CGFloat($0) }
            )
        }

        return (nil, nil)
    }

    /// Apply clip bounds to a view using CALayer masking.
    /// Clip bounds are in logical points (no density conversion needed on iOS).
    private static func applyClipBounds(
        view: UIView,
        viewX: CGFloat, viewY: CGFloat,
        viewWidth: CGFloat, viewHeight: CGFloat,
        clipLeft: CGFloat?, clipTop: CGFloat?,
        clipRight: CGFloat?, clipBottom: CGFloat?
    ) {
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

        // Partial clip - apply mask
        // Note: Allocates a new CAShapeLayer each update. If scroll performance degrades,
        // consider reusing a mask layer per view.
        let maskPath = UIBezierPath(rect: CGRect(x: left, y: top, width: right - left, height: bottom - top))
        let maskLayer = CAShapeLayer()
        maskLayer.path = maskPath.cgPath
        view.layer.mask = maskLayer
        view.isHidden = false
    }

    /// Batch geometry update. Posts geometry changes to the main thread and returns
    /// immediately. The frameSeq mechanism ensures stale batches are skipped if the
    /// main thread falls behind.
    private static func batchSetGeometry(args: [String: Any]) -> (Any?, Error?) {
        guard let frameSeq = args["frameSeq"] as? UInt64,
              let geometries = args["geometries"] as? [[String: Any]] else {
            return (nil, nil)
        }

        if geometries.isEmpty {
            return (nil, nil)
        }

        // Skip stale batches (older than last applied).
        // Still signal geometry applied so the render thread doesn't timeout.
        if frameSeq <= lastAppliedSeq {
            DriftGeometryApplied()
            return (nil, nil)
        }

        let applyGeometries = {
            for geom in geometries {
                guard let viewId = geom["viewId"] as? Int,
                      let container = views[viewId] else {
                    continue
                }

                let x = geom["x"] as? Double ?? 0
                let y = geom["y"] as? Double ?? 0
                let width = geom["width"] as? Double ?? 0
                let height = geom["height"] as? Double ?? 0
                let clipLeft = geom["clipLeft"] as? Double
                let clipTop = geom["clipTop"] as? Double
                let clipRight = geom["clipRight"] as? Double
                let clipBottom = geom["clipBottom"] as? Double

                container.view.frame = CGRect(x: x, y: y, width: width, height: height)
                applyClipBounds(
                    view: container.view,
                    viewX: CGFloat(x), viewY: CGFloat(y),
                    viewWidth: CGFloat(width), viewHeight: CGFloat(height),
                    clipLeft: clipLeft.map { CGFloat($0) },
                    clipTop: clipTop.map { CGFloat($0) },
                    clipRight: clipRight.map { CGFloat($0) },
                    clipBottom: clipBottom.map { CGFloat($0) }
                )
            }
            lastAppliedSeq = frameSeq
        }

        // If already on main thread, apply directly and signal immediately.
        if Thread.isMainThread {
            applyGeometries()
            DriftGeometryApplied()
            return (nil, nil)
        }

        // Post to main thread and return immediately.
        // frameSeq ensures stale batches are skipped if main thread falls behind.
        // Signal geometry applied after the closure runs so the render thread
        // can defer surface presentation until geometry lands.
        DispatchQueue.main.async {
            applyGeometries()
            DriftGeometryApplied()
        }
        return (nil, nil)
    }

    private static func setVisible(args: [String: Any]) -> (Any?, Error?) {
        guard let viewId = args["viewId"] as? Int,
              let visible = args["visible"] as? Bool,
              let container = views[viewId] else {
            return (nil, nil)
        }

        DispatchQueue.main.async {
            container.view.isHidden = !visible
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
