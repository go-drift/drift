package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/semantics"
)

// ActivityIndicatorSize represents the size of the activity indicator.
type ActivityIndicatorSize = platform.ActivityIndicatorSize

const (
	// ActivityIndicatorSizeSmall is a small spinner.
	ActivityIndicatorSizeSmall = platform.ActivityIndicatorSizeSmall
	// ActivityIndicatorSizeMedium is a medium spinner (default).
	ActivityIndicatorSizeMedium = platform.ActivityIndicatorSizeMedium
	// ActivityIndicatorSizeLarge is a large spinner.
	ActivityIndicatorSizeLarge = platform.ActivityIndicatorSizeLarge
)

// ActivityIndicator displays a native platform spinner.
// Uses UIActivityIndicatorView on iOS and ProgressBar on Android.
type ActivityIndicator struct {
	// Animating controls whether the indicator is spinning.
	// Defaults to true.
	Animating bool

	// Size is the indicator size (Small, Medium, Large).
	// Defaults to Medium.
	Size ActivityIndicatorSize

	// Color is the spinner color (optional, uses system default if not set).
	Color graphics.Color
}

func (a ActivityIndicator) CreateElement() core.Element {
	return core.NewStatefulElement(a, nil)
}

func (a ActivityIndicator) Key() any {
	return nil
}

func (a ActivityIndicator) CreateState() core.State {
	return &activityIndicatorState{}
}

type activityIndicatorState struct {
	element      *core.StatefulElement
	platformView *platform.ActivityIndicatorView
}

func (s *activityIndicatorState) SetElement(e *core.StatefulElement) {
	s.element = e
}

func (s *activityIndicatorState) InitState() {
	// Platform view will be created on first layout
}

func (s *activityIndicatorState) Dispose() {
	if s.platformView != nil {
		platform.GetPlatformViewRegistry().Dispose(s.platformView.ViewID())
		s.platformView = nil
	}
}

func (s *activityIndicatorState) DidChangeDependencies() {}

func (s *activityIndicatorState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	if s.platformView == nil {
		return
	}

	w := s.element.Widget().(ActivityIndicator)
	old := oldWidget.(ActivityIndicator)

	// Update config if anything changed
	if w.Animating != old.Animating || w.Size != old.Size || w.Color != old.Color {
		s.platformView.UpdateConfig(platform.ActivityIndicatorViewConfig{
			Animating: w.Animating,
			Size:      w.Size,
			Color:     uint32(w.Color),
		})
	}
}

func (s *activityIndicatorState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *activityIndicatorState) Build(ctx core.BuildContext) core.Widget {
	w := s.element.Widget().(ActivityIndicator)

	return activityIndicatorRender{
		state:     s,
		animating: w.Animating,
		size:      w.Size,
		color:     w.Color,
	}
}

func (s *activityIndicatorState) ensurePlatformView() {
	if s.platformView != nil {
		return
	}

	w := s.element.Widget().(ActivityIndicator)

	params := map[string]any{
		"animating": w.Animating,
		"size":      int(w.Size),
	}

	if w.Color != 0 {
		params["color"] = uint32(w.Color)
	}

	view, err := platform.GetPlatformViewRegistry().Create("activity_indicator", params)
	if err != nil {
		return
	}

	indicatorView, ok := view.(*platform.ActivityIndicatorView)
	if !ok {
		return
	}

	s.platformView = indicatorView
}

type activityIndicatorRender struct {
	state     *activityIndicatorState
	animating bool
	size      ActivityIndicatorSize
	color     graphics.Color
}

func (a activityIndicatorRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(a, nil)
}

func (a activityIndicatorRender) Key() any {
	return nil
}

func (a activityIndicatorRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderActivityIndicator{
		state:     a.state,
		animating: a.animating,
		size:      a.size,
		color:     a.color,
	}
	r.SetSelf(r)
	return r
}

func (a activityIndicatorRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderActivityIndicator); ok {
		r.state = a.state
		r.animating = a.animating
		r.size = a.size
		r.color = a.color
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

type renderActivityIndicator struct {
	layout.RenderBoxBase
	state     *activityIndicatorState
	animating bool
	size      ActivityIndicatorSize
	color     graphics.Color
}

func (r *renderActivityIndicator) PerformLayout() {
	constraints := r.Constraints()

	// Size based on indicator size
	var width, height float64
	switch r.size {
	case ActivityIndicatorSizeSmall:
		width, height = 20, 20
	case ActivityIndicatorSizeLarge:
		width, height = 40, 40
	default: // Medium
		width, height = 30, 30
	}

	width = min(max(width, constraints.MinWidth), constraints.MaxWidth)
	height = min(max(height, constraints.MinHeight), constraints.MaxHeight)
	r.SetSize(graphics.Size{Width: width, Height: height})
}

func (r *renderActivityIndicator) updatePlatformView(clipBounds *graphics.Rect) {
	if r.state == nil || r.state.element == nil {
		return
	}

	// Ensure view exists
	r.state.ensurePlatformView()

	if r.state.platformView == nil {
		return
	}

	// Get global position
	globalOffset := core.GlobalOffsetOf(r.state.element)
	size := r.Size()

	// Update native view geometry with clip bounds
	// Note: SetGeometry/applyClipBounds controls visibility based on clip state
	r.state.platformView.SetGeometry(globalOffset, size, clipBounds)
}

func (r *renderActivityIndicator) Paint(ctx *layout.PaintContext) {
	// Get clip bounds for platform view
	clip, hasClip := ctx.CurrentClipBounds()
	var clipPtr *graphics.Rect
	if hasClip {
		clipPtr = &clip
	}

	// Update platform view position each frame to animate with page transitions
	r.updatePlatformView(clipPtr)

	// Native view handles rendering - nothing to draw in Skia
}

func (r *renderActivityIndicator) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

func (r *renderActivityIndicator) HandlePointer(event gestures.PointerEvent) {
	// Activity indicator doesn't handle touch events
}

// DescribeSemanticsConfiguration implements SemanticsDescriber for accessibility.
func (r *renderActivityIndicator) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	config.IsSemanticBoundary = true
	config.Properties.Role = semantics.SemanticsRoleProgressIndicator

	if r.animating {
		config.Properties.Label = "Loading"
		config.Properties.Value = "In progress"
	} else {
		config.Properties.Value = "Stopped"
	}

	return true
}
