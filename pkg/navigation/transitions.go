package navigation

import (
	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/semantics"
)

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

// backgroundParallaxFactor controls how far the background page shifts left
// during a foreground push/pop transition (fraction of page width).
const backgroundParallaxFactor = 0.33

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
	Animation *animation.AnimationController
	Direction SlideDirection
	Child     core.Widget
}

// CreateElement returns a RenderObjectElement for this SlideTransition.
func (s SlideTransition) CreateElement() core.Element {
	return core.NewRenderObjectElement()
}

// Key returns nil (no key).
func (s SlideTransition) Key() any {
	return nil
}

// ChildWidget returns the child widget.
func (s SlideTransition) ChildWidget() core.Widget {
	return s.Child
}

// CreateRenderObject creates the RenderSlideTransition.
func (s SlideTransition) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	slide := &renderSlideTransition{
		animation: s.Animation,
		direction: s.Direction,
	}
	slide.SetSelf(slide)
	slide.subscribeAnimation()
	return slide
}

// UpdateRenderObject updates the RenderSlideTransition.
func (s SlideTransition) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if slide, ok := renderObject.(*renderSlideTransition); ok {
		if slide.animation != s.Animation {
			slide.unsubscribeAnimation()
			slide.animation = s.Animation
			slide.subscribeAnimation()
		}
		slide.direction = s.Direction
		slide.MarkNeedsPaint()
	}
}

type renderSlideTransition struct {
	layout.RenderBoxBase
	child       layout.RenderBox
	animation   *animation.AnimationController
	direction   SlideDirection
	unsubscribe func()
}

func (r *renderSlideTransition) subscribeAnimation() {
	if r.animation != nil {
		r.unsubscribe = r.animation.AddListener(func() {
			r.MarkNeedsPaint()
		})
	}
}

func (r *renderSlideTransition) unsubscribeAnimation() {
	if r.unsubscribe != nil {
		r.unsubscribe()
		r.unsubscribe = nil
	}
}

func (r *renderSlideTransition) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	if child == nil {
		r.child = nil
		return
	}
	if box, ok := child.(layout.RenderBox); ok {
		r.child = box
		setParentOnChild(r.child, r)
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

func (r *renderSlideTransition) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		r.child.Layout(constraints, true) // true: we read child.Size()
		r.SetSize(r.child.Size())
		r.child.SetParentData(&layout.BoxParentData{})
	} else {
		r.SetSize(constraints.Constrain(graphics.Size{}))
	}
}

func (r *renderSlideTransition) slideOffset() graphics.Offset {
	offset := graphics.Offset{}
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

func (r *renderSlideTransition) ScrollOffset() graphics.Offset {
	return r.slideOffset()
}

func (r *renderSlideTransition) Paint(ctx *layout.PaintContext) {
	if r.child == nil {
		return
	}

	offset := r.slideOffset()
	ctx.PaintChildWithLayer(r.child, offset)
}

func (r *renderSlideTransition) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if r.child == nil {
		return false
	}
	return r.child.HitTest(position, result)
}

// BackgroundSlideTransition slides its child to the left as a foreground page
// enters. At animation value 0 the child is at its normal position; at value 1
// the child is shifted left by 33% of the width.
type BackgroundSlideTransition struct {
	Animation *animation.AnimationController
	Child     core.Widget
}

// CreateElement returns a RenderObjectElement for this BackgroundSlideTransition.
func (b BackgroundSlideTransition) CreateElement() core.Element {
	return core.NewRenderObjectElement()
}

// Key returns nil (no key).
func (b BackgroundSlideTransition) Key() any {
	return nil
}

// ChildWidget returns the child widget.
func (b BackgroundSlideTransition) ChildWidget() core.Widget {
	return b.Child
}

// CreateRenderObject creates the renderBackgroundSlideTransition.
func (b BackgroundSlideTransition) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderBackgroundSlideTransition{
		animation: b.Animation,
	}
	r.SetSelf(r)
	r.subscribeAnimation()
	return r
}

// UpdateRenderObject updates the renderBackgroundSlideTransition.
func (b BackgroundSlideTransition) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderBackgroundSlideTransition); ok {
		if r.animation != b.Animation {
			r.unsubscribeAnimation()
			r.animation = b.Animation
			r.subscribeAnimation()
		}
		r.MarkNeedsPaint()
	}
}

type renderBackgroundSlideTransition struct {
	layout.RenderBoxBase
	child       layout.RenderBox
	animation   *animation.AnimationController
	unsubscribe func()
}

func (r *renderBackgroundSlideTransition) subscribeAnimation() {
	if r.animation != nil {
		r.unsubscribe = r.animation.AddListener(func() {
			r.MarkNeedsPaint()
		})
	}
}

func (r *renderBackgroundSlideTransition) unsubscribeAnimation() {
	if r.unsubscribe != nil {
		r.unsubscribe()
		r.unsubscribe = nil
	}
}

func (r *renderBackgroundSlideTransition) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	if child == nil {
		r.child = nil
		return
	}
	if box, ok := child.(layout.RenderBox); ok {
		r.child = box
		setParentOnChild(r.child, r)
	}
}

func (r *renderBackgroundSlideTransition) VisitChildren(visitor func(layout.RenderObject)) {
	if r.child != nil {
		visitor(r.child)
	}
}

func (r *renderBackgroundSlideTransition) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		r.child.Layout(constraints, true)
		r.SetSize(r.child.Size())
		r.child.SetParentData(&layout.BoxParentData{})
	} else {
		r.SetSize(constraints.Constrain(graphics.Size{}))
	}
}

func (r *renderBackgroundSlideTransition) slideOffset() graphics.Offset {
	if r.animation == nil {
		return graphics.Offset{}
	}
	return graphics.Offset{
		X: -r.Size().Width * backgroundParallaxFactor * r.animation.Value,
	}
}

func (r *renderBackgroundSlideTransition) ScrollOffset() graphics.Offset {
	return r.slideOffset()
}

func (r *renderBackgroundSlideTransition) Paint(ctx *layout.PaintContext) {
	if r.child == nil {
		return
	}
	offset := r.slideOffset()
	ctx.PaintChildWithLayer(r.child, offset)
}

func (r *renderBackgroundSlideTransition) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if r.child == nil {
		return false
	}
	return r.child.HitTest(position, result)
}

// FadeTransition animates the opacity of its child.
type FadeTransition struct {
	Animation *animation.AnimationController
	Child     core.Widget
}

// CreateElement returns a RenderObjectElement for this FadeTransition.
func (f FadeTransition) CreateElement() core.Element {
	return core.NewRenderObjectElement()
}

// Key returns nil (no key).
func (f FadeTransition) Key() any {
	return nil
}

// ChildWidget returns the child widget.
func (f FadeTransition) ChildWidget() core.Widget {
	return f.Child
}

// CreateRenderObject creates the RenderFadeTransition.
func (f FadeTransition) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	fade := &renderFadeTransition{
		animation: f.Animation,
	}
	fade.SetSelf(fade)
	fade.subscribeAnimation()
	return fade
}

// UpdateRenderObject updates the RenderFadeTransition.
func (f FadeTransition) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if fade, ok := renderObject.(*renderFadeTransition); ok {
		if fade.animation != f.Animation {
			fade.unsubscribeAnimation()
			fade.animation = f.Animation
			fade.subscribeAnimation()
		}
		fade.MarkNeedsPaint()
	}
}

type renderFadeTransition struct {
	layout.RenderBoxBase
	child       layout.RenderBox
	animation   *animation.AnimationController
	unsubscribe func()
}

func (r *renderFadeTransition) subscribeAnimation() {
	if r.animation != nil {
		r.unsubscribe = r.animation.AddListener(func() {
			r.MarkNeedsPaint()
		})
	}
}

func (r *renderFadeTransition) unsubscribeAnimation() {
	if r.unsubscribe != nil {
		r.unsubscribe()
		r.unsubscribe = nil
	}
}

func (r *renderFadeTransition) SetChild(child layout.RenderObject) {
	setParentOnChild(r.child, nil)
	if child == nil {
		r.child = nil
		return
	}
	if box, ok := child.(layout.RenderBox); ok {
		r.child = box
		setParentOnChild(r.child, r)
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

func (r *renderFadeTransition) PerformLayout() {
	constraints := r.Constraints()
	if r.child != nil {
		r.child.Layout(constraints, true) // true: we read child.Size()
		r.SetSize(r.child.Size())
		r.child.SetParentData(&layout.BoxParentData{})
	} else {
		r.SetSize(constraints.Constrain(graphics.Size{}))
	}
}

func (r *renderFadeTransition) Paint(ctx *layout.PaintContext) {
	if r.child == nil {
		return
	}
	// Note: Full opacity support would require layer compositing.
	// For now, just paint the child directly.
	// In a full implementation, we'd use an OpacityLayer.
	ctx.PaintChildWithLayer(r.child, graphics.Offset{})
}

func (r *renderFadeTransition) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if r.child == nil {
		return false
	}
	return r.child.HitTest(position, result)
}
