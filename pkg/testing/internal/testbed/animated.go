package testbed

import (
	"time"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// AnimatedBox animates its width between From and To over Duration.
type AnimatedBox struct {
	core.StatefulBase
	Duration time.Duration
	From     float64
	To       float64
	Height   float64
	Color    graphics.Color
}

func (a AnimatedBox) CreateState() core.State {
	return &animatedBoxState{}
}

type animatedBoxState struct {
	core.StateBase
	controller *animation.AnimationController
	tween      *animation.Tween[float64]
}

func (s *animatedBoxState) InitState() {
	w := s.Element().Widget().(AnimatedBox)
	s.controller = animation.NewAnimationController(w.Duration)
	s.tween = animation.TweenFloat64(w.From, w.To)
	s.controller.AddListener(func() {
		s.SetState(func() {})
	})
	s.controller.Forward()
}

func (s *animatedBoxState) Build(ctx core.BuildContext) core.Widget {
	w := s.Element().Widget().(AnimatedBox)
	currentWidth := s.tween.Transform(s.controller)
	return LayoutBox{
		Width:  currentWidth,
		Height: w.Height,
		Color:  w.Color,
	}
}

func (s *animatedBoxState) Dispose() {
	if s.controller != nil {
		s.controller.Dispose()
	}
	s.StateBase.Dispose()
}

// AnimatedBoxValue returns the current animated width value.
// This is for tests that want to inspect intermediate animation values.
func AnimatedBoxValue(element core.Element) float64 {
	// Walk to find the render object's size
	if ro, ok := element.(interface{ RenderObject() layout.RenderObject }); ok {
		if renderObj := ro.RenderObject(); renderObj != nil {
			return renderObj.Size().Width
		}
	}
	return 0
}
