package widgets

import (
	"math"
	"time"

	"github.com/go-drift/drift/pkg/animation"
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/semantics"
)

// CircularProgressIndicator displays a circular progress indicator.
// When Value is nil, it shows an indeterminate animation.
// When Value is set, it shows determinate progress from 0.0 to 1.0.
//
// # Styling Model
//
// CircularProgressIndicator is explicit by default — zero values mean zero
// (no color). For theme-styled indicators, use [theme.CircularProgressIndicatorOf]
// which pre-fills Color from Primary and TrackColor from SurfaceVariant.
//
// # Creation Patterns
//
// Struct literal (full control):
//
//	widgets.CircularProgressIndicator{
//	    Value:       nil, // indeterminate
//	    Color:       colors.Primary,
//	    TrackColor:  colors.SurfaceVariant,
//	    Size:        36,
//	    StrokeWidth: 4,
//	}
//
// Themed (reads from current theme):
//
//	theme.CircularProgressIndicatorOf(ctx, nil)  // indeterminate
//	theme.CircularProgressIndicatorOf(ctx, &progress)  // determinate
type CircularProgressIndicator struct {
	core.StatefulBase

	// Value is the progress value (0.0 to 1.0). Nil means indeterminate.
	Value *float64

	// Color is the indicator color. Zero means transparent (invisible).
	Color graphics.Color

	// TrackColor is the background track color. Zero means no track.
	TrackColor graphics.Color

	// StrokeWidth is the thickness of the indicator. Zero means zero stroke (invisible).
	StrokeWidth float64

	// Size is the diameter of the indicator. Zero means zero size (not rendered).
	Size float64
}

func (c CircularProgressIndicator) CreateState() core.State {
	return &circularProgressState{}
}

type circularProgressState struct {
	core.StateBase
	controller  *animation.AnimationController
	rotationRad float64
	sweepRad    float64
}

func (s *circularProgressState) InitState() {
	s.controller = core.UseController(s, func() *animation.AnimationController {
		c := animation.NewAnimationController(1800 * time.Millisecond)
		c.Curve = animation.LinearCurve
		return c
	})
	core.UseListenable(s, s.controller)
	s.controller.AddStatusListener(func(status animation.AnimationStatus) {
		if status == animation.AnimationCompleted {
			s.controller.Reset()
			s.controller.Forward()
		}
	})

	w := s.Element().Widget().(CircularProgressIndicator)
	if w.Value == nil {
		s.controller.Forward()
	}
}

func (s *circularProgressState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	old := oldWidget.(CircularProgressIndicator)
	w := s.Element().Widget().(CircularProgressIndicator)

	wasIndeterminate := old.Value == nil
	isIndeterminate := w.Value == nil

	if wasIndeterminate && !isIndeterminate {
		s.controller.Stop()
	} else if !wasIndeterminate && isIndeterminate {
		s.controller.Reset()
		s.controller.Forward()
	}
}

func (s *circularProgressState) Build(ctx core.BuildContext) core.Widget {
	w := s.Element().Widget().(CircularProgressIndicator)

	// Use field values directly — zero means zero
	size := w.Size
	strokeWidth := w.StrokeWidth
	color := w.Color
	trackColor := w.TrackColor

	// Calculate animation values for indeterminate mode
	var rotationRad, sweepRad float64
	if w.Value == nil && s.controller != nil {
		t := s.controller.Value

		// Rotation: full circle over the animation duration
		rotationRad = t * 2 * math.Pi * 3 // 3 full rotations

		// Sweep: varies between min and max for the "pulsing" effect
		// Use a sine wave to smoothly vary the sweep
		minSweep := 0.1 * 2 * math.Pi  // ~36 degrees
		maxSweep := 0.75 * 2 * math.Pi // ~270 degrees

		// Create a pulsing effect with the sweep angle
		sweepPhase := math.Sin(t * 2 * math.Pi * 2) // 2 pulses per cycle
		sweepRad = minSweep + (maxSweep-minSweep)*(sweepPhase+1)/2
	}

	s.rotationRad = rotationRad
	s.sweepRad = sweepRad

	return circularProgressRender{
		value:       w.Value,
		color:       color,
		trackColor:  trackColor,
		strokeWidth: strokeWidth,
		size:        size,
		rotationRad: rotationRad,
		sweepRad:    sweepRad,
	}
}

type circularProgressRender struct {
	value       *float64
	color       graphics.Color
	trackColor  graphics.Color
	strokeWidth float64
	size        float64
	rotationRad float64
	sweepRad    float64
}

func (c circularProgressRender) CreateElement() core.Element {
	return core.NewRenderObjectElement()
}

func (c circularProgressRender) Key() any {
	return nil
}

func (c circularProgressRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderCircularProgress{
		value:       c.value,
		color:       c.color,
		trackColor:  c.trackColor,
		strokeWidth: c.strokeWidth,
		size:        c.size,
		rotationRad: c.rotationRad,
		sweepRad:    c.sweepRad,
	}
	r.SetSelf(r)
	return r
}

func (c circularProgressRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderCircularProgress); ok {
		r.value = c.value
		r.color = c.color
		r.trackColor = c.trackColor
		r.strokeWidth = c.strokeWidth
		r.size = c.size
		r.rotationRad = c.rotationRad
		r.sweepRad = c.sweepRad
		r.MarkNeedsPaint()
	}
}

type renderCircularProgress struct {
	layout.RenderBoxBase
	value       *float64
	color       graphics.Color
	trackColor  graphics.Color
	strokeWidth float64
	size        float64
	rotationRad float64
	sweepRad    float64
}

func (r *renderCircularProgress) PerformLayout() {
	constraints := r.Constraints()
	width := min(max(r.size, constraints.MinWidth), constraints.MaxWidth)
	height := min(max(r.size, constraints.MinHeight), constraints.MaxHeight)
	r.SetSize(graphics.Size{Width: width, Height: height})
}

func (r *renderCircularProgress) Paint(ctx *layout.PaintContext) {
	size := r.Size()
	centerX := size.Width / 2
	centerY := size.Height / 2
	radius := (min(size.Width, size.Height) - r.strokeWidth) / 2

	// Draw track (background circle)
	if r.trackColor != 0 {
		trackPaint := graphics.DefaultPaint()
		trackPaint.Style = graphics.PaintStyleStroke
		trackPaint.StrokeWidth = r.strokeWidth
		trackPaint.Color = r.trackColor

		// Draw full circle as track
		ctx.Canvas.DrawCircle(graphics.Offset{X: centerX, Y: centerY}, radius, trackPaint)
	}

	// Draw progress arc
	arcPaint := graphics.DefaultPaint()
	arcPaint.Style = graphics.PaintStyleStroke
	arcPaint.StrokeWidth = r.strokeWidth
	arcPaint.Color = r.color

	if r.value != nil {
		// Determinate mode: draw arc based on value
		progress := *r.value
		if progress < 0 {
			progress = 0
		}
		if progress > 1 {
			progress = 1
		}

		if progress > 0 {
			// Start at top (-90 degrees)
			startAngle := -math.Pi / 2
			sweepAngle := progress * 2 * math.Pi

			r.drawArc(ctx, centerX, centerY, radius, startAngle, sweepAngle, arcPaint)
		}
	} else {
		// Indeterminate mode: draw animated arc
		startAngle := -math.Pi/2 + r.rotationRad
		r.drawArc(ctx, centerX, centerY, radius, startAngle, r.sweepRad, arcPaint)
	}
}

func (r *renderCircularProgress) drawArc(ctx *layout.PaintContext, cx, cy, radius, startAngle, sweepAngle float64, paint graphics.Paint) {
	if sweepAngle == 0 {
		return
	}

	// Draw arc using cubic bezier curves
	// For best accuracy, split into 90-degree (π/2) segments
	path := graphics.NewPath()

	// Calculate start point
	startX := cx + radius*math.Cos(startAngle)
	startY := cy + radius*math.Sin(startAngle)
	path.MoveTo(startX, startY)

	// Split arc into segments of at most 90 degrees
	maxSegmentAngle := math.Pi / 2
	remaining := sweepAngle
	currentAngle := startAngle

	for math.Abs(remaining) > 0.0001 {
		// Determine segment angle
		segmentAngle := remaining
		if math.Abs(segmentAngle) > maxSegmentAngle {
			if segmentAngle > 0 {
				segmentAngle = maxSegmentAngle
			} else {
				segmentAngle = -maxSegmentAngle
			}
		}

		// Calculate control points for cubic bezier approximation of arc
		// Using the formula: k = (4/3) * tan(angle/4)
		k := (4.0 / 3.0) * math.Tan(segmentAngle/4)

		endAngle := currentAngle + segmentAngle

		// Start point tangent direction (perpendicular to radius)
		tx1 := -math.Sin(currentAngle)
		ty1 := math.Cos(currentAngle)

		// End point tangent direction
		tx2 := -math.Sin(endAngle)
		ty2 := math.Cos(endAngle)

		// Calculate end point
		endX := cx + radius*math.Cos(endAngle)
		endY := cy + radius*math.Sin(endAngle)

		// Calculate current point (start of this segment)
		currX := cx + radius*math.Cos(currentAngle)
		currY := cy + radius*math.Sin(currentAngle)

		// Control points
		cp1X := currX + k*radius*tx1
		cp1Y := currY + k*radius*ty1
		cp2X := endX - k*radius*tx2
		cp2Y := endY - k*radius*ty2

		path.CubicTo(cp1X, cp1Y, cp2X, cp2Y, endX, endY)

		currentAngle = endAngle
		remaining -= segmentAngle
	}

	ctx.Canvas.DrawPath(path, paint)
}

func (r *renderCircularProgress) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	// Progress indicators typically don't handle hit tests
	return false
}

// DescribeSemanticsConfiguration implements SemanticsDescriber for accessibility.
func (r *renderCircularProgress) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
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

// formatPercent formats an integer percentage (0-100) as a string with % suffix.
func formatPercent(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 100 {
		n = 100
	}
	// Simple int to string without fmt package
	if n == 0 {
		return "0%"
	}
	if n == 100 {
		return "100%"
	}
	result := make([]byte, 0, 4)
	for n > 0 {
		result = append([]byte{byte('0' + n%10)}, result...)
		n /= 10
	}
	return string(result) + "%"
}
