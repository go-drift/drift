package layout

import (
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/graphics"
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
	Canvas           graphics.Canvas
	clipStack        []graphics.Rect   // Each entry is already-intersected global clip
	transformStack   []graphics.Offset // Stack of translation deltas
	transform        graphics.Offset   // Current accumulated translation
	ShowLayoutBounds bool              // Debug flag to draw bounds around widgets
	debugDepth       int               // For color cycling in debug bounds
	DebugStrokeWidth float64           // Scaled stroke width (0 = use default 1.0)
}

// PushTranslation adds a translation delta to the stack.
func (p *PaintContext) PushTranslation(dx, dy float64) {
	p.transformStack = append(p.transformStack, graphics.Offset{X: dx, Y: dy})
	p.transform.X += dx
	p.transform.Y += dy
}

// PopTranslation removes the most recent translation from the stack.
func (p *PaintContext) PopTranslation() {
	if len(p.transformStack) == 0 {
		return
	}
	last := p.transformStack[len(p.transformStack)-1]
	p.transformStack = p.transformStack[:len(p.transformStack)-1]
	p.transform.X -= last.X
	p.transform.Y -= last.Y
}

// PushClipRect pushes a clip rectangle (in local coordinates).
// The rect is transformed to global coordinates and intersected with current clip.
func (p *PaintContext) PushClipRect(localRect graphics.Rect) {
	// Transform local rect to global coordinates
	globalRect := localRect.Translate(p.transform.X, p.transform.Y)

	// Intersect with current effective clip (if any)
	if len(p.clipStack) > 0 {
		globalRect = p.clipStack[len(p.clipStack)-1].Intersect(globalRect)
	}

	p.clipStack = append(p.clipStack, globalRect)
}

// PopClipRect removes the most recent clip rectangle.
func (p *PaintContext) PopClipRect() {
	if len(p.clipStack) > 0 {
		p.clipStack = p.clipStack[:len(p.clipStack)-1]
	}
}

// CurrentClipBounds returns the effective clip in global coordinates.
// Returns (clip, true) if a clip is active, (Rect{}, false) if not.
func (p *PaintContext) CurrentClipBounds() (graphics.Rect, bool) {
	if len(p.clipStack) == 0 {
		return graphics.Rect{}, false
	}
	return p.clipStack[len(p.clipStack)-1], true
}

// CurrentTransform returns the accumulated translation offset.
func (p *PaintContext) CurrentTransform() graphics.Offset {
	return p.transform
}

// PaintChild paints a child render box at the given offset.
func (p *PaintContext) PaintChild(child RenderBox, offset graphics.Offset) {
	if child == nil {
		return
	}
	p.Canvas.Save()
	p.Canvas.Translate(offset.X, offset.Y)
	p.PushTranslation(offset.X, offset.Y)

	if p.ShowLayoutBounds {
		p.debugDepth++
	}

	child.Paint(p)

	// Draw bounds after child paints so overlay is visible on top
	if p.ShowLayoutBounds {
		p.drawDebugBounds(child.Size())
		p.debugDepth--
	}

	p.PopTranslation()
	p.Canvas.Restore()
}

// PaintChildWithLayer paints a child, using its cached layer if available.
func (p *PaintContext) PaintChildWithLayer(child RenderBox, offset graphics.Offset) {
	if child == nil {
		return
	}

	p.Canvas.Save()
	p.Canvas.Translate(offset.X, offset.Y)
	p.PushTranslation(offset.X, offset.Y)

	if p.ShowLayoutBounds {
		p.debugDepth++
	}

	// Use cached layer if child is a repaint boundary with valid cache
	if boundary, ok := child.(interface {
		IsRepaintBoundary() bool
		Layer() *graphics.DisplayList
		NeedsPaint() bool
	}); ok && boundary.IsRepaintBoundary() {
		if layer := boundary.Layer(); layer != nil && !boundary.NeedsPaint() {
			layer.Paint(p.Canvas)
			// Draw bounds after layer paints so overlay is visible on top
			if p.ShowLayoutBounds {
				p.drawDebugBounds(child.Size())
				p.debugDepth--
			}
			p.PopTranslation()
			p.Canvas.Restore()
			return
		}
	}

	child.Paint(p)

	// Draw bounds after child paints so overlay is visible on top
	if p.ShowLayoutBounds {
		p.drawDebugBounds(child.Size())
		p.debugDepth--
	}

	p.PopTranslation()
	p.Canvas.Restore()
}

// debugBoundsColors cycles through colors by depth for visual distinction.
var debugBoundsColors = []graphics.Color{
	graphics.RGBA(255, 100, 100, 0.71), // Red
	graphics.RGBA(100, 255, 100, 0.71), // Green
	graphics.RGBA(100, 100, 255, 0.71), // Blue
	graphics.RGBA(255, 255, 100, 0.71), // Yellow
	graphics.RGBA(255, 100, 255, 0.71), // Magenta
	graphics.RGBA(100, 255, 255, 0.71), // Cyan
}

// drawDebugBounds draws a colored border around the given size for debugging.
func (p *PaintContext) drawDebugBounds(size graphics.Size) {
	if size.Width <= 0 || size.Height <= 0 {
		return
	}

	color := debugBoundsColors[p.debugDepth%len(debugBoundsColors)]

	strokeWidth := p.DebugStrokeWidth
	if strokeWidth <= 0 {
		strokeWidth = 1.0
	}

	rect := graphics.RectFromLTWH(0, 0, size.Width, size.Height)
	p.Canvas.DrawRect(rect, graphics.Paint{
		Color:       color,
		Style:       graphics.PaintStyleStroke,
		StrokeWidth: strokeWidth,
		BlendMode:   graphics.BlendModeSrcOver,
		Alpha:       1.0,
	})
}
