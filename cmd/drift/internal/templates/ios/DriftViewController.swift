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
///     Go engine calls scheduleFrame callback
///         │
///         ▼ scheduleFrame() unpauses display link
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
/// On-Demand Scheduling:
///   The CADisplayLink starts paused and is unpaused only when the Go engine
///   requests a frame via the schedule-frame callback. After rendering, if no
///   more frames are needed (e.g. animations have finished), the display link
///   is paused again. This avoids waking the CPU/GPU every vsync when the UI
///   is idle, matching the Android embedder's on-demand pattern.
///
/// Lifecycle:
///   - viewDidLoad: Creates the display link (paused), registers callbacks
///     and attaches plugins
///   - viewWillAppear: Unpauses after reappearing (e.g. modal dismissal)
///   - viewDidDisappear: Invalidates the display link to save resources
///   - tearDown: When the scene disconnects (SceneDelegate; DriftApp's
///     dismantle on xtool), stops rendering and detaches plugins, so they
///     drop the controller and it can be released. The display link holds
///     the controller only weakly, through DisplayLinkProxy.

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

    /// Hosts plugin overlays above Drift's content and platform views
    /// (DriftViewBinding.overlayView); PlatformViewHandler inserts platform
    /// views below it.
    private let overlayView = DriftOverlayView()

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
        overlayView.frame = view.bounds
        overlayView.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        view.addSubview(overlayView)
        // Platform views go into this view, below the plugin overlays
        PlatformViewHandler.setHostView(view, below: overlayView)
        // Initialize accessibility support
        AccessibilityHandler.shared.initialize(hostView: view)
        applySystemUIStyle(SystemUIHandler.currentStyle)
        // Register the schedule-frame callback so the Go engine can request frames
        driftScheduleFrameCallback = { [weak self] in self?.scheduleFrame() }
        DriftSetScheduleFrameHandler(nativeScheduleFrame)
        // Create the display link (starts paused) and request the first frame
        startDisplayLink()
        scheduleFrame()
        // Pre-warm expensive platform views (WebView, TextInput, plugin views)
        // on the next main-thread tick so the cost is absorbed before the user
        // navigates to pages that use them.
        if DriftShouldWarmUpViews() != 0 {
            DispatchQueue.main.async {
                PlatformViewHandler.warmUp()
            }
        }
        // Plugins attach while the launch screen is still up (the window is
        // not visible yet), so an overlay shows from the first frame.
        DriftPlugins.shared.attach(self, overlayView: overlayView)
    }

    /// Stops rendering and detaches plugins, for a scene that disconnected.
    /// Idempotent; deinit does the same in case no one called it.
    func tearDown() {
        stopDisplayLink()
        DriftPlugins.shared.detach(from: ObjectIdentifier(self))
        // The platform views move to the next controller's view.
        if isViewLoaded {
            PlatformViewHandler.removeHostView(view)
        }
    }

    deinit {
        displayLink?.invalidate()
        DriftPlugins.shared.detach(from: ObjectIdentifier(self))
        DriftLog.general.debug("DriftViewController released")
    }

    /// Tracks whether the initial safe area insets have been sent to the Go side.
    private var didSendInitialInsets = false

    override func viewWillLayoutSubviews() {
        super.viewWillLayoutSubviews()
        // Push initial insets before the first layout. At this point the view is
        // in the window hierarchy and safeAreaInsets are populated, unlike
        // viewDidLoad (too early) or viewDidAppear (too late, frames already drawn).
        if !didSendInitialInsets {
            didSendInitialInsets = true
            SafeAreaHandler.sendInsetsUpdate()
        }
    }

    override func viewWillAppear(_ animated: Bool) {
        super.viewWillAppear(animated)
        // Restart the render loop when view becomes visible again
        // (e.g., after dismissing a modal like camera picker)
        if displayLink == nil {
            startDisplayLink()
        }
        scheduleFrame()
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

    override func viewWillTransition(to size: CGSize, with coordinator: UIViewControllerTransitionCoordinator) {
        super.viewWillTransition(to: size, with: coordinator)

        // Force synchronous drawable presentation during rotation so each
        // frame is synchronized with the animation. Without this, Core
        // Animation distorts a stale snapshot of the old content.
        metalView.syncPresentationForRotation = true

        coordinator.animate(alongsideTransition: nil, completion: { [weak self] _ in
            self?.metalView.syncPresentationForRotation = false
        })
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

    /// Creates the display link for vsync-synchronized rendering.
    ///
    /// The link starts paused and is unpaused on demand by scheduleFrame().
    /// It's added to the main run loop in `.common` mode so it continues
    /// running even during UI tracking (e.g., scrolling).
    private func startDisplayLink() {
        // CADisplayLink retains its target; a weak proxy keeps it from
        // keeping the controller alive.
        let link = CADisplayLink(target: DisplayLinkProxy(self), selector: #selector(DisplayLinkProxy.tick))
        if #available(iOS 15.0, *) {
            link.preferredFrameRateRange = CAFrameRateRange(minimum: 60, maximum: 60, preferred: 60)
        } else {
            link.preferredFramesPerSecond = 60
        }
        link.add(to: .main, forMode: .common)
        link.isPaused = true
        displayLink = link
    }

    /// Unpauses the display link so the next vsync triggers a frame render.
    /// Called from the Go engine's schedule-frame callback.
    func scheduleFrame() {
        displayLink?.isPaused = false
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
        // The status bar is transparent over Drift's content; apps inset
        // content with SafeArea. Only visibility and icon style change.
        setNeedsStatusBarAppearanceUpdate()
    }

    /// Called by the display link on each vsync to render a frame.
    ///
    /// After rendering, pauses the display link if no more frames are needed
    /// (e.g. animations have finished). The link will be unpaused again when
    /// the Go engine calls the schedule-frame callback.
    fileprivate func drawFrame() {
        metalView.renderFrame()
        if DriftNeedsFrame() == 0 {
            displayLink?.isPaused = true
        }
    }
}

/// The display link's target, holding the controller weakly.
private final class DisplayLinkProxy: NSObject {
    private weak var controller: DriftViewController?

    init(_ controller: DriftViewController) {
        self.controller = controller
        super.init()
    }

    @objc func tick() {
        controller?.drawFrame()
    }
}

/// Full-screen host for plugin overlays. Transparent to touches where it has
/// no subviews, so it never blocks Drift's content or platform views.
final class DriftOverlayView: UIView {
    override func hitTest(_ point: CGPoint, with event: UIEvent?) -> UIView? {
        let hit = super.hitTest(point, with: event)
        return hit === self ? nil : hit
    }
}
