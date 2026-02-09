package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// LayoutBuilder defers child building to the layout phase, providing the parent's
// constraints to the builder function. This enables responsive layouts that adapt
// to available space.
//
// Because Drift's pipeline runs Build before Layout, widgets normally cannot
// observe constraints. LayoutBuilder bridges this gap by invoking the builder
// during the render object's PerformLayout, once actual constraints are known.
//
// Example:
//
//	LayoutBuilder{
//	    Builder: func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
//	        if constraints.MaxWidth > 600 {
//	            return wideLayout()
//	        }
//	        return narrowLayout()
//	    },
//	}
type LayoutBuilder struct {
	Builder func(ctx core.BuildContext, constraints layout.Constraints) core.Widget
}

// CreateElement returns a [core.LayoutBuilderElement] that defers child
// building to the layout phase.
func (lb LayoutBuilder) CreateElement() core.Element {
	return core.NewLayoutBuilderElement(lb, nil)
}

// Key returns nil. LayoutBuilder does not support keyed identity.
func (lb LayoutBuilder) Key() any {
	return nil
}

// CreateRenderObject creates the backing renderLayoutBuilder, which invokes
// the layout callback during PerformLayout and lays out the resulting child.
func (lb LayoutBuilder) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderLayoutBuilder{}
	r.SetSelf(r)
	return r
}

// UpdateRenderObject is a no-op. The render object's only mutable state is
// the layout callback, which is set directly by the element during Mount.
func (lb LayoutBuilder) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
}

// LayoutBuilder returns the builder callback. This satisfies the
// [core.LayoutBuilderWidget] interface so the element can retrieve the
// builder during the layout callback.
func (lb LayoutBuilder) LayoutBuilder() func(ctx core.BuildContext, constraints layout.Constraints) core.Widget {
	return lb.Builder
}

// renderLayoutBuilder is the render object for LayoutBuilder.
// It calls the layout callback during PerformLayout, then lays out and
// sizes itself to the child.
type renderLayoutBuilder struct {
	layout.RenderBoxBase
	child          layout.RenderBox
	layoutCallback func(layout.Constraints)
}

func (r *renderLayoutBuilder) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = setChildFromRenderObject(child)
	setParentOnChild(r.child, r)
	if r.child != nil {
		r.child.SetParentData(&layout.BoxParentData{})
	}
}

func (r *renderLayoutBuilder) SetLayoutCallback(fn func(layout.Constraints)) {
	r.layoutCallback = fn
}

func (r *renderLayoutBuilder) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderLayoutBuilder) PerformLayout() {
	constraints := r.Constraints()

	// Invoke the element's callback, which builds the child widget tree.
	if r.layoutCallback != nil {
		r.layoutCallback(constraints)
	}

	if r.child != nil {
		r.child.Layout(constraints, true)
		r.SetSize(constraints.Constrain(r.child.Size()))
	} else {
		r.SetSize(constraints.Constrain(graphics.Size{}))
	}
}

func (r *renderLayoutBuilder) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child, graphics.Offset{})
	}
}

func (r *renderLayoutBuilder) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return withinBounds(position, r.Size()) &&
		r.child != nil && r.child.HitTest(position, result)
}
