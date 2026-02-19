//go:build !android && !darwin && !ios

package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/semantics"
)

// Semantics is a no-op widget on non-mobile platforms.
// It just passes through to its child.
type Semantics struct {
	Child                core.Widget
	Label                string
	Value                string
	Hint                 string
	Tooltip              string
	Role                 semantics.SemanticsRole
	Flags                semantics.SemanticsFlag
	Container            bool
	MergeDescendants     bool
	ExplicitChildNodes   bool
	CurrentValue         *float64
	MinValue             *float64
	MaxValue             *float64
	HeadingLevel         int
	OnTap                func()
	OnLongPress          func()
	OnScrollLeft         func()
	OnScrollRight        func()
	OnScrollUp           func()
	OnScrollDown         func()
	OnIncrease           func()
	OnDecrease           func()
	OnDismiss            func()
	CustomActions        []semantics.CustomSemanticsAction
	CustomActionHandlers map[int64]func()
}

func (s Semantics) CreateElement() core.Element {
	return core.NewRenderObjectElement()
}

func (s Semantics) Key() any {
	return nil
}

func (s Semantics) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderSemanticsStub{}
	r.SetSelf(r)
	return r
}

func (s Semantics) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
}

func (s Semantics) ChildWidget() core.Widget {
	return s.Child
}

type renderSemanticsStub struct {
	layout.RenderBoxBase
	child layout.RenderObject
}

func (r *renderSemanticsStub) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = child
	setParentOnChild(r.child, r)
}

func (r *renderSemanticsStub) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderSemanticsStub) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		r.child.Layout(constraints, true)
		r.SetSize(r.child.Size())
	} else {
		r.SetSize(constraints.Constrain(graphics.Size{}))
	}
}

func (r *renderSemanticsStub) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child.(layout.RenderBox), graphics.Offset{})
	}
}

func (r *renderSemanticsStub) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if r.child != nil {
		return r.child.HitTest(position, result)
	}
	return false
}

// MergeSemantics is a no-op widget on non-mobile platforms.
type MergeSemantics struct {
	Child core.Widget
}

func (m MergeSemantics) CreateElement() core.Element {
	return core.NewRenderObjectElement()
}

func (m MergeSemantics) Key() any {
	return nil
}

func (m MergeSemantics) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderMergeSemanticsStub{}
	r.SetSelf(r)
	return r
}

func (m MergeSemantics) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
}

func (m MergeSemantics) ChildWidget() core.Widget {
	return m.Child
}

type renderMergeSemanticsStub struct {
	layout.RenderBoxBase
	child layout.RenderObject
}

func (r *renderMergeSemanticsStub) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = child
	setParentOnChild(r.child, r)
}

func (r *renderMergeSemanticsStub) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderMergeSemanticsStub) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		r.child.Layout(constraints, true)
		r.SetSize(r.child.Size())
	} else {
		r.SetSize(constraints.Constrain(graphics.Size{}))
	}
}

func (r *renderMergeSemanticsStub) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child.(layout.RenderBox), graphics.Offset{})
	}
}

func (r *renderMergeSemanticsStub) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if r.child != nil {
		return r.child.HitTest(position, result)
	}
	return false
}

// ExcludeSemantics is a no-op on non-mobile platforms.
type ExcludeSemantics struct {
	Child     core.Widget
	Excluding bool
}

// NewExcludeSemantics creates an ExcludeSemantics widget (no-op on non-mobile platforms).
func NewExcludeSemantics(child core.Widget) ExcludeSemantics {
	return ExcludeSemantics{Excluding: true, Child: child}
}

func (e ExcludeSemantics) CreateElement() core.Element {
	return core.NewRenderObjectElement()
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

func (e ExcludeSemantics) ChildWidget() core.Widget {
	return e.Child
}

type renderExcludeSemanticsStub struct {
	layout.RenderBoxBase
	child layout.RenderObject
}

func (r *renderExcludeSemanticsStub) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	r.child = child
	setParentOnChild(r.child, r)
}

func (r *renderExcludeSemanticsStub) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderExcludeSemanticsStub) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		r.child.Layout(constraints, true) // true: we read child.Size()
		r.SetSize(r.child.Size())
	} else {
		r.SetSize(constraints.Constrain(graphics.Size{}))
	}
}

func (r *renderExcludeSemanticsStub) Paint(ctx *layout.PaintContext) {
	if r.child != nil {
		ctx.PaintChildWithLayer(r.child.(layout.RenderBox), graphics.Offset{})
	}
}

func (r *renderExcludeSemanticsStub) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if r.child != nil {
		return r.child.HitTest(position, result)
	}
	return false
}
