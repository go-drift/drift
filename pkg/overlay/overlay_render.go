package overlay

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// overlayRender is a custom render widget that enforces Opaque hit testing.
type overlayRender struct {
	child   core.Widget
	entries []core.Widget
	opaque  int // index of first opaque entry in rendered list (-1 if none)
}

func (o overlayRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(o, nil)
}

func (o overlayRender) Key() any {
	return nil
}

func (o overlayRender) ChildrenWidgets() []core.Widget {
	result := make([]core.Widget, 0, len(o.entries)+1)
	if o.child != nil {
		result = append(result, o.child)
	}
	result = append(result, o.entries...)
	return result
}

func (o overlayRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderOverlay{
		opaqueIndex: o.opaque,
		hasChild:    o.child != nil,
	}
	r.SetSelf(r)
	return r
}

func (o overlayRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderOverlay); ok {
		r.opaqueIndex = o.opaque
		r.hasChild = o.child != nil
		r.MarkNeedsLayout()
	}
}

type renderOverlay struct {
	layout.RenderBoxBase
	child       layout.RenderBox   // The main content (route stack)
	entries     []layout.RenderBox // Overlay entries
	opaqueIndex int                // index of first opaque entry (-1 if none)
	hasChild    bool               // whether widget has a child
}

// SetChildren sets the child render objects.
// First child is the main content (if hasChild), rest are overlay entries.
func (r *renderOverlay) SetChildren(children []layout.RenderObject) {
	// Clear parent on old children
	if r.child != nil {
		setParentOnChild(r.child, nil)
	}
	for _, entry := range r.entries {
		setParentOnChild(entry, nil)
	}

	r.child = nil
	r.entries = nil

	if len(children) == 0 {
		return
	}

	startIdx := 0
	if r.hasChild {
		if box, ok := children[0].(layout.RenderBox); ok {
			r.child = box
			setParentOnChild(r.child, r)
		}
		startIdx = 1
	}

	r.entries = make([]layout.RenderBox, 0, len(children)-startIdx)
	for i := startIdx; i < len(children); i++ {
		if box, ok := children[i].(layout.RenderBox); ok {
			r.entries = append(r.entries, box)
			setParentOnChild(box, r)
		}
	}
}

// VisitChildren calls the visitor for each child.
func (r *renderOverlay) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
	for _, entry := range r.entries {
		visitor(entry)
	}
}

// PerformLayout computes the size of the overlay and positions children.
// Child gets passed constraints directly (fills overlay).
// Entries get loose constraints (can be smaller than overlay).
func (r *renderOverlay) PerformLayout() {
	constraints := r.Constraints()

	// Layout child with incoming constraints
	var size graphics.Size
	if r.child != nil {
		r.child.Layout(constraints, true)
		size = r.child.Size()
	} else {
		// Use max size from constraints
		size = graphics.Size{Width: constraints.MaxWidth, Height: constraints.MaxHeight}
	}
	r.SetSize(size)

	// Layout entries with loose constraints (entries can be any size)
	looseConstraints := layout.Loose(size)
	for _, entry := range r.entries {
		entry.Layout(looseConstraints, false)
		// Position at (0,0)
		entry.SetParentData(&layout.BoxParentData{})
	}

	// Set child parent data
	if r.child != nil {
		r.child.SetParentData(&layout.BoxParentData{})
	}
}

// Paint paints the child first (bottom), then entries in order (first = bottom, last = top).
func (r *renderOverlay) Paint(ctx *layout.PaintContext) {
	// Paint child first (bottom)
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child, graphics.Offset{})
	}
	// Paint entries in order (first = bottom, last = top)
	for _, entry := range r.entries {
		ctx.PaintChildWithLayer(entry, getChildOffset(entry))
	}
}

// HitTest tests entries top-to-bottom (last in slice = top of z-order).
// All overlay entries are tested regardless of opaque flags - this allows
// barriers below opaque content to still receive dismiss taps.
// When any entry is marked Opaque, hits don't pass through to the child
// (page content), but other overlay entries can still receive hits.
func (r *renderOverlay) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}

	// Test all overlay entries from top to bottom
	for i := len(r.entries) - 1; i >= 0; i-- {
		entry := r.entries[i]
		offset := getChildOffset(entry)
		local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
		if entry.HitTest(local, result) {
			return true
		}
	}

	// If any entry is opaque, don't test the child (page content)
	// This blocks hits from reaching the underlying page while still
	// allowing overlay entries (like barriers) to receive hits
	if r.opaqueIndex >= 0 {
		return false
	}

	// Test the child (route stack)
	if r.child != nil {
		offset := getChildOffset(r.child)
		local := graphics.Offset{X: position.X - offset.X, Y: position.Y - offset.Y}
		return r.child.HitTest(local, result)
	}

	return false
}

// getChildOffset extracts the offset from a child's parent data.
func getChildOffset(child layout.RenderBox) graphics.Offset {
	if child == nil {
		return graphics.Offset{}
	}
	if data, ok := child.ParentData().(*layout.BoxParentData); ok {
		return data.Offset
	}
	return graphics.Offset{}
}

// withinBounds checks if a position is within the given size.
func withinBounds(position graphics.Offset, size graphics.Size) bool {
	return position.X >= 0 && position.Y >= 0 && position.X <= size.Width && position.Y <= size.Height
}

// setParentOnChild sets the parent reference on a child render object.
func setParentOnChild(child, parent layout.RenderObject) {
	if child == nil {
		return
	}
	getter, _ := child.(interface{ Parent() layout.RenderObject })
	setter, ok := child.(interface{ SetParent(layout.RenderObject) })
	if !ok {
		return
	}
	currentParent := layout.RenderObject(nil)
	if getter != nil {
		currentParent = getter.Parent()
	}
	if currentParent == parent {
		return
	}
	setter.SetParent(parent)
	if currentParent != nil {
		if marker, ok := currentParent.(interface{ MarkNeedsLayout() }); ok {
			marker.MarkNeedsLayout()
		}
	}
	if parent != nil {
		if marker, ok := parent.(interface{ MarkNeedsLayout() }); ok {
			marker.MarkNeedsLayout()
		}
	}
}
