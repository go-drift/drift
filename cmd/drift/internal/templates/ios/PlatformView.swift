/// PlatformView.swift
/// Provides platform view management for embedding native views in Drift UI.

import UIKit

// MARK: - Platform View Handler

/// Handles platform view channel methods from Go.
enum PlatformViewHandler {
    private static var views: [Int: PlatformViewContainer] = [:]
    private static weak var hostView: UIView?

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
        case "setVisible":
            return setVisible(args: dict)
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
            supportedMethods = ["loadUrl", "goBack", "goForward", "reload"]
        } else {
            supportedMethods = []
        }

        guard supportedMethods.contains(method) else {
            return (nil, NSError(domain: "PlatformView", code: 400, userInfo: [NSLocalizedDescriptionKey: "Unknown method '\(method)' for view type"]))
        }

        if let webViewContainer = container as? NativeWebViewContainer {
            DispatchQueue.main.async {
                switch method {
                case "loadUrl":
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

        DispatchQueue.main.async {
            container.view.frame = CGRect(x: x, y: y, width: width, height: height)
            container.view.isHidden = false
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
                "method": "onError",
                "viewId": viewId,
                "error": error.localizedDescription
            ]
        )
    }
}
