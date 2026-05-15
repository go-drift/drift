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

/// Platform view geometry from a single frame, decoded from packed binary.
struct FrameSnapshot {
    let views: [ViewSnapshot]
}

/// Geometry for a single platform view within a frame snapshot.
struct ViewSnapshot {
    let viewId: Int
    let x: Float
    let y: Float
    let width: Float
    let height: Float
    let clipLeft: Float?
    let clipTop: Float?
    let clipRight: Float?
    let clipBottom: Float?
    let visible: Bool
    let visibleLeft: Float
    let visibleTop: Float
    let visibleRight: Float
    let visibleBottom: Float
    let occlusionPaths: [CGPath]
}

/// Metal renderer that bridges Go engine output to the iOS display.
final class DriftRenderer {

    /// The Metal device used for creating resources.
    let device: MTLDevice

    /// The command queue for presenting drawables.
    private let commandQueue: MTLCommandQueue

    /// One-shot guard for the first-frame post-present event. Set to true the
    /// first time a drawable's `addPresentedHandler` fires. Subsequent frames
    /// skip registration so we don't emit `first_frame` more than once.
    private var firstFrameRendered = false

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
        let rawData = Data(bytes: data, count: Int(outLen))
        return decodeSnapshot(rawData)
    }

    /// Composites the recorded display lists into the Metal texture and presents
    /// the drawable. This is the second half of the split pipeline.
    func renderSync(to drawable: CAMetalDrawable, width: Int32, height: Int32, synchronous: Bool = false) {
        guard width > 0, height > 0 else { return }

        let texturePtr = UInt(bitPattern: Unmanaged.passUnretained(drawable.texture).toOpaque())
        let result = DriftSkiaRenderMetalSync(width, height, texturePtr)
        guard result == 0 else { return }

        guard let commandBuffer = commandQueue.makeCommandBuffer() else { return }

        // Register a one-shot post-present handler that fires `first_frame`
        // on the drift/rendering/frame_events channel. Drawable presented
        // handlers fire after the buffer is actually on screen, so this is
        // the most accurate "user can see pixels" signal iOS exposes.
        if !firstFrameRendered {
            firstFrameRendered = true
            drawable.addPresentedHandler { _ in
                PlatformChannelManager.shared.sendEvent(
                    "drift/rendering/frame_events",
                    data: ["type": "first_frame"]
                )
            }
        }

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

// MARK: - Binary Snapshot Decoder

/// Decodes a packed binary frame snapshot.
///
/// Wire format (v1, little-endian):
///   Header: uint32 version, uint32 viewCount
///   Per view (60 bytes fixed):
///     int64 viewId, 4xfloat32 pos/size, 4xfloat32 clip, 4xfloat32 visible,
///     uint8 flags, uint8 reserved, uint16 pathCount
///   Per occlusion path (variable):
///     uint16 commandCount
///     Per command: uint8 op, uint8 argCount, float32[argCount] args
private func decodeSnapshot(_ data: Data) -> FrameSnapshot? {
    return data.withUnsafeBytes { raw in
        let count = raw.count
        var offset = 0

        // Header
        guard count >= 8 else { return nil }
        let version: UInt32 = raw.loadUnaligned(fromByteOffset: offset, as: UInt32.self)
        offset += 4
        guard version == 1 else { return nil }
        let viewCount: UInt32 = raw.loadUnaligned(fromByteOffset: offset, as: UInt32.self)
        offset += 4

        var views: [ViewSnapshot] = []
        views.reserveCapacity(Int(viewCount))

        for _ in 0..<viewCount {
            // Fixed part: 60 bytes
            guard offset + 60 <= count else { return nil }

            let viewId: Int64 = raw.loadUnaligned(fromByteOffset: offset, as: Int64.self)
            offset += 8
            let x: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
            offset += 4
            let y: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
            offset += 4
            let w: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
            offset += 4
            let h: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
            offset += 4
            let clipL: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
            offset += 4
            let clipT: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
            offset += 4
            let clipR: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
            offset += 4
            let clipB: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
            offset += 4
            let visL: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
            offset += 4
            let visT: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
            offset += 4
            let visR: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
            offset += 4
            let visB: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
            offset += 4
            let flags: UInt8 = raw.loadUnaligned(fromByteOffset: offset, as: UInt8.self)
            offset += 1
            offset += 1 // reserved
            let pathCount: UInt16 = raw.loadUnaligned(fromByteOffset: offset, as: UInt16.self)
            offset += 2

            let hasClip = (flags & 1) != 0
            let visible = (flags & 2) != 0

            // Decode occlusion paths with view-local coordinate transform
            var occPaths: [CGPath] = []
            if pathCount > 0 {
                occPaths.reserveCapacity(Int(pathCount))
                for _ in 0..<pathCount {
                    guard offset + 2 <= count else { return nil }
                    let cmdCount: UInt16 = raw.loadUnaligned(fromByteOffset: offset, as: UInt16.self)
                    offset += 2

                    let path = CGMutablePath()
                    for _ in 0..<cmdCount {
                        guard offset + 2 <= count else { return nil }
                        let op: UInt8 = raw.loadUnaligned(fromByteOffset: offset, as: UInt8.self)
                        offset += 1
                        let argCount: UInt8 = raw.loadUnaligned(fromByteOffset: offset, as: UInt8.self)
                        offset += 1
                        let argBytes = Int(argCount) * 4
                        guard offset + argBytes <= count else { return nil }

                        switch op {
                        case 0: // MoveTo
                            let px: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
                            let py: Float = raw.loadUnaligned(fromByteOffset: offset + 4, as: Float.self)
                            path.move(to: CGPoint(x: CGFloat(px) - CGFloat(x), y: CGFloat(py) - CGFloat(y)))
                        case 1: // LineTo
                            let px: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
                            let py: Float = raw.loadUnaligned(fromByteOffset: offset + 4, as: Float.self)
                            path.addLine(to: CGPoint(x: CGFloat(px) - CGFloat(x), y: CGFloat(py) - CGFloat(y)))
                        case 2: // QuadTo
                            let x1: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
                            let y1: Float = raw.loadUnaligned(fromByteOffset: offset + 4, as: Float.self)
                            let x2: Float = raw.loadUnaligned(fromByteOffset: offset + 8, as: Float.self)
                            let y2: Float = raw.loadUnaligned(fromByteOffset: offset + 12, as: Float.self)
                            path.addQuadCurve(
                                to: CGPoint(x: CGFloat(x2) - CGFloat(x), y: CGFloat(y2) - CGFloat(y)),
                                control: CGPoint(x: CGFloat(x1) - CGFloat(x), y: CGFloat(y1) - CGFloat(y)))
                        case 3: // CubicTo
                            let x1: Float = raw.loadUnaligned(fromByteOffset: offset, as: Float.self)
                            let y1: Float = raw.loadUnaligned(fromByteOffset: offset + 4, as: Float.self)
                            let x2: Float = raw.loadUnaligned(fromByteOffset: offset + 8, as: Float.self)
                            let y2: Float = raw.loadUnaligned(fromByteOffset: offset + 12, as: Float.self)
                            let x3: Float = raw.loadUnaligned(fromByteOffset: offset + 16, as: Float.self)
                            let y3: Float = raw.loadUnaligned(fromByteOffset: offset + 20, as: Float.self)
                            path.addCurve(
                                to: CGPoint(x: CGFloat(x3) - CGFloat(x), y: CGFloat(y3) - CGFloat(y)),
                                control1: CGPoint(x: CGFloat(x1) - CGFloat(x), y: CGFloat(y1) - CGFloat(y)),
                                control2: CGPoint(x: CGFloat(x2) - CGFloat(x), y: CGFloat(y2) - CGFloat(y)))
                        case 4: // Close
                            path.closeSubpath()
                        default:
                            // Unknown op: skip argCount * 4 bytes
                            break
                        }
                        offset += argBytes
                    }
                    occPaths.append(path)
                }
            }

            views.append(ViewSnapshot(
                viewId: Int(viewId),
                x: x,
                y: y,
                width: w,
                height: h,
                clipLeft: hasClip ? clipL : nil,
                clipTop: hasClip ? clipT : nil,
                clipRight: hasClip ? clipR : nil,
                clipBottom: hasClip ? clipB : nil,
                visible: visible,
                visibleLeft: visL,
                visibleTop: visT,
                visibleRight: visR,
                visibleBottom: visB,
                occlusionPaths: occPaths
            ))
        }

        return FrameSnapshot(views: views)
    }
}
