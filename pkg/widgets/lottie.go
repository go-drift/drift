package widgets

import (
	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/lottie"
)

// LottieRepeat controls how a Lottie animation repeats after completing.
type LottieRepeat int

const (
	// LottiePlayOnce plays the animation once and stops at the last frame.
	LottiePlayOnce LottieRepeat = iota
	// LottieLoop replays the animation from the beginning continuously.
	LottieLoop
	// LottieBounce plays forward then backward continuously (ping-pong).
	LottieBounce
)

// Lottie renders a Lottie animation from a loaded [lottie.Animation].
//
// # Loading
//
// The Source field requires a pre-loaded animation. Use [lottie.Load],
// [lottie.LoadBytes], or [lottie.LoadFile] to obtain one.
//
// # Auto-play Behavior
//
// By default, the animation plays automatically on mount. The Repeat field
// controls what happens when playback completes: stop, loop, or bounce.
//
// # Programmatic Control
//
// Pass a Controller to take full control of playback. When a Controller is
// provided, the widget does not auto-play and ignores Repeat and OnComplete.
// The controller's Value (0.0 to 1.0) maps directly to animation progress.
// Switching between external and self-managed control at runtime is supported.
//
// # Sizing Behavior
//
// When Width and/or Height are specified, the animation scales to fit.
// When both are zero, the widget uses the animation's intrinsic dimensions.
// When one dimension is specified, the other is calculated from the aspect ratio.
//
// # Easing
//
// Lottie animations contain their own easing curves baked into keyframes,
// so the internal controller uses linear interpolation by default.
//
// # Creation Patterns
//
//	// Play once
//	widgets.Lottie{Source: anim, Width: 200, Height: 200}
//
//	// Loop
//	widgets.Lottie{Source: anim, Width: 200, Height: 200, Repeat: widgets.LottieLoop}
//
//	// Bounce (forward then reverse)
//	widgets.Lottie{Source: anim, Width: 200, Height: 200, Repeat: widgets.LottieBounce}
//
//	// Completion callback
//	widgets.Lottie{Source: anim, Width: 200, Height: 200, OnComplete: func() { /* done */ }}
//
//	// Intrinsic size
//	widgets.Lottie{Source: anim, Repeat: widgets.LottieLoop}
//
//	// Full programmatic control
//	widgets.Lottie{Source: anim, Controller: ctrl, Width: 200, Height: 200}
//
// # Lifetime
//
// The Source must remain valid for as long as any widget or display list
// references it. Do not call [lottie.Animation.Destroy] while widgets may
// still render the animation.
type Lottie struct {
	core.StatefulBase

	// Source is the pre-loaded Lottie animation to render. Use [lottie.Load],
	// [lottie.LoadBytes], or [lottie.LoadFile] to create one. If nil, the
	// widget renders nothing.
	Source *lottie.Animation

	// Width is the desired width. If zero and Height is set, calculated from aspect ratio.
	// If both zero, uses the animation's intrinsic width.
	Width float64

	// Height is the desired height. If zero and Width is set, calculated from aspect ratio.
	// If both zero, uses the animation's intrinsic height.
	Height float64

	// Repeat controls how the animation repeats. Ignored when Controller is set.
	Repeat LottieRepeat

	// Controller allows external control of the animation. When set, the widget
	// does not auto-play and ignores Repeat and OnComplete. The controller's
	// Value (0.0 to 1.0) maps directly to animation progress.
	Controller *animation.AnimationController

	// OnComplete is called when the animation finishes playing.
	// Only called in LottiePlayOnce mode without an external Controller.
	OnComplete func()
}

func (l Lottie) CreateState() core.State {
	return &lottieState{}
}

type lottieState struct {
	core.StateBase
	ownController   *animation.AnimationController
	unsubListenable func()
}

func (s *lottieState) subscribeListenable(c *animation.AnimationController) {
	if s.unsubListenable != nil {
		s.unsubListenable()
	}
	s.unsubListenable = c.AddListener(func() {
		s.SetState(nil)
	})
}

func (s *lottieState) controller() *animation.AnimationController {
	w := s.Element().Widget().(Lottie)
	if w.Controller != nil {
		return w.Controller
	}
	return s.ownController
}

func (s *lottieState) InitState() {
	w := s.Element().Widget().(Lottie)

	// Create the internal controller when the source has a valid duration.
	// This is created even when an external Controller is provided so that
	// switching from external to self-managed control is seamless.
	if w.Source != nil {
		dur := w.Source.Duration()
		if dur > 0 {
			s.ownController = core.UseController(s, func() *animation.AnimationController {
				c := animation.NewAnimationController(dur)
				c.Curve = animation.LinearCurve
				return c
			})
			s.ownController.AddStatusListener(func(status animation.AnimationStatus) {
				s.onStatus(status)
			})
		}
	}

	// Clean up the active listener on disposal.
	s.OnDispose(func() {
		if s.unsubListenable != nil {
			s.unsubListenable()
		}
	})

	// Listen to whichever controller is active.
	if c := s.controller(); c != nil {
		s.subscribeListenable(c)
	}

	// Auto-play when self-managed.
	if w.Controller == nil && s.ownController != nil {
		s.ownController.Forward()
	}
}

func (s *lottieState) onStatus(status animation.AnimationStatus) {
	w := s.Element().Widget().(Lottie)
	if w.Controller != nil {
		return
	}

	switch status {
	case animation.AnimationCompleted:
		switch w.Repeat {
		case LottieLoop:
			s.ownController.Reset()
			s.ownController.Forward()
		case LottieBounce:
			s.ownController.Reverse()
		default:
			if w.OnComplete != nil {
				w.OnComplete()
			}
		}
	case animation.AnimationDismissed:
		if w.Repeat == LottieBounce {
			s.ownController.Forward()
		}
	}
}

func (s *lottieState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	old := oldWidget.(Lottie)
	w := s.Element().Widget().(Lottie)

	if old.Controller != w.Controller {
		// Re-subscribe to the newly active controller.
		if c := s.controller(); c != nil {
			s.subscribeListenable(c)
		}

		if old.Controller == nil && w.Controller != nil {
			// Switched to external control: stop own playback.
			if s.ownController != nil {
				s.ownController.Stop()
			}
		} else if old.Controller != nil && w.Controller == nil {
			// Switched to self-managed: restart playback.
			if s.ownController != nil {
				s.ownController.Reset()
				s.ownController.Forward()
			}
		}
	}

	// Repeat mode changed to a looping mode while animation is completed:
	// restart playback so the new mode takes effect.
	if old.Repeat != w.Repeat && w.Controller == nil && s.ownController != nil {
		if (w.Repeat == LottieLoop || w.Repeat == LottieBounce) && s.ownController.IsCompleted() {
			s.ownController.Reset()
			s.ownController.Forward()
		}
	}

	// Source changed while self-managed: update duration and restart,
	// or stop the controller if the source was removed.
	if old.Source != w.Source && w.Controller == nil && s.ownController != nil {
		if w.Source != nil {
			dur := w.Source.Duration()
			if dur > 0 {
				s.ownController.Duration = dur
				s.ownController.Reset()
				s.ownController.Forward()
			}
		} else {
			s.ownController.Stop()
		}
	}
}

func (s *lottieState) Build(ctx core.BuildContext) core.Widget {
	w := s.Element().Widget().(Lottie)

	var t float64
	if c := s.controller(); c != nil {
		t = c.Value
	}

	return lottieRender{
		source: w.Source,
		width:  w.Width,
		height: w.Height,
		t:      t,
	}
}

// lottieRender is the inner RenderObjectWidget for the Lottie animation.
type lottieRender struct {
	source *lottie.Animation
	width  float64
	height float64
	t      float64
}

func (l lottieRender) CreateElement() core.Element {
	return core.NewRenderObjectElement()
}

func (l lottieRender) Key() any {
	return nil
}

func (l lottieRender) Child() core.Widget {
	return nil
}

func (l lottieRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderLottie{
		source: l.source,
		width:  l.width,
		height: l.height,
		t:      l.t,
	}
	r.SetSelf(r)
	return r
}

func (l lottieRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderLottie); ok {
		layoutChanged := r.source != l.source || r.width != l.width || r.height != l.height
		paintChanged := layoutChanged || r.t != l.t

		r.source = l.source
		r.width = l.width
		r.height = l.height
		r.t = l.t

		if layoutChanged {
			r.MarkNeedsLayout()
		}
		if paintChanged {
			r.MarkNeedsPaint()
		}
	}
}

type renderLottie struct {
	layout.RenderBoxBase
	source *lottie.Animation
	width  float64
	height float64
	t      float64
}

func (r *renderLottie) IsRepaintBoundary() bool {
	return true
}

func (r *renderLottie) SetChild(child layout.RenderObject) {}

func (r *renderLottie) PerformLayout() {
	constraints := r.Constraints()
	var size graphics.Size

	if r.source != nil {
		intrinsic := r.source.Size()
		aspectRatio := 1.0
		if intrinsic.Height > 0 {
			aspectRatio = intrinsic.Width / intrinsic.Height
		}

		switch {
		case r.width > 0 && r.height > 0:
			size = graphics.Size{Width: r.width, Height: r.height}
		case r.width > 0:
			size = graphics.Size{Width: r.width, Height: r.width / aspectRatio}
		case r.height > 0:
			size = graphics.Size{Width: r.height * aspectRatio, Height: r.height}
		default:
			size = graphics.Size{Width: intrinsic.Width, Height: intrinsic.Height}
		}
	}

	r.SetSize(constraints.Constrain(size))
}

func (r *renderLottie) Paint(ctx *layout.PaintContext) {
	if r.source == nil {
		return
	}

	bounds := graphics.RectFromLTWH(0, 0, r.Size().Width, r.Size().Height)
	r.source.Draw(ctx.Canvas, bounds, r.t)
}

func (r *renderLottie) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}
