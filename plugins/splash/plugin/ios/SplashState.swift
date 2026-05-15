/// SplashState.swift
///
/// Thread-safe shared state for the Drift splash plugin. The Go-side runtime
/// adjusts the preserve count via the `drift/splash` channel; the first-frame
/// event from `drift/rendering/frame_events` flips the rendered flag. Both
/// inputs feed `canDismiss()` which the overlay (and the Android 12+
/// controller, via its Kotlin twin) polls before tearing down.

import Foundation

final class DriftSplashState {
    static let shared = DriftSplashState()

    private let lock = NSLock()
    private var preserveCount = 0
    private var firstFrameRenderedFlag = false

    private init() {}

    /// Applies a signed delta to the preserve count. Clamps at zero so a
    /// `Remove()` before any `Preserve()` is end-to-end a no-op. This is the
    /// single count-mutation entry point; the Go runtime always ships signed
    /// deltas via `apply_delta`.
    func apply(_ delta: Int) {
        lock.lock()
        defer { lock.unlock() }
        preserveCount = max(0, preserveCount + delta)
    }

    /// Marks the first frame as rendered. Idempotent; subsequent calls are
    /// no-ops. The first_frame event fires post-present so flipping this
    /// flag is the signal that pixels are on screen.
    func markFirstFrame() {
        lock.lock()
        defer { lock.unlock() }
        firstFrameRenderedFlag = true
    }

    /// Returns true if and only if a frame has been presented *and* no
    /// outstanding `Preserve()` is holding the splash up.
    func canDismiss() -> Bool {
        lock.lock()
        defer { lock.unlock() }
        return firstFrameRenderedFlag && preserveCount == 0
    }
}
