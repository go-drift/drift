package navigation

import (
	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/semantics"
)

// SlideDirection determines the direction of a slide transition.
type SlideDirection int

const (
	// SlideFromRight slides content in from the right.
	SlideFromRight SlideDirection = iota
	// SlideFromLeft slides content in from the left.
	SlideFromLeft
	// SlideFromBottom slides content in from the bottom.
	SlideFromBottom
	// SlideFromTop slides content in from the top.
	SlideFromTop
)

// SlideTransition animates a child sliding from a direction.
type SlideTransition struct {
	Animation   *animation.AnimationController
	Direction   SlideDirection
	ChildWidget core.Widget
}

// CreateElement returns a RenderObjectElement for this SlideTransition.
func (s SlideTransition) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

// Key returns nil (no key).
func (s SlideTransition) Key() any {
	return nil
}

// Child returns the child widget.
func (s SlideTransition) Child() core.Widget {
	return s.ChildWidget
}

// CreateRenderObject creates the RenderSlideTransition.
func (s SlideTransition) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	slide := &renderSlideTransition{
		animation: s.Animation,
		direction: s.Direction,
	}
	slide.SetSelf(slide)
	if s.Animation != nil {
		s.Animation.AddListener(func() {
			slide.MarkNeedsPaint()
		})
	}
	return slide
}

// UpdateRenderObject updates the RenderSlideTransition.
func (s SlideTransition) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if slide, ok := renderObject.(*renderSlideTransition); ok {
		slide.animation = s.Animation
		slide.direction = s.Direction
		slide.MarkNeedsPaint()
	}
}

type renderSlideTransition struct {
	layout.RenderBoxBase
	child     layout.RenderBox
	animation *animation.AnimationController
	direction SlideDirection
}

func (r *renderSlideTransition) SetChild(child layout.RenderObject) {
	if child == nil {
		r.child = nil
		return
	}
	if box, ok := child.(layout.RenderBox); ok {
		r.child = box
	}
}

func (r *renderSlideTransition) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

// DescribeSemanticsConfiguration makes the slide transition act as a semantic container.
// This ensures all page content is grouped under one node for accessibility navigation.
func (r *renderSlideTransition) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	config.IsSemanticBoundary = true
	return true
}

func (r *renderSlideTransition) Layout(constraints layout.Constraints) {
	if r.child != nil {
		r.child.Layout(constraints)
		r.SetSize(r.child.Size())
		r.child.SetParentData(&layout.BoxParentData{})
	} else {
		r.SetSize(constraints.Constrain(rendering.Size{}))
	}
}

func (r *renderSlideTransition) slideOffset() rendering.Offset {
	offset := rendering.Offset{}
	if r.animation != nil {
		// Calculate offset based on animation value and direction
		// value 0 = off screen, value 1 = on screen
		t := 1.0 - r.animation.Value // Invert so 0 = visible, 1 = off screen
		size := r.Size()

		switch r.direction {
		case SlideFromRight:
			offset.X = size.Width * t
		case SlideFromLeft:
			offset.X = -size.Width * t
		case SlideFromBottom:
			offset.Y = size.Height * t
		case SlideFromTop:
			offset.Y = -size.Height * t
		}
	}
	return offset
}

func (r *renderSlideTransition) ScrollOffset() rendering.Offset {
	return r.slideOffset()
}

func (r *renderSlideTransition) Paint(ctx *layout.PaintContext) {
	if r.child == nil {
		return
	}

	offset := r.slideOffset()
	ctx.PaintChild(r.child, offset)
}

func (r *renderSlideTransition) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if r.child == nil {
		return false
	}
	return r.child.HitTest(position, result)
}

// FadeTransition animates the opacity of its child.
type FadeTransition struct {
	Animation   *animation.AnimationController
	ChildWidget core.Widget
}

// CreateElement returns a RenderObjectElement for this FadeTransition.
func (f FadeTransition) CreateElement() core.Element {
	return core.NewRenderObjectElement(f, nil)
}

// Key returns nil (no key).
func (f FadeTransition) Key() any {
	return nil
}

// Child returns the child widget.
func (f FadeTransition) Child() core.Widget {
	return f.ChildWidget
}

// CreateRenderObject creates the RenderFadeTransition.
func (f FadeTransition) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	fade := &renderFadeTransition{
		animation: f.Animation,
	}
	fade.SetSelf(fade)
	if f.Animation != nil {
		f.Animation.AddListener(func() {
			fade.MarkNeedsPaint()
		})
	}
	return fade
}

// UpdateRenderObject updates the RenderFadeTransition.
func (f FadeTransition) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if fade, ok := renderObject.(*renderFadeTransition); ok {
		fade.animation = f.Animation
		fade.MarkNeedsPaint()
	}
}

type renderFadeTransition struct {
	layout.RenderBoxBase
	child     layout.RenderBox
	animation *animation.AnimationController
}

func (r *renderFadeTransition) SetChild(child layout.RenderObject) {
	if child == nil {
		r.child = nil
		return
	}
	if box, ok := child.(layout.RenderBox); ok {
		r.child = box
	}
}

func (r *renderFadeTransition) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

// DescribeSemanticsConfiguration makes the fade transition act as a semantic container.
// This ensures all page content is grouped under one node for accessibility navigation.
func (r *renderFadeTransition) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	config.IsSemanticBoundary = true
	return true
}

func (r *renderFadeTransition) Layout(constraints layout.Constraints) {
	if r.child != nil {
		r.child.Layout(constraints)
		r.SetSize(r.child.Size())
		r.child.SetParentData(&layout.BoxParentData{})
	} else {
		r.SetSize(constraints.Constrain(rendering.Size{}))
	}
}

func (r *renderFadeTransition) Paint(ctx *layout.PaintContext) {
	if r.child == nil {
		return
	}
	// Note: Full opacity support would require layer compositing.
	// For now, just paint the child directly.
	// In a full implementation, we'd use an OpacityLayer.
	ctx.PaintChild(r.child, rendering.Offset{})
}

func (r *renderFadeTransition) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if r.child == nil {
		return false
	}
	return r.child.HitTest(position, result)
}
