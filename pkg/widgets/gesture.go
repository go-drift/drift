package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// GestureDetector wraps a child with tap handling.
type GestureDetector struct {
	ChildWidget core.Widget
	OnTap       func()
	OnPanStart  func(DragStartDetails)
	OnPanUpdate func(DragUpdateDetails)
	OnPanEnd    func(DragEndDetails)
	OnPanCancel func()
}

func (g GestureDetector) CreateElement() core.Element {
	return core.NewRenderObjectElement(g, nil)
}

func (g GestureDetector) Key() any {
	return nil
}

func (g GestureDetector) Child() core.Widget {
	return g.ChildWidget
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
	child layout.RenderBox
	tap   *gestures.TapGestureRecognizer
	pan   *gestures.PanGestureRecognizer
}

func (r *renderGestureDetector) SetChild(child layout.RenderObject) {
	r.child = setChildFromRenderObject(child)
}

func (r *renderGestureDetector) Layout(constraints layout.Constraints) {
	if r.child == nil {
		r.SetSize(constraints.Constrain(rendering.Size{}))
		return
	}
	r.child.Layout(constraints)
	r.SetSize(r.child.Size())
	r.child.SetParentData(&layout.BoxParentData{})
}

func (r *renderGestureDetector) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChild(r.child, rendering.Offset{})
	}
}

func (r *renderGestureDetector) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
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
}

func (r *renderGestureDetector) configure(g GestureDetector) {
	r.configureTap(g)
	r.configurePan(g)
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
	if !hasPanHandler {
		if r.pan != nil {
			r.pan.Dispose()
			r.pan = nil
		}
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

// DragStartDetails describes the start of a drag.
type DragStartDetails = gestures.DragStartDetails

// DragUpdateDetails describes a drag update.
type DragUpdateDetails = gestures.DragUpdateDetails

// DragEndDetails describes the end of a drag.
type DragEndDetails = gestures.DragEndDetails
