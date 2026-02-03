package overlay

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/widgets"
)

// ModalBarrier prevents interaction with widgets behind it.
// Uses GestureDetector for tap handling and Semantics for accessibility.
// Always absorbs all hit tests (even when Dismissible=false).
type ModalBarrier struct {
	// Color is the barrier's background color (typically semi-transparent black).
	Color graphics.Color

	// Dismissible allows tapping the barrier to trigger OnDismiss.
	// When false, barrier still absorbs all touches but doesn't call OnDismiss.
	Dismissible bool

	// OnDismiss is called when barrier is tapped (if Dismissible=true).
	OnDismiss func()

	// SemanticLabel for accessibility (e.g., "Dismiss dialog").
	SemanticLabel string
}

func (b ModalBarrier) CreateElement() core.Element {
	return core.NewStatelessElement(b, nil)
}

func (b ModalBarrier) Key() any {
	return nil
}

func (b ModalBarrier) Build(ctx core.BuildContext) core.Widget {
	// The barrier itself - fills all available space with the given color
	barrier := barrierRender{color: b.Color}

	// Determine tap handler
	var onTap func()
	if b.Dismissible && b.OnDismiss != nil {
		onTap = b.OnDismiss
	} else {
		onTap = func() {} // No-op but still absorbs touch
	}

	// Wrap in GestureDetector to absorb touches
	result := widgets.GestureDetector{
		OnTap: onTap,
		Child: barrier,
	}

	// Add semantics for accessibility
	if b.SemanticLabel != "" {
		sem := widgets.Semantics{
			Label:     b.SemanticLabel,
			Container: true,
			Child:     result,
		}
		// Only expose OnDismiss to screen readers if barrier is dismissible
		if b.Dismissible {
			sem.OnDismiss = b.OnDismiss
		}
		return sem
	}

	return result
}

// barrierRender is a render widget that fills available space with a color
// and always absorbs hit tests.
type barrierRender struct {
	color graphics.Color
}

func (b barrierRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(b, nil)
}

func (b barrierRender) Key() any {
	return nil
}

func (b barrierRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderBarrier{color: b.color}
	r.SetSelf(r)
	return r
}

func (b barrierRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderBarrier); ok {
		r.color = b.color
		r.MarkNeedsPaint()
	}
}

type renderBarrier struct {
	layout.RenderBoxBase
	color graphics.Color
}

func (r *renderBarrier) PerformLayout() {
	// Fill all available space
	constraints := r.Constraints()
	r.SetSize(graphics.Size{
		Width:  constraints.MaxWidth,
		Height: constraints.MaxHeight,
	})
}

func (r *renderBarrier) Paint(ctx *layout.PaintContext) {
	if r.color == 0 {
		return
	}
	size := r.Size()
	paint := graphics.DefaultPaint()
	paint.Color = r.color
	ctx.Canvas.DrawRect(graphics.RectFromLTWH(0, 0, size.Width, size.Height), paint)
}

// HitTest always returns true to absorb all hits.
func (r *renderBarrier) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

func (r *renderBarrier) VisitChildren(visitor func(layout.RenderObject)) {
	// No children
}
