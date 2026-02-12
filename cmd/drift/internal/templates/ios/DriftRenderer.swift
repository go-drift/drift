/// DriftRenderer.swift
/// Metal-based renderer that displays content from the Go Drift engine on iOS.
///
/// This renderer initializes a Skia Metal context and asks the Go engine to draw
/// directly into the CAMetalDrawable's texture.

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

/// FFI declaration for the split pipeline render function (composite only).
@_silgen_name("DriftSkiaRenderMetalSync")
func DriftSkiaRenderMetalSync(
    _ width: Int32,
    _ height: Int32,
    _ texture: UInt
) -> Int32

/// FFI declaration for running the engine pipeline and returning geometry snapshot.
@_silgen_name("DriftStepAndSnapshot")
func DriftStepAndSnapshot(
    _ width: Int32,
    _ height: Int32,
    _ outData: UnsafeMutablePointer<UnsafeMutablePointer<CChar>?>,
    _ outLen: UnsafeMutablePointer<Int32>
) -> Int32

// MARK: - Frame Snapshot Types

/// Platform view geometry from a single frame, decoded from JSON.
struct FrameSnapshot: Decodable {
    let views: [ViewSnapshot]
}

/// Geometry for a single platform view within a frame snapshot.
struct ViewSnapshot: Decodable {
    let viewId: Int
    let x: Double
    let y: Double
    let width: Double
    let height: Double
    let clipLeft: Double?
    let clipTop: Double?
    let clipRight: Double?
    let clipBottom: Double?
    let visible: Bool
}

/// Metal renderer that bridges Go engine output to the iOS display.
final class DriftRenderer {

    /// The Metal device used for creating resources.
    let device: MTLDevice

    /// The command queue for presenting drawables.
    private let commandQueue: MTLCommandQueue

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
        if DriftSkiaInitMetal(devicePtr, queuePtr) != 0 {
            fatalError("Failed to initialize Skia Metal backend")
        }
    }

    /// Runs the engine pipeline (build, layout, record) and returns the platform
    /// view geometry snapshot. This is the first half of the split pipeline.
    func stepAndSnapshot(width: Int32, height: Int32) -> FrameSnapshot? {
        var outData: UnsafeMutablePointer<CChar>? = nil
        var outLen: Int32 = 0
        let result = DriftStepAndSnapshot(width, height, &outData, &outLen)
        guard result == 0 else { return nil }
        guard let data = outData, outLen > 0 else { return nil }
        defer { free(data) }
        let jsonData = Data(bytes: data, count: Int(outLen))
        return try? JSONDecoder().decode(FrameSnapshot.self, from: jsonData)
    }

    /// Composites the recorded display lists into the Metal texture and presents
    /// the drawable. This is the second half of the split pipeline.
    func renderSync(to drawable: CAMetalDrawable, width: Int32, height: Int32, synchronous: Bool = false) {
        guard width > 0, height > 0 else { return }

        let texturePtr = UInt(bitPattern: Unmanaged.passUnretained(drawable.texture).toOpaque())
        let result = DriftSkiaRenderMetalSync(width, height, texturePtr)
        guard result == 0 else { return }

        guard let commandBuffer = commandQueue.makeCommandBuffer() else { return }
        if synchronous {
            commandBuffer.commit()
            commandBuffer.waitUntilScheduled()
            drawable.present()
        } else {
            commandBuffer.present(drawable)
            commandBuffer.commit()
        }
    }

}
