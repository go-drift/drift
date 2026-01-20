package layout

import (
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/rendering"
)

// HitTestResult collects hit test entries in paint order.
type HitTestResult struct {
	Entries []RenderObject
}

// Add inserts a render object into the hit test result list.
func (h *HitTestResult) Add(target RenderObject) {
	h.Entries = append(h.Entries, target)
}

// TapTarget is a render object that responds to tap events.
type TapTarget interface {
	OnTap()
}

// PointerHandler receives pointer events routed from hit testing.
type PointerHandler interface {
	HandlePointer(event gestures.PointerEvent)
}

// PaintContext provides the canvas for painting render objects.
type PaintContext struct {
	Canvas rendering.Canvas
}

// PaintChild paints a child render box at the given offset.
func (p *PaintContext) PaintChild(child RenderBox, offset rendering.Offset) {
	if child == nil {
		return
	}
	p.Canvas.Save()
	p.Canvas.Translate(offset.X, offset.Y)
	child.Paint(p)
	p.Canvas.Restore()
}
