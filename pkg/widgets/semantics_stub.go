//go:build !android && !darwin && !ios
// +build !android,!darwin,!ios

package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
)

// ExcludeSemantics is a no-op on non-mobile platforms.
type ExcludeSemantics struct {
	ChildWidget core.Widget
	Excluding   bool
}

// NewExcludeSemantics creates an ExcludeSemantics widget (no-op on non-mobile platforms).
func NewExcludeSemantics(child core.Widget) ExcludeSemantics {
	return ExcludeSemantics{Excluding: true, ChildWidget: child}
}

func (e ExcludeSemantics) CreateElement() core.Element {
	return core.NewRenderObjectElement(e, nil)
}

func (e ExcludeSemantics) Key() any {
	return nil
}

func (e ExcludeSemantics) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderExcludeSemanticsStub{}
	r.SetSelf(r)
	return r
}

func (e ExcludeSemantics) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
}

func (e ExcludeSemantics) Child() core.Widget {
	return e.ChildWidget
}

type renderExcludeSemanticsStub struct {
	layout.RenderBoxBase
	child layout.RenderObject
}

func (r *renderExcludeSemanticsStub) SetChild(child layout.RenderObject) {
	r.child = child
}

func (r *renderExcludeSemanticsStub) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderExcludeSemanticsStub) Layout(constraints layout.Constraints) {
	if r.child != nil {
		r.child.Layout(constraints)
		r.SetSize(r.child.Size())
	} else {
		r.SetSize(constraints.Constrain(rendering.Size{}))
	}
}

func (r *renderExcludeSemanticsStub) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChild(r.child.(layout.RenderBox), rendering.Offset{})
	}
}

func (r *renderExcludeSemanticsStub) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if r.child != nil {
		return r.child.HitTest(position, result)
	}
	return false
}
