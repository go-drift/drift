/// DriftMetalView.swift
/// Metal-backed UIView that displays the Drift engine's rendered content on iOS.
///
/// This view provides:
///   1. A CAMetalLayer for efficient GPU rendering
///   2. Touch event handling that forwards input to the Go engine
///   3. Integration with DriftRenderer for actual rendering
///
/// Rendering Architecture (split pipeline):
///
///     DriftViewController
///         |
///         v renderFrame()
///     DriftMetalView (this file)
///         |
///         1. DriftRenderer.stepAndSnapshot()  (layout + record)
///         2. PlatformViewHandler.applySnapshot()  (sync geometry)
///         3. DriftRenderer.renderSync()  (composite + present)
///         |
///         DriftPointerEvent()  (touch input via FFI)
///
/// CAMetalLayer:
///   Unlike regular CALayer, CAMetalLayer provides direct access to Metal
///   drawables for efficient GPU rendering. By using it as the layer class,
///   the view's content is rendered by Metal rather than Core Animation.
///
/// Touch Handling:
///   Touch coordinates are converted from points to pixels by multiplying
///   by contentScaleFactor (2x on Retina, 3x on Plus/Max devices). This
///   matches the Go engine's pixel-based coordinate system.

import Metal
import UIKit

/// FFI declaration for the Go pointer event handler.
///
/// @_silgen_name tells Swift to use this exact symbol name when linking,
/// bypassing Swift's name mangling. This allows direct calls to the
/// Go function exported via CGO.
///
/// - Parameters:
///   - pointerID: Unique identifier for the pointer/touch (enables multi-touch).
///   - phase: The touch phase (0=Down, 1=Move, 2=Up, 3=Cancel).
///   - x: X coordinate in pixels.
///   - y: Y coordinate in pixels.
@_silgen_name("DriftPointerEvent")
func DriftPointerEvent(_ pointerID: Int64, _ phase: Int32, _ x: Double, _ y: Double)

/// FFI declaration for updating the device scale factor in the Go engine.
///
/// - Parameter scale: The device scale factor (e.g., 2.0 or 3.0 on Retina).
@_silgen_name("DriftSetDeviceScale")
func DriftSetDeviceScale(_ scale: Double)

/// FFI declaration for checking if a new frame needs to be rendered.
/// Returns 1 if a frame is needed, 0 otherwise.
@_silgen_name("DriftNeedsFrame")
func DriftNeedsFrame() -> Int32

/// FFI declaration for requesting a new frame from the Go engine.
@_silgen_name("DriftRequestFrame")
func DriftRequestFrame()

/// FFI declaration for registering the schedule-frame callback with the Go engine.
/// The Go engine calls this handler when it needs the platform to produce a frame.
@_silgen_name("DriftSetScheduleFrameHandler")
func DriftSetScheduleFrameHandler(_ handler: @convention(c) () -> Void)

/// Callback invoked when the Go engine needs a frame. Set by DriftViewController.
/// This assumes a single DriftViewController instance, which is Drift's architecture.
var driftScheduleFrameCallback: (() -> Void)?

/// C-callable function registered with Go via DriftSetScheduleFrameHandler.
/// Dispatches to the main thread since CADisplayLink must be controlled from there.
/// When already on the main thread (the common case for touch-initiated requests),
/// unpauses the display link synchronously so the very next vsync fires without
/// waiting for the async dispatch to run.
func nativeScheduleFrame() {
    if Thread.isMainThread {
        driftScheduleFrameCallback?()
    } else {
        DispatchQueue.main.async {
            driftScheduleFrameCallback?()
        }
    }
}

/// A UIView backed by a CAMetalLayer for displaying Drift engine content.
///
/// Marked as `final` for performance optimization since no subclassing is expected.
final class DriftMetalView: UIView {

    /// The renderer that handles Metal setup and drawing operations.
    ///
    /// Created immediately and lives for the view's lifetime.
    private let renderer = DriftRenderer()

    /// Closure that provides accessibility elements from the AccessibilityBridge.
    /// Set by AccessibilityHandler during initialization.
    var accessibilityElementsProvider: (() -> [Any]?)?

    // MARK: - Accessibility Container

    override var isAccessibilityElement: Bool {
        get { false }
        set { }
    }

    override var accessibilityElements: [Any]? {
        get { accessibilityElementsProvider?() }
        set { }
    }

    override var accessibilityContainerType: UIAccessibilityContainerType {
        get { .semanticGroup }
        set { }
    }

    override func accessibilityElementCount() -> Int {
        return accessibilityElements?.count ?? 0
    }

    override func accessibilityElement(at index: Int) -> Any? {
        guard let elements = accessibilityElements, index >= 0, index < elements.count else {
            return nil
        }
        return elements[index]
    }

    override func index(ofAccessibilityElement element: Any) -> Int {
        guard let elements = accessibilityElements else { return NSNotFound }
        for (index, e) in elements.enumerated() {
            if (e as AnyObject) === (element as AnyObject) {
                return index
            }
        }
        return NSNotFound
    }

    /// Maps active UITouch objects to stable, non-negative pointer IDs.
    /// Using ObjectIdentifier ensures we track touch identity correctly.
    private var touchToPointerID: [ObjectIdentifier: Int64] = [:]

    /// Counter for assigning monotonically increasing pointer IDs.
    private var nextPointerID: Int64 = 0

    /// Specifies CAMetalLayer as the backing layer class.
    ///
    /// This is a class property that UIView uses when creating the layer.
    /// By returning CAMetalLayer.self, we get a Metal-compatible layer
    /// instead of the default CALayer.
    override class var layerClass: AnyClass {
        CAMetalLayer.self
    }

    /// Provides typed access to the view's Metal layer.
    ///
    /// Force unwrap is safe because layerClass guarantees the layer type.
    private var metalLayer: CAMetalLayer {
        layer as! CAMetalLayer
    }

    /// Initializes the view with a frame rectangle.
    ///
    /// Called when creating the view programmatically.
    ///
    /// - Parameter frame: The frame rectangle for the view.
    override init(frame: CGRect) {
        super.init(frame: frame)
        configureLayer()
    }

    /// Initializes the view from a storyboard or nib.
    ///
    /// Called when loading from Interface Builder.
    ///
    /// - Parameter coder: The decoder containing archived data.
    required init?(coder: NSCoder) {
        super.init(coder: coder)
        configureLayer()
    }

    /// Configures the Metal layer for rendering.
    ///
    /// Sets up:
    ///   - device: The Metal device for creating resources
    ///   - pixelFormat: RGBA8 to match Go engine output
    ///   - framebufferOnly: false to allow texture reads (needed for backdrop blur)
    ///   - contentScaleFactor: Matches screen scale for Retina support
    private func configureLayer() {
        // Use the same Metal device as the renderer for resource sharing.
        metalLayer.device = renderer.device

        // RGBA8 format matches the Go engine's output format.
        // Unorm means unsigned normalized (0.0-1.0 range).
        metalLayer.pixelFormat = .rgba8Unorm

        // Allow texture reads for backdrop blur and other effects.
        // Setting to false matches Flutter/Impeller behavior.
        metalLayer.framebufferOnly = false

        // Use triple buffering for smoother frame pacing.
        // Default is 2, but 3 gives more headroom for GPU/CPU overlap.
        metalLayer.maximumDrawableCount = 3

        // Match the screen's scale factor for proper Retina rendering.
        // 2x for standard Retina, 3x for Plus/Max devices.
        contentScaleFactor = UIScreen.main.scale

        // Black background fills any gaps visible during rotation transitions
        // before the engine renders at the new size.
        backgroundColor = .black

        // Inform the Go engine of the device scale for consistent sizing.
        DriftSetDeviceScale(Double(contentScaleFactor))
    }

    /// Called when the view's bounds change.
    ///
    /// Updates the Metal layer's drawable size to match the new bounds,
    /// accounting for the device's scale factor.
    override func layoutSubviews() {
        super.layoutSubviews()

        // Remove any in-flight Core Animation transforms on this layer.
        // During rotation, UIKit adds a bounds/position animation that
        // distorts the rendered content. Stripping it here makes the
        // Metal layer jump directly to the new geometry.
        layer.removeAllAnimations()

        CATransaction.begin()
        CATransaction.setDisableActions(true)
        metalLayer.drawableSize = CGSize(
            width: bounds.width * contentScaleFactor,
            height: bounds.height * contentScaleFactor
        )
        CATransaction.commit()

        // Keep the Go engine scale in sync with the view's scale factor.
        DriftSetDeviceScale(Double(contentScaleFactor))

        // Request and render a frame so the engine re-renders at the new size.
        DriftRequestFrame()
        renderFrame()
    }

    /// Set by the view controller during rotation transitions to force
    /// synchronous presentation regardless of platform view state.
    var syncPresentationForRotation = false

    /// Renders a single frame to the Metal layer using the split pipeline.
    ///
    /// Called by DriftViewController on each display link callback.
    /// The split pipeline separates layout from compositing:
    ///   1. stepAndSnapshot: runs build/layout/record, returns platform view geometry
    ///   2. applySnapshot: positions native views synchronously on the main thread
    ///   3. renderSync: composites display lists into the Metal texture
    ///
    /// This eliminates the async geometry round-trip.
    ///
    /// - Returns: true if a frame was rendered, false if skipped.
    @discardableResult
    func renderFrame() -> Bool {
        guard DriftNeedsFrame() != 0 else { return false }

        let width = Int32(bounds.width * contentScaleFactor)
        let height = Int32(bounds.height * contentScaleFactor)
        guard width > 0, height > 0 else { return false }

        // Step 1: Run the engine pipeline and capture platform view geometry.
        let snapshot = renderer.stepAndSnapshot(width: width, height: height)

        // Step 2: Apply geometry synchronously before compositing.
        if let views = snapshot?.views, !views.isEmpty {
            PlatformViewHandler.applySnapshot(views)
        }

        // Synchronize Metal presentation with Core Animation when platform
        // views are active or during rotation.
        let syncPresentation = PlatformViewHandler.hasPlatformViews || syncPresentationForRotation
        metalLayer.presentsWithTransaction = syncPresentation

        // Step 3: Acquire drawable and composite into it.
        guard let drawable = metalLayer.nextDrawable() else { return false }

        renderer.renderSync(
            to: drawable,
            width: width,
            height: height,
            synchronous: syncPresentation
        )
        return true
    }

    // MARK: - Touch Handling

    /// Called when one or more fingers touch down on the screen.
    ///
    /// Converts the touch to a pointer event and forwards to the Go engine.
    ///
    /// - Parameters:
    ///   - touches: The set of touches that began.
    ///   - event: The event to which the touches belong.
    override func touchesBegan(_ touches: Set<UITouch>, with event: UIEvent?) {
        handleTouch(touches, phase: 0)  // Phase 0 = Down
    }

    /// Called when one or more fingers move on the screen.
    ///
    /// - Parameters:
    ///   - touches: The set of touches that moved.
    ///   - event: The event to which the touches belong.
    override func touchesMoved(_ touches: Set<UITouch>, with event: UIEvent?) {
        handleTouch(touches, phase: 1)  // Phase 1 = Move
    }

    /// Called when one or more fingers lift from the screen.
    ///
    /// - Parameters:
    ///   - touches: The set of touches that ended.
    ///   - event: The event to which the touches belong.
    override func touchesEnded(_ touches: Set<UITouch>, with event: UIEvent?) {
        handleTouch(touches, phase: 2)  // Phase 2 = Up
    }

    /// Called when the system cancels the touch sequence.
    ///
    /// This can happen due to system gestures, incoming calls, or other interruptions.
    ///
    /// - Parameters:
    ///   - touches: The set of touches that were cancelled.
    ///   - event: The event to which the touches belong.
    override func touchesCancelled(_ touches: Set<UITouch>, with event: UIEvent?) {
        handleTouch(touches, phase: 3)  // Phase 3 = Cancel
    }

    /// Converts UIKit touch events to Drift pointer events and forwards to Go.
    ///
    /// Handles coordinate conversion from points to pixels and calls the
    /// Go engine via FFI. Processes all touches for multi-touch support.
    ///
    /// - Parameters:
    ///   - touches: The set of touches to process.
    ///   - phase: The pointer phase (0=Down, 1=Move, 2=Up, 3=Cancel).
    private func handleTouch(_ touches: Set<UITouch>, phase: Int32) {
        // Get the scale factor for converting points to pixels.
        let scale = contentScaleFactor

        // Process all touches for multi-touch support.
        for touch in touches {
            let touchID = ObjectIdentifier(touch)
            let pointerID: Int64

            if phase == 0 {
                // Touch began: assign a new pointer ID
                pointerID = nextPointerID
                nextPointerID += 1
                touchToPointerID[touchID] = pointerID
            } else if let existingID = touchToPointerID[touchID] {
                // Existing touch: use the assigned ID
                pointerID = existingID
                if phase == 2 {
                    // Touch ended: remove from map
                    touchToPointerID.removeValue(forKey: touchID)
                }
            } else {
                // Unknown touch (shouldn't happen): skip
                continue
            }

            // Get the touch location in view coordinates (points).
            let location = touch.location(in: self)

            // Convert to pixels and call the Go engine.
            // The Go engine uses pixel coordinates matching the render buffer.
            DriftPointerEvent(pointerID, phase, Double(location.x * scale), Double(location.y * scale))
        }

        // On cancel, clear all tracked touches to avoid stale entries.
        // touchesCancelled may only deliver a subset of active touches.
        if phase == 3 {
            touchToPointerID.removeAll()
        }

        // Mark the engine dirty so the display link renders at the next vsync.
        // Calling renderFrame() here would double-render when the display link
        // also fires within this vsync.
        DriftRequestFrame()
    }
}
