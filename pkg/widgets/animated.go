package widgets

import (
	"time"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
)

// AnimatedContainer is a Container that animates changes to its properties.
//
// When properties like Color, Width, Height, Padding, or Alignment change,
// the widget automatically animates from the old value to the new value
// over the specified Duration using the specified Curve.
//
// Note: Gradient is not animated; changes to Gradient will apply immediately.
//
// Example:
//
//	widgets.AnimatedContainer{
//	    Duration: 300 * time.Millisecond,
//	    Curve:    animation.EaseInOut,
//	    Color:    s.isActive ? colors.Primary : colors.Surface,
//	    Width:    100,
//	    Height:   100,
//	    ChildWidget: child,
//	}
type AnimatedContainer struct {
	// Duration is the length of the animation.
	Duration time.Duration
	// Curve transforms the animation progress. If nil, uses linear interpolation.
	Curve func(float64) float64
	// OnEnd is called when the animation completes.
	OnEnd func()

	// Container properties that will be animated when they change.
	Padding     layout.EdgeInsets
	Width       float64
	Height      float64
	Color       graphics.Color
	Gradient    *graphics.Gradient
	Alignment   layout.Alignment
	ChildWidget core.Widget
}

func (a AnimatedContainer) CreateElement() core.Element {
	return core.NewStatefulElement(a, nil)
}

func (a AnimatedContainer) Key() any {
	return nil
}

func (a AnimatedContainer) CreateState() core.State {
	return &animatedContainerState{}
}

type animatedContainerState struct {
	core.StateBase
	controller *animation.AnimationController

	// Tweens for each property
	paddingTween   *animation.Tween[layout.EdgeInsets]
	widthTween     *animation.Tween[float64]
	heightTween    *animation.Tween[float64]
	colorTween     *animation.Tween[graphics.Color]
	alignmentTween *animation.Tween[layout.Alignment]

	// Current values (used as starting point for new animations)
	currentPadding   layout.EdgeInsets
	currentWidth     float64
	currentHeight    float64
	currentColor     graphics.Color
	currentAlignment layout.Alignment
}

func (s *animatedContainerState) InitState() {
	w := s.Element().Widget().(AnimatedContainer)
	s.controller = core.UseController(&s.StateBase, func() *animation.AnimationController {
		c := animation.NewAnimationController(w.Duration)
		if w.Curve != nil {
			c.Curve = w.Curve
		}
		return c
	})
	core.UseListenable(&s.StateBase, s.controller)

	// Listen for animation completion
	s.controller.AddStatusListener(func(status animation.AnimationStatus) {
		if status == animation.AnimationCompleted {
			w := s.Element().Widget().(AnimatedContainer)
			if w.OnEnd != nil {
				w.OnEnd()
			}
		}
	})

	// Initialize current values to target values (no initial animation)
	s.currentPadding = w.Padding
	s.currentWidth = w.Width
	s.currentHeight = w.Height
	s.currentColor = w.Color
	s.currentAlignment = w.Alignment
}

func (s *animatedContainerState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	old := oldWidget.(AnimatedContainer)
	w := s.Element().Widget().(AnimatedContainer)

	// Update controller settings
	s.controller.Duration = w.Duration
	if w.Curve != nil {
		s.controller.Curve = w.Curve
	} else {
		s.controller.Curve = animation.LinearCurve
	}

	// Check if any property changed
	changed := old.Padding != w.Padding ||
		old.Width != w.Width ||
		old.Height != w.Height ||
		old.Color != w.Color ||
		old.Alignment != w.Alignment

	if changed {
		// Create tweens for ALL properties from current animated value to target.
		// This ensures smooth transitions even when changing mid-animation.
		// For unchanged properties, this creates a tween that continues toward the same target.
		s.paddingTween = animation.TweenEdgeInsets(s.currentPadding, w.Padding)
		s.widthTween = animation.TweenFloat64(s.currentWidth, w.Width)
		s.heightTween = animation.TweenFloat64(s.currentHeight, w.Height)
		s.colorTween = animation.TweenColor(s.currentColor, w.Color)
		s.alignmentTween = animation.TweenAlignment(s.currentAlignment, w.Alignment)

		s.controller.Reset()
		s.controller.Forward()
	}
}

func (s *animatedContainerState) Build(ctx core.BuildContext) core.Widget {
	w := s.Element().Widget().(AnimatedContainer)
	t := s.controller.Value

	// Calculate current animated values
	if s.paddingTween != nil {
		s.currentPadding = s.paddingTween.Evaluate(t)
	} else {
		s.currentPadding = w.Padding
	}

	if s.widthTween != nil {
		s.currentWidth = s.widthTween.Evaluate(t)
	} else {
		s.currentWidth = w.Width
	}

	if s.heightTween != nil {
		s.currentHeight = s.heightTween.Evaluate(t)
	} else {
		s.currentHeight = w.Height
	}

	if s.colorTween != nil {
		s.currentColor = s.colorTween.Evaluate(t)
	} else {
		s.currentColor = w.Color
	}

	if s.alignmentTween != nil {
		s.currentAlignment = s.alignmentTween.Evaluate(t)
	} else {
		s.currentAlignment = w.Alignment
	}

	return Container{
		Padding:     s.currentPadding,
		Width:       s.currentWidth,
		Height:      s.currentHeight,
		Color:       s.currentColor,
		Gradient:    w.Gradient,
		Alignment:   s.currentAlignment,
		ChildWidget: w.ChildWidget,
	}
}

// AnimatedOpacity animates changes to opacity over a duration.
//
// When the Opacity property changes, the widget automatically animates
// from the old value to the new value over the specified Duration.
//
// Example:
//
//	widgets.AnimatedOpacity{
//	    Duration: 200 * time.Millisecond,
//	    Curve:    animation.EaseOut,
//	    Opacity:  s.isVisible ? 1.0 : 0.0,
//	    ChildWidget: child,
//	}
type AnimatedOpacity struct {
	// Duration is the length of the animation.
	Duration time.Duration
	// Curve transforms the animation progress. If nil, uses linear interpolation.
	Curve func(float64) float64
	// OnEnd is called when the animation completes.
	OnEnd func()

	// Opacity is the target opacity (0.0 to 1.0).
	Opacity float64
	// ChildWidget is the widget to which opacity is applied.
	ChildWidget core.Widget
}

func (a AnimatedOpacity) CreateElement() core.Element {
	return core.NewStatefulElement(a, nil)
}

func (a AnimatedOpacity) Key() any {
	return nil
}

func (a AnimatedOpacity) CreateState() core.State {
	return &animatedOpacityState{}
}

type animatedOpacityState struct {
	core.StateBase
	controller     *animation.AnimationController
	opacityTween   *animation.Tween[float64]
	currentOpacity float64
}

func (s *animatedOpacityState) InitState() {
	w := s.Element().Widget().(AnimatedOpacity)
	s.controller = core.UseController(&s.StateBase, func() *animation.AnimationController {
		c := animation.NewAnimationController(w.Duration)
		if w.Curve != nil {
			c.Curve = w.Curve
		}
		return c
	})
	core.UseListenable(&s.StateBase, s.controller)

	// Listen for animation completion
	s.controller.AddStatusListener(func(status animation.AnimationStatus) {
		if status == animation.AnimationCompleted {
			w := s.Element().Widget().(AnimatedOpacity)
			if w.OnEnd != nil {
				w.OnEnd()
			}
		}
	})

	// Initialize to target value (no initial animation)
	s.currentOpacity = w.Opacity
}

func (s *animatedOpacityState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	old := oldWidget.(AnimatedOpacity)
	w := s.Element().Widget().(AnimatedOpacity)

	// Update controller settings
	s.controller.Duration = w.Duration
	if w.Curve != nil {
		s.controller.Curve = w.Curve
	} else {
		s.controller.Curve = animation.LinearCurve
	}

	if old.Opacity != w.Opacity {
		// Capture current animated value as new start
		s.opacityTween = animation.TweenFloat64(s.currentOpacity, w.Opacity)
		s.controller.Reset()
		s.controller.Forward()
	}
}

func (s *animatedOpacityState) Build(ctx core.BuildContext) core.Widget {
	w := s.Element().Widget().(AnimatedOpacity)
	t := s.controller.Value

	if s.opacityTween != nil {
		s.currentOpacity = s.opacityTween.Evaluate(t)
	} else {
		s.currentOpacity = w.Opacity
	}

	return Opacity{
		Opacity:     s.currentOpacity,
		ChildWidget: w.ChildWidget,
	}
}
