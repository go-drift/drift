package engine

import (
	"sync/atomic"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/platform"
)

// frameCounter provides monotonic frame IDs for snapshots.
var frameCounter atomic.Uint64

// FrameSnapshot captures the platform view geometry from a single frame.
// Serialized as JSON and sent across the JNI boundary so the Android UI thread
// can position platform views synchronously before Skia renders.
type FrameSnapshot struct {
	FrameID uint64         `json:"frameId"`
	Views   []ViewSnapshot `json:"views"`
}

// ViewSnapshot holds the resolved geometry for one platform view.
type ViewSnapshot struct {
	ViewID     int64   `json:"viewId"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	ClipLeft   float64 `json:"clipLeft"`
	ClipTop    float64 `json:"clipTop"`
	ClipRight  float64 `json:"clipRight"`
	ClipBottom float64 `json:"clipBottom"`
	HasClip    bool    `json:"hasClip,omitempty"`
	Visible    bool    `json:"visible"`
	// Occlusion masking. Always present.
	VisibleLeft    float64   `json:"visibleLeft"`
	VisibleTop     float64   `json:"visibleTop"`
	VisibleRight   float64   `json:"visibleRight"`
	VisibleBottom  float64   `json:"visibleBottom"`
	OcclusionMasks [][][]any `json:"occlusionMasks"`
}

// viewSnapshotFromCapture converts a captured platform view geometry into a
// ViewSnapshot for JSON serialization. A view is hidden when it has a zero-area
// clip and zero size (unseen during compositing).
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

	vs.OcclusionMasks = occlusionMasksFromPaths(cv.OcclusionPaths)
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

// occlusionMasksFromPaths converts path-based occlusion masks to the wire format.
// Each path becomes a list of command arrays: [["M",x,y],["L",x,y],...,["Z"]].
// Returns an empty (non-nil) slice when input is empty, ensuring JSON serializes
// as [] rather than null.
func occlusionMasksFromPaths(paths []*graphics.Path) [][][]any {
	if len(paths) == 0 {
		return [][][]any{}
	}
	result := make([][][]any, len(paths))
	for i, p := range paths {
		var cmds [][]any
		for _, cmd := range p.Commands {
			switch cmd.Op {
			case graphics.PathOpMoveTo:
				cmds = append(cmds, []any{"M", cmd.Args[0], cmd.Args[1]})
			case graphics.PathOpLineTo:
				cmds = append(cmds, []any{"L", cmd.Args[0], cmd.Args[1]})
			case graphics.PathOpQuadTo:
				cmds = append(cmds, []any{"Q", cmd.Args[0], cmd.Args[1], cmd.Args[2], cmd.Args[3]})
			case graphics.PathOpCubicTo:
				cmds = append(cmds, []any{"C", cmd.Args[0], cmd.Args[1], cmd.Args[2], cmd.Args[3], cmd.Args[4], cmd.Args[5]})
			case graphics.PathOpClose:
				cmds = append(cmds, []any{"Z"})
			}
		}
		if cmds == nil {
			cmds = [][]any{}
		}
		result[i] = cmds
	}
	return result
}
