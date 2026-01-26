package widgets

import (
	"math"
	"time"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/semantics"
	"github.com/go-drift/drift/pkg/theme"
)

// LinearProgressIndicator displays a linear progress indicator.
// When Value is nil, it shows an indeterminate animation.
// When Value is set, it shows determinate progress from 0.0 to 1.0.
type LinearProgressIndicator struct {
	// Value is the progress value (0.0 to 1.0). Nil means indeterminate.
	Value *float64

	// Color is the indicator color. Uses theme primary color if not set.
	Color rendering.Color

	// TrackColor is the background track color. Uses theme surface variant if not set.
	TrackColor rendering.Color

	// Height is the thickness of the indicator. Default: 4.0
	Height float64

	// BorderRadius is the corner radius. Default: 2.0
	BorderRadius float64

	// MinWidth is the minimum width for the indicator. Default: 0 (use constraints).
	MinWidth float64
}

func (l LinearProgressIndicator) CreateElement() core.Element {
	return core.NewStatefulElement(l, nil)
}

func (l LinearProgressIndicator) Key() any {
	return nil
}

func (l LinearProgressIndicator) CreateState() core.State {
	return &linearProgressState{}
}

type linearProgressState struct {
	core.StateBase
	controller *animation.AnimationController
}

func (s *linearProgressState) InitState() {
	w := s.Element().Widget().(LinearProgressIndicator)

	// Only create animation controller for indeterminate mode
	if w.Value == nil {
		s.controller = core.UseController(&s.StateBase, func() *animation.AnimationController {
			c := animation.NewAnimationController(1500 * time.Millisecond)
			c.Curve = animation.LinearCurve
			return c
		})
		core.UseListenable(&s.StateBase, s.controller)

		// Add status listener to repeat the animation
		s.controller.AddStatusListener(func(status animation.AnimationStatus) {
			if status == animation.AnimationCompleted {
				s.controller.Reset()
				s.controller.Forward()
			}
		})

		// Start the animation
		s.controller.Forward()
	}
}

func (s *linearProgressState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	old := oldWidget.(LinearProgressIndicator)
	w := s.Element().Widget().(LinearProgressIndicator)

	// Handle transition between determinate and indeterminate
	wasIndeterminate := old.Value == nil
	isIndeterminate := w.Value == nil

	if wasIndeterminate && !isIndeterminate {
		// Transitioning to determinate - stop animation
		if s.controller != nil {
			s.controller.Stop()
		}
	} else if !wasIndeterminate && isIndeterminate {
		// Transitioning to indeterminate - start animation
		if s.controller == nil {
			s.controller = core.UseController(&s.StateBase, func() *animation.AnimationController {
				c := animation.NewAnimationController(1500 * time.Millisecond)
				c.Curve = animation.LinearCurve
				return c
			})
			core.UseListenable(&s.StateBase, s.controller)
			s.controller.AddStatusListener(func(status animation.AnimationStatus) {
				if status == animation.AnimationCompleted {
					s.controller.Reset()
					s.controller.Forward()
				}
			})
		}
		s.controller.Reset()
		s.controller.Forward()
	}
}

func (s *linearProgressState) Build(ctx core.BuildContext) core.Widget {
	w := s.Element().Widget().(LinearProgressIndicator)
	themeData, _, _ := theme.UseTheme(ctx)

	// Apply defaults
	height := w.Height
	if height == 0 {
		height = 4
	}

	borderRadius := w.BorderRadius
	if borderRadius == 0 {
		borderRadius = 2
	}

	color := w.Color
	if color == 0 {
		color = themeData.ColorScheme.Primary
	}

	trackColor := w.TrackColor
	if trackColor == 0 {
		trackColor = themeData.ColorScheme.SurfaceVariant
	}

	// Calculate animation value for indeterminate mode
	var animValue float64
	if w.Value == nil && s.controller != nil {
		animValue = s.controller.Value
	}

	return linearProgressRender{
		value:        w.Value,
		color:        color,
		trackColor:   trackColor,
		height:       height,
		borderRadius: borderRadius,
		minWidth:     w.MinWidth,
		animValue:    animValue,
	}
}

type linearProgressRender struct {
	value        *float64
	color        rendering.Color
	trackColor   rendering.Color
	height       float64
	borderRadius float64
	minWidth     float64
	animValue    float64
}

func (l linearProgressRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(l, nil)
}

func (l linearProgressRender) Key() any {
	return nil
}

func (l linearProgressRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderLinearProgress{
		value:        l.value,
		color:        l.color,
		trackColor:   l.trackColor,
		height:       l.height,
		borderRadius: l.borderRadius,
		minWidth:     l.minWidth,
		animValue:    l.animValue,
	}
	r.SetSelf(r)
	return r
}

func (l linearProgressRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderLinearProgress); ok {
		r.value = l.value
		r.color = l.color
		r.trackColor = l.trackColor
		r.height = l.height
		r.borderRadius = l.borderRadius
		r.minWidth = l.minWidth
		r.animValue = l.animValue
		r.MarkNeedsPaint()
	}
}

type renderLinearProgress struct {
	layout.RenderBoxBase
	value        *float64
	color        rendering.Color
	trackColor   rendering.Color
	height       float64
	borderRadius float64
	minWidth     float64
	animValue    float64
}

func (r *renderLinearProgress) PerformLayout() {
	constraints := r.Constraints()

	// Width: prefer max width, but use default if unbounded
	width := constraints.MaxWidth
	if width == math.MaxFloat64 {
		width = 200 // Default width if unconstrained
	}
	if r.minWidth > 0 && width < r.minWidth {
		width = r.minWidth
	}
	width = min(max(width, constraints.MinWidth), constraints.MaxWidth)

	// Height: use specified height
	height := r.height
	height = min(max(height, constraints.MinHeight), constraints.MaxHeight)

	r.SetSize(rendering.Size{Width: width, Height: height})
}

func (r *renderLinearProgress) Paint(ctx *layout.PaintContext) {
	size := r.Size()
	radius := rendering.CircularRadius(r.borderRadius)

	// Draw track (background)
	if r.trackColor != 0 {
		trackPaint := rendering.DefaultPaint()
		trackPaint.Color = r.trackColor

		trackRect := rendering.RectFromLTWH(0, 0, size.Width, size.Height)
		trackRRect := rendering.RRectFromRectAndRadius(trackRect, radius)
		ctx.Canvas.DrawRRect(trackRRect, trackPaint)
	}

	// Draw progress
	progressPaint := rendering.DefaultPaint()
	progressPaint.Color = r.color

	if r.value != nil {
		// Determinate mode: draw progress bar from left
		progress := *r.value
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}

		if progress > 0 {
			progressWidth := size.Width * progress
			progressRect := rendering.RectFromLTWH(0, 0, progressWidth, size.Height)
			progressRRect := rendering.RRectFromRectAndRadius(progressRect, radius)
			ctx.Canvas.DrawRRect(progressRRect, progressPaint)
		}
	} else {
		// Indeterminate mode: draw animated bar that moves across
		// Create a "sliding" effect where the bar moves from left to right

		// Calculate bar position and width based on animation progress
		t := r.animValue

		// Bar width varies: starts small, grows, then shrinks
		// Using a sin-based width variation
		minBarWidth := size.Width * 0.1
		maxBarWidth := size.Width * 0.4

		// Phase determines the "pulse" of the width
		widthPhase := t * 2
		if widthPhase > 1 {
			widthPhase = 2 - widthPhase
		}
		barWidth := minBarWidth + (maxBarWidth-minBarWidth)*widthPhase

		// Position: moves from off-screen left to off-screen right
		// Start position: -barWidth (fully off-screen left)
		// End position: size.Width (fully off-screen right)
		totalTravel := size.Width + barWidth
		barX := -barWidth + totalTravel*t

		// Clamp to visible area
		visibleLeft := max(barX, 0)
		visibleRight := min(barX+barWidth, size.Width)

		if visibleRight > visibleLeft {
			progressRect := rendering.RectFromLTWH(visibleLeft, 0, visibleRight-visibleLeft, size.Height)
			progressRRect := rendering.RRectFromRectAndRadius(progressRect, radius)
			ctx.Canvas.DrawRRect(progressRRect, progressPaint)
		}
	}
}

func (r *renderLinearProgress) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	// Progress indicators typically don't handle hit tests
	return false
}

// DescribeSemanticsConfiguration implements SemanticsDescriber for accessibility.
func (r *renderLinearProgress) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	config.IsSemanticBoundary = true
	config.Properties.Role = semantics.SemanticsRoleProgressIndicator

	if r.value != nil {
		progress := int(*r.value * 100)
		config.Properties.Value = formatPercent(progress)
		config.Properties.Label = "Progress"
	} else {
		config.Properties.Label = "Loading"
		config.Properties.Value = "In progress"
	}

	return true
}
