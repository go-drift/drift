package engine

import (
	"sync/atomic"

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
}

// viewSnapshotFromCapture converts a captured platform view geometry into a
// ViewSnapshot for JSON serialization. A view is hidden when it has a zero-area
// clip and zero size (unseen during compositing).
func viewSnapshotFromCapture(cv platform.CapturedViewGeometry) ViewSnapshot {
	vs := ViewSnapshot{
		ViewID: cv.ViewID,
		X:      cv.Offset.X,
		Y:      cv.Offset.Y,
		Width:  cv.Size.Width,
		Height: cv.Size.Height,
	}
	if cv.ClipBounds != nil {
		isEmpty := cv.ClipBounds.Left == 0 && cv.ClipBounds.Top == 0 &&
			cv.ClipBounds.Right == 0 && cv.ClipBounds.Bottom == 0
		if isEmpty && cv.Size.Width == 0 && cv.Size.Height == 0 {
			vs.Visible = false
		} else {
			vs.Visible = true
			vs.HasClip = true
			vs.ClipLeft = cv.ClipBounds.Left
			vs.ClipTop = cv.ClipBounds.Top
			vs.ClipRight = cv.ClipBounds.Right
			vs.ClipBottom = cv.ClipBounds.Bottom
		}
	} else {
		vs.Visible = true
	}
	return vs
}
