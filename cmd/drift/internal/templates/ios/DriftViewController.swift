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
///   - viewDidLoad: Creates the display link (paused) and registers callbacks
///   - viewWillAppear: Unpauses after reappearing (e.g. modal dismissal)
///   - viewDidDisappear: Invalidates the display link to save resources

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
        // Initialize accessibility support
        AccessibilityHandler.shared.initialize(hostView: view)
        applySystemUIStyle(SystemUIHandler.currentStyle)
        // Register the schedule-frame callback so the Go engine can request frames
        driftScheduleFrameCallback = { [weak self] in self?.scheduleFrame() }
        DriftSetScheduleFrameHandler(nativeScheduleFrame)
        // Create the display link (starts paused) and request the first frame
        startDisplayLink()
        scheduleFrame()
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
        let link = CADisplayLink(target: self, selector: #selector(drawFrame))
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
    /// After rendering, pauses the display link if no more frames are needed
    /// (e.g. animations have finished). The link will be unpaused again when
    /// the Go engine calls the schedule-frame callback.
    @objc private func drawFrame() {
        metalView.renderFrame()
        if DriftNeedsFrame() == 0 {
            displayLink?.isPaused = true
        }
    }
}
