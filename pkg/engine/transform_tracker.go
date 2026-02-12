package engine

import "github.com/go-drift/drift/pkg/graphics"

// transformTracker maintains translation and clip state for canvas implementations
// that need to resolve platform view geometry in global coordinates.
// Embedded by both CompositingCanvas and GeometryCanvas to avoid duplicating
// the save/restore/translate/clip logic.
type transformTracker struct {
	transform graphics.Offset
	saveStack []trackerSaveState
	clips     []graphics.Rect
}

type trackerSaveState struct {
	transform graphics.Offset
	clipDepth int
}

func (t *transformTracker) save() {
	t.saveStack = append(t.saveStack, trackerSaveState{
		transform: t.transform,
		clipDepth: len(t.clips),
	})
}

func (t *transformTracker) restore() {
	if len(t.saveStack) > 0 {
		state := t.saveStack[len(t.saveStack)-1]
		t.saveStack = t.saveStack[:len(t.saveStack)-1]
		t.transform = state.transform
		t.clips = t.clips[:state.clipDepth]
	}
}

func (t *transformTracker) translate(dx, dy float64) {
	t.transform.X += dx
	t.transform.Y += dy
}

func (t *transformTracker) clipRect(rect graphics.Rect) {
	globalRect := rect.Translate(t.transform.X, t.transform.Y)
	if len(t.clips) > 0 {
		globalRect = t.clips[len(t.clips)-1].Intersect(globalRect)
	}
	t.clips = append(t.clips, globalRect)
}

func (t *transformTracker) clipRRect(rrect graphics.RRect) {
	globalRect := rrect.Rect.Translate(t.transform.X, t.transform.Y)
	if len(t.clips) > 0 {
		globalRect = t.clips[len(t.clips)-1].Intersect(globalRect)
	}
	t.clips = append(t.clips, globalRect)
}

// currentClip returns the active clip bounds, or nil if no clip is active.
func (t *transformTracker) currentClip() *graphics.Rect {
	if len(t.clips) > 0 {
		clip := t.clips[len(t.clips)-1]
		return &clip
	}
	return nil
}

// embedPlatformView resolves the current transform and clip state and reports
// the platform view geometry to the sink.
func (t *transformTracker) embedPlatformView(sink PlatformViewSink, viewID int64, size graphics.Size) {
	if sink == nil {
		return
	}
	offset := t.transform
	clipBounds := t.currentClip()
	sink.UpdateViewGeometry(viewID, offset, size, clipBounds)
}
