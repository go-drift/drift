/// DriftViewController.swift
/// View controller that manages the Drift rendering surface on iOS.
///
/// This controller is responsible for:
///   1. Hosting the DriftMetalView as its main view
///   2. Managing the CADisplayLink for vsync-synchronized rendering
///   3. Coordinating the render loop lifecycle with the view controller lifecycle
///
/// Rendering Pipeline:
///
///     CADisplayLink (vsync signal)
///         │
///         ▼ drawFrame() selector
///     DriftViewController (this file)
///         │
///         ▼ renderFrame()
///     DriftMetalView
///         │
///         ▼ draw(to:)
///     DriftRenderer
///         │
///         ▼ DriftRenderFrame (FFI)
///     Go Engine
///
/// Display Link:
///   CADisplayLink is a timer synchronized to the display's refresh rate
///   (typically 60Hz, or 120Hz on ProMotion devices). Using it ensures:
///     - Smooth animation without tearing
///     - Efficient battery usage (no spinning on the CPU)
///     - Automatic adaptation to display refresh rate
///
/// Lifecycle:
///   - viewDidLoad: Starts the display link when view is ready
///   - viewDidDisappear: Stops the display link to save resources

import UIKit

/// View controller that hosts the Metal rendering surface and manages the render loop.
///
/// Marked as `final` because there's no need for subclassing, and it enables
/// the compiler to optimize virtual method calls.
final class DriftViewController: UIViewController {

    /// The display link that synchronizes rendering with the display's refresh rate.
    ///
    /// Optional because it's created in viewDidLoad and invalidated in viewDidDisappear.
    /// When invalidated, it should be set to nil to allow deallocation.
    private var displayLink: CADisplayLink?

    /// The Metal view that displays the Go engine's rendered content.
    ///
    /// Created immediately as a constant since it's used throughout the controller's lifetime.
    private let metalView = DriftMetalView()

    override var preferredStatusBarStyle: UIStatusBarStyle {
        SystemUIHandler.currentStyle.statusBarStyle
    }

    override var prefersStatusBarHidden: Bool {
        SystemUIHandler.currentStyle.statusBarHidden
    }

    /// Provides the Metal view as this controller's main view.
    ///
    /// This is called before viewDidLoad to get the controller's root view.
    /// By overriding loadView, we can use our Metal view directly without
    /// needing to add it as a subview later.
    override func loadView() {
        // Set our Metal view as the controller's main view.
        // This replaces the default UIView that would be created.
        view = metalView
    }

    /// Called after the view has been loaded into memory.
    ///
    /// This is the appropriate place to start the display link since
    /// the view hierarchy is now set up.
    override func viewDidLoad() {
        super.viewDidLoad()
        // Initialize platform view handler with this view as the host
        PlatformViewHandler.setHostView(view)
        applySystemUIStyle(SystemUIHandler.currentStyle)
        // Start the render loop
        startDisplayLink()
    }

    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
        applySystemUIStyle(SystemUIHandler.currentStyle)
        SafeAreaHandler.sendInsetsUpdate()
    }

    override func viewSafeAreaInsetsDidChange() {
        super.viewSafeAreaInsetsDidChange()
        SafeAreaHandler.sendInsetsUpdate()
    }

    /// Called when the view is removed from the window.
    ///
    /// Stops the display link to conserve battery and CPU when the
    /// view is not visible. The display link will be restarted if
    /// the view appears again.
    ///
    /// - Parameter animated: Whether the disappearance is animated.
    override func viewDidDisappear(_ animated: Bool) {
        super.viewDidDisappear(animated)
        // Stop the render loop when view is not visible
        stopDisplayLink()
    }

    /// Creates and starts the display link for vsync-synchronized rendering.
    ///
    /// The display link will call drawFrame() at the display's refresh rate.
    /// It's added to the main run loop in `.common` mode so it continues
    /// running even during UI tracking (e.g., scrolling).
    private func startDisplayLink() {
        // Create a display link that calls our drawFrame method on each vsync.
        // The target is `self` and the selector is the method to call.
        let link = CADisplayLink(target: self, selector: #selector(drawFrame))

        // Add to the main run loop in common mode.
        // Common mode includes default and tracking modes, so rendering
        // continues even during UI interactions like scrolling.
        link.add(to: .main, forMode: .common)

        // Store the link so we can invalidate it later.
        displayLink = link
    }

    /// Stops and releases the display link.
    ///
    /// Invalidating the display link removes it from all run loops and releases
    /// its target. After invalidation, the link cannot be restarted; a new
    /// CADisplayLink must be created.
    private func stopDisplayLink() {
        // Invalidate removes from run loop and releases the target reference.
        displayLink?.invalidate()

        // Set to nil to allow deallocation and indicate stopped state.
        displayLink = nil
    }

    func applySystemUIStyle(_ style: SystemUIStyle) {
        // On iOS, Transparent and BackgroundColor are no-ops since iOS doesn't
        // have a status bar background color (unlike Android's statusBarColor).
        // The status bar is always transparent and shows whatever content is
        // rendered behind it. Apps control this by using SafeArea to inset
        // content away from the status bar area.
        //
        // We only need to update status bar visibility and style (light/dark icons).
        setNeedsStatusBarAppearanceUpdate()
    }

    /// Called by the display link on each vsync to render a frame.
    ///
    /// This method bridges the CADisplayLink callback to the Metal view's
    /// render method. It's marked @objc because Objective-C selectors
    /// are used by CADisplayLink.
    @objc private func drawFrame() {
        // Delegate actual rendering to the Metal view.
        metalView.renderFrame()
    }
}
