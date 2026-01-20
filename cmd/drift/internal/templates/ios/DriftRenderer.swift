/// DriftRenderer.swift
/// Metal-based renderer that displays content from the Go Drift engine on iOS.
///
/// This renderer initializes a Skia Metal context and asks the Go engine to draw
/// directly into the CAMetalDrawable's texture.

import CoreGraphics
import Metal
import QuartzCore

/// FFI declaration for the Go app initializer.
@_silgen_name("DriftAppInit")
func DriftAppInit() -> Int32

/// FFI declaration for the Go Skia initialization function.
@_silgen_name("DriftSkiaInitMetal")
func DriftSkiaInitMetal(
    _ device: UInt,
    _ queue: UInt
) -> Int32

/// FFI declaration for the Go Skia render function.
@_silgen_name("DriftSkiaRenderMetal")
func DriftSkiaRenderMetal(
    _ width: Int32,
    _ height: Int32,
    _ texture: UInt
) -> Int32

/// Metal renderer that bridges Go engine output to the iOS display.
final class DriftRenderer {

    /// The Metal device used for creating resources.
    let device: MTLDevice

    /// The command queue for presenting drawables.
    private let commandQueue: MTLCommandQueue

    /// Whether Skia successfully initialized.
    private var skiaReady = false

    /// Initializes the renderer with the default Metal device.
    init() {
        guard let device = MTLCreateSystemDefaultDevice(),
              let queue = device.makeCommandQueue() else {
            fatalError("Metal not available")
        }

        self.device = device
        self.commandQueue = queue

        if DriftAppInit() != 0 {
            fatalError("Failed to initialize Drift app")
        }

        // Register the native method handler so Go can call Swift platform channels
        DriftPlatformRegisterHandler()

        let devicePtr = UInt(bitPattern: Unmanaged.passUnretained(device).toOpaque())
        let queuePtr = UInt(bitPattern: Unmanaged.passUnretained(queue).toOpaque())
        skiaReady = (DriftSkiaInitMetal(devicePtr, queuePtr) == 0)
    }

    /// Renders a frame and presents it to the drawable.
    func draw(to drawable: CAMetalDrawable, size: CGSize, scale: CGFloat) {
        guard skiaReady else { return }

        let width = Int(size.width * scale)
        let height = Int(size.height * scale)
        guard width > 0, height > 0 else { return }

        let texturePtr = UInt(bitPattern: Unmanaged.passUnretained(drawable.texture).toOpaque())
        let result = DriftSkiaRenderMetal(Int32(width), Int32(height), texturePtr)
        guard result == 0 else { return }

        guard let commandBuffer = commandQueue.makeCommandBuffer() else { return }
        commandBuffer.present(drawable)
        commandBuffer.commit()
        // Triple buffering handles frame pacing naturally - no need to wait.
    }
}
