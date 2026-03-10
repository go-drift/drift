package engine

import (
	"encoding/binary"
	"math"
	"sync"
	"sync/atomic"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/platform"
)

// frameCounter provides monotonic frame IDs for snapshots.
var frameCounter atomic.Uint64

// FrameSnapshot captures the platform view geometry from a single frame.
// Serialized as a packed binary format and sent across the FFI boundary so
// the platform UI thread can position platform views synchronously before
// Skia renders.
type FrameSnapshot struct {
	FrameID uint64 // Go-side only; not included in binary wire format
	Views   []ViewSnapshot
}

// ViewSnapshot holds the resolved geometry for one platform view.
type ViewSnapshot struct {
	ViewID         int64
	X              float64
	Y              float64
	Width          float64
	Height         float64
	ClipLeft       float64
	ClipTop        float64
	ClipRight      float64
	ClipBottom     float64
	HasClip        bool
	Visible        bool
	VisibleLeft    float64
	VisibleTop     float64
	VisibleRight   float64
	VisibleBottom  float64
	OcclusionPaths []*graphics.Path
}

// viewSnapshotFromCapture converts a captured platform view geometry into a
// ViewSnapshot. A view is hidden when it has a zero-area clip and zero size
// (unseen during compositing).
func viewSnapshotFromCapture(cv platform.CapturedViewGeometry) ViewSnapshot {
	vs := ViewSnapshot{
		ViewID:        cv.ViewID,
		X:             cv.Offset.X,
		Y:             cv.Offset.Y,
		Width:         cv.Size.Width,
		Height:        cv.Size.Height,
		VisibleLeft:   cv.VisibleRect.Left,
		VisibleTop:    cv.VisibleRect.Top,
		VisibleRight:  cv.VisibleRect.Right,
		VisibleBottom: cv.VisibleRect.Bottom,
	}

	vs.OcclusionPaths = cv.OcclusionPaths
	vs.Visible = !cv.VisibleRect.IsEmpty()

	if cv.ClipBounds != nil {
		vs.HasClip = true
		vs.ClipLeft = cv.ClipBounds.Left
		vs.ClipTop = cv.ClipBounds.Top
		vs.ClipRight = cv.ClipBounds.Right
		vs.ClipBottom = cv.ClipBounds.Bottom
	}
	return vs
}

// Binary wire format v1 (little-endian). This is the canonical spec;
// decoders in UnifiedFrameOrchestrator.kt (Android) and DriftRenderer.swift
// (iOS) must match exactly.
//
//	Header (8 bytes): uint32 version (1), uint32 viewCount
//	Per view, fixed (60 bytes):
//	  int64 viewId
//	  float32 x, y, width, height
//	  float32 clipLeft, clipTop, clipRight, clipBottom
//	  float32 visibleLeft, visibleTop, visibleRight, visibleBottom
//	  uint8 flags (bit0=hasClip, bit1=visible), uint8 reserved, uint16 pathCount
//	Per occlusion path (variable):
//	  uint16 commandCount
//	  Per command: uint8 op (0=M,1=L,2=Q,3=C,4=Z), uint8 argCount, float32[argCount]
const (
	binaryVersion       = 1
	headerSize          = 8  // uint32 version + uint32 viewCount
	viewFixedSize       = 60 // int64 viewId + 12*float32 + 1 flags + 1 reserved + uint16 pathCount
	opMoveTo      uint8 = 0
	opLineTo      uint8 = 1
	opQuadTo      uint8 = 2
	opCubicTo     uint8 = 3
	opClose       uint8 = 4
)

// snapshotBufferPool reuses byte slices for binary encoding to avoid
// per-frame allocations on the hot path.
var snapshotBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, 1024)
		return &buf
	},
}

func getSnapshotBuffer() *[]byte {
	bp := snapshotBufferPool.Get().(*[]byte)
	*bp = (*bp)[:0]
	return bp
}

// PutSnapshotBuffer returns a buffer obtained from MarshalBinary to the pool.
// Call this after the encoded bytes have been copied (e.g. by C.CBytes).
func PutSnapshotBuffer(bp *[]byte) {
	if bp == nil {
		return
	}
	if cap(*bp) > 64*1024 {
		return
	}
	snapshotBufferPool.Put(bp)
}

// MarshalBinary encodes a FrameSnapshot into the packed binary wire format.
// Returns a pointer to a pooled byte slice. The caller must call
// PutSnapshotBuffer after copying the data.
func MarshalBinary(snap *FrameSnapshot) *[]byte {
	bp := getSnapshotBuffer()
	buf := *bp

	// Header: version + viewCount
	buf = appendUint32(buf, binaryVersion)
	buf = appendUint32(buf, uint32(len(snap.Views)))

	for i := range snap.Views {
		v := &snap.Views[i]

		// viewId (int64, 8 bytes)
		buf = appendInt64(buf, v.ViewID)

		// x, y, width, height (4 * float32 = 16 bytes)
		buf = appendFloat32(buf, float32(v.X))
		buf = appendFloat32(buf, float32(v.Y))
		buf = appendFloat32(buf, float32(v.Width))
		buf = appendFloat32(buf, float32(v.Height))

		// clipLeft, clipTop, clipRight, clipBottom (4 * float32 = 16 bytes)
		buf = appendFloat32(buf, float32(v.ClipLeft))
		buf = appendFloat32(buf, float32(v.ClipTop))
		buf = appendFloat32(buf, float32(v.ClipRight))
		buf = appendFloat32(buf, float32(v.ClipBottom))

		// visibleLeft, visibleTop, visibleRight, visibleBottom (4 * float32 = 16 bytes)
		buf = appendFloat32(buf, float32(v.VisibleLeft))
		buf = appendFloat32(buf, float32(v.VisibleTop))
		buf = appendFloat32(buf, float32(v.VisibleRight))
		buf = appendFloat32(buf, float32(v.VisibleBottom))

		// flags (1 byte): bit 0 = hasClip, bit 1 = visible
		var flags uint8
		if v.HasClip {
			flags |= 1
		}
		if v.Visible {
			flags |= 2
		}
		buf = append(buf, flags)

		// reserved (1 byte)
		buf = append(buf, 0)

		// pathCount (uint16)
		pathCount := len(v.OcclusionPaths)
		buf = appendUint16(buf, uint16(pathCount))

		// Variable-length occlusion paths
		for _, p := range v.OcclusionPaths {
			if p == nil {
				buf = appendUint16(buf, 0)
				continue
			}
			buf = appendUint16(buf, uint16(len(p.Commands)))
			for _, cmd := range p.Commands {
				switch cmd.Op {
				case graphics.PathOpMoveTo:
					buf = append(buf, opMoveTo, 2)
					buf = appendFloat32(buf, float32(cmd.Args[0]))
					buf = appendFloat32(buf, float32(cmd.Args[1]))
				case graphics.PathOpLineTo:
					buf = append(buf, opLineTo, 2)
					buf = appendFloat32(buf, float32(cmd.Args[0]))
					buf = appendFloat32(buf, float32(cmd.Args[1]))
				case graphics.PathOpQuadTo:
					buf = append(buf, opQuadTo, 4)
					buf = appendFloat32(buf, float32(cmd.Args[0]))
					buf = appendFloat32(buf, float32(cmd.Args[1]))
					buf = appendFloat32(buf, float32(cmd.Args[2]))
					buf = appendFloat32(buf, float32(cmd.Args[3]))
				case graphics.PathOpCubicTo:
					buf = append(buf, opCubicTo, 6)
					buf = appendFloat32(buf, float32(cmd.Args[0]))
					buf = appendFloat32(buf, float32(cmd.Args[1]))
					buf = appendFloat32(buf, float32(cmd.Args[2]))
					buf = appendFloat32(buf, float32(cmd.Args[3]))
					buf = appendFloat32(buf, float32(cmd.Args[4]))
					buf = appendFloat32(buf, float32(cmd.Args[5]))
				case graphics.PathOpClose:
					buf = append(buf, opClose, 0)
				}
			}
		}
	}

	*bp = buf
	return bp
}

func appendUint16(buf []byte, v uint16) []byte {
	return binary.LittleEndian.AppendUint16(buf, v)
}

func appendUint32(buf []byte, v uint32) []byte {
	return binary.LittleEndian.AppendUint32(buf, v)
}

func appendInt64(buf []byte, v int64) []byte {
	return binary.LittleEndian.AppendUint64(buf, uint64(v))
}

func appendFloat32(buf []byte, v float32) []byte {
	return binary.LittleEndian.AppendUint32(buf, math.Float32bits(v))
}
