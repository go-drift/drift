package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// GestureDetector wraps a child widget with gesture recognition callbacks.
//
// GestureDetector supports multiple gesture types that can be used together:
//   - Tap: Simple tap/click detection via OnTap
//   - Pan: Free-form drag in any direction via OnPanStart/Update/End
//   - Horizontal drag: Constrained horizontal drag via OnHorizontalDrag*
//   - Vertical drag: Constrained vertical drag via OnVerticalDrag*
//
// Example (tap detection):
//
//	GestureDetector{
//	    OnTap: func() { handleTap() },
//	    Child: Container{Color: colors.Blue, Child: icon},
//	}
//
// Example (draggable widget):
//
//	GestureDetector{
//	    OnPanStart:  func(d DragStartDetails) { ... },
//	    OnPanUpdate: func(d DragUpdateDetails) { ... },
//	    OnPanEnd:    func(d DragEndDetails) { ... },
//	    Child:       draggableItem,
//	}
//
// For simple tap handling on buttons, prefer [Button] or [InkWell] which
// provide visual feedback. GestureDetector is best for custom gestures.
type GestureDetector struct {
	Child       core.Widget
	OnTap       func()
	OnPanStart  func(DragStartDetails)
	OnPanUpdate func(DragUpdateDetails)
	OnPanEnd    func(DragEndDetails)
	OnPanCancel func()

	OnHorizontalDragStart  func(DragStartDetails)
	OnHorizontalDragUpdate func(DragUpdateDetails)
	OnHorizontalDragEnd    func(DragEndDetails)
	OnHorizontalDragCancel func()

	OnVerticalDragStart  func(DragStartDetails)
	OnVerticalDragUpdate func(DragUpdateDetails)
	OnVerticalDragEnd    func(DragEndDetails)
	OnVerticalDragCancel func()
}

func (g GestureDetector) CreateElement() core.Element {
	return core.NewRenderObjectElement(g, nil)
}

func (g GestureDetector) Key() any {
	return nil
}

func (g GestureDetector) ChildWidget() core.Widget {
	return g.Child
}

func (g GestureDetector) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	detector := &renderGestureDetector{}
	detector.SetSelf(detector)
	detector.configure(g)
	return detector
}

func (g GestureDetector) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if detector, ok := renderObject.(*renderGestureDetector); ok {
		detector.configure(g)
		detector.MarkNeedsPaint()
	}
}

type renderGestureDetector struct {
	layout.RenderBoxBase
	child          layout.RenderBox
	tap            *gestures.TapGestureRecognizer
	pan            *gestures.PanGestureRecognizer
	horizontalDrag *gestures.HorizontalDragGestureRecognizer
	verticalDrag   *gestures.VerticalDragGestureRecognizer
}

func (r *renderGestureDetector) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
}

func (r *renderGestureDetector) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderGestureDetector) PerformLayout() {
	constraints := r.Constraints()
	if r.child == nil {
		r.SetSize(constraints.Constrain(graphics.Size{}))
		return
	}
	r.child.Layout(constraints, true) // true: we read child.Size()
	r.SetSize(r.child.Size())
	r.child.SetParentData(&layout.BoxParentData{})
}

func (r *renderGestureDetector) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child, graphics.Offset{})
	}
}

func (r *renderGestureDetector) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	if r.child != nil {
		r.child.HitTest(position, result)
	}
	result.Add(r)
	return true
}

func (r *renderGestureDetector) HandlePointer(event gestures.PointerEvent) {
	isDown := event.Phase == gestures.PointerPhaseDown
	if r.tap != nil {
		if isDown {
			r.tap.AddPointer(event)
		} else {
			r.tap.HandleEvent(event)
		}
	}
	if r.pan != nil {
		if isDown {
			r.pan.AddPointer(event)
		} else {
			r.pan.HandleEvent(event)
		}
	}
	if r.horizontalDrag != nil {
		if isDown {
			r.horizontalDrag.AddPointer(event)
		} else {
			r.horizontalDrag.HandleEvent(event)
		}
	}
	if r.verticalDrag != nil {
		if isDown {
			r.verticalDrag.AddPointer(event)
		} else {
			r.verticalDrag.HandleEvent(event)
		}
	}
}

func (r *renderGestureDetector) configure(g GestureDetector) {
	r.configureTap(g)
	r.configurePan(g)
	r.configureHorizontalDrag(g)
	r.configureVerticalDrag(g)
}

func (r *renderGestureDetector) configureTap(g GestureDetector) {
	if g.OnTap == nil {
		if r.tap != nil {
			r.tap.Dispose()
			r.tap = nil
		}
		return
	}
	if r.tap == nil {
		r.tap = gestures.NewTapGestureRecognizer(gestures.DefaultArena)
	}
	r.tap.OnTap = g.OnTap
}

func (r *renderGestureDetector) configurePan(g GestureDetector) {
	hasPanHandler := g.OnPanStart != nil || g.OnPanUpdate != nil || g.OnPanEnd != nil || g.OnPanCancel != nil
	// Don't use pan when axis-specific handlers are present (they would conflict)
	hasAxisHandler := g.OnHorizontalDragStart != nil || g.OnHorizontalDragUpdate != nil ||
		g.OnHorizontalDragEnd != nil || g.OnHorizontalDragCancel != nil ||
		g.OnVerticalDragStart != nil || g.OnVerticalDragUpdate != nil ||
		g.OnVerticalDragEnd != nil || g.OnVerticalDragCancel != nil
	if !hasPanHandler || hasAxisHandler {
		if r.pan != nil {
			r.pan.Dispose()
			r.pan = nil
		}
		if !hasPanHandler {
			return
		}
		// hasPanHandler && hasAxisHandler: axis handlers take precedence, skip pan
		return
	}
	if r.pan == nil {
		r.pan = gestures.NewPanGestureRecognizer(gestures.DefaultArena)
	}
	r.pan.OnStart = g.OnPanStart
	r.pan.OnUpdate = g.OnPanUpdate
	r.pan.OnEnd = g.OnPanEnd
	r.pan.OnCancel = g.OnPanCancel
}

func (r *renderGestureDetector) configureHorizontalDrag(g GestureDetector) {
	hasHandler := g.OnHorizontalDragStart != nil || g.OnHorizontalDragUpdate != nil ||
		g.OnHorizontalDragEnd != nil || g.OnHorizontalDragCancel != nil
	if !hasHandler {
		if r.horizontalDrag != nil {
			r.horizontalDrag.Dispose()
			r.horizontalDrag = nil
		}
		return
	}
	if r.horizontalDrag == nil {
		r.horizontalDrag = gestures.NewHorizontalDragGestureRecognizer(gestures.DefaultArena)
	}
	r.horizontalDrag.OnStart = g.OnHorizontalDragStart
	r.horizontalDrag.OnUpdate = g.OnHorizontalDragUpdate
	r.horizontalDrag.OnEnd = g.OnHorizontalDragEnd
	r.horizontalDrag.OnCancel = g.OnHorizontalDragCancel
}

func (r *renderGestureDetector) configureVerticalDrag(g GestureDetector) {
	hasHandler := g.OnVerticalDragStart != nil || g.OnVerticalDragUpdate != nil ||
		g.OnVerticalDragEnd != nil || g.OnVerticalDragCancel != nil
	if !hasHandler {
		if r.verticalDrag != nil {
			r.verticalDrag.Dispose()
			r.verticalDrag = nil
		}
		return
	}
	if r.verticalDrag == nil {
		r.verticalDrag = gestures.NewVerticalDragGestureRecognizer(gestures.DefaultArena)
	}
	r.verticalDrag.OnStart = g.OnVerticalDragStart
	r.verticalDrag.OnUpdate = g.OnVerticalDragUpdate
	r.verticalDrag.OnEnd = g.OnVerticalDragEnd
	r.verticalDrag.OnCancel = g.OnVerticalDragCancel
}

// DragStartDetails describes the start of a drag.
type DragStartDetails = gestures.DragStartDetails

// DragUpdateDetails describes a drag update.
type DragUpdateDetails = gestures.DragUpdateDetails

// DragEndDetails describes the end of a drag.
type DragEndDetails = gestures.DragEndDetails
