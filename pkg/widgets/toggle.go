package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/semantics"
)

// Toggle is a Skia-rendered toggle switch for on/off states.
//
// # Styling Model
//
// Toggle is explicit by default — all visual properties use their struct field
// values directly. A zero value means zero, not "use theme default." For example:
//
//   - ActiveColor: 0 means transparent track when on
//   - Width: 0 means zero width (not rendered)
//   - Height: 0 means zero height (not rendered)
//
// For theme-styled toggles, use [theme.ToggleOf] which pre-fills visual
// properties from the current theme's [theme.SwitchThemeData].
//
// # Creation Patterns
//
// Struct literal (full control):
//
//	widgets.Toggle{
//	    Value:         isEnabled,
//	    OnChanged:     func(v bool) { s.SetState(func() { s.isEnabled = v }) },
//	    ActiveColor:   graphics.RGB(52, 199, 89),
//	    InactiveColor: graphics.RGB(229, 229, 234),
//	    ThumbColor:    graphics.ColorWhite,
//	    Width:         51,
//	    Height:        31,
//	}
//
// Themed (reads from current theme):
//
//	theme.ToggleOf(ctx, isEnabled, onChanged)
//	// Pre-filled with theme colors and dimensions
//
// Toggle is a controlled component - it displays the Value you provide and
// calls OnChanged when toggled. To change the toggle state, update Value in
// your state in response to OnChanged.
//
// For native platform toggles (UISwitch on iOS, SwitchCompat on Android),
// use [Switch] instead.
type Toggle struct {
	// Value indicates the current on/off state.
	Value bool
	// OnChanged is called when the toggle switches.
	OnChanged func(bool)
	// Disabled disables interaction when true.
	Disabled bool
	// Width controls the overall width. Zero means zero width (not rendered).
	Width float64
	// Height controls the overall height. Zero means zero height (not rendered).
	Height float64
	// ActiveColor is the track color when on. Zero means transparent.
	ActiveColor graphics.Color
	// InactiveColor is the track color when off. Zero means transparent.
	InactiveColor graphics.Color
	// ThumbColor is the thumb fill color. Zero means transparent.
	ThumbColor graphics.Color

	// DisabledActiveColor is the track color when on and disabled.
	// If zero, falls back to 0.5 opacity on the normal colors.
	DisabledActiveColor graphics.Color

	// DisabledInactiveColor is the track color when off and disabled.
	// If zero, falls back to 0.5 opacity on the normal colors.
	DisabledInactiveColor graphics.Color

	// DisabledThumbColor is the thumb color when disabled.
	// If zero, falls back to 0.5 opacity on the normal colors.
	DisabledThumbColor graphics.Color
}

func (s Toggle) CreateElement() core.Element {
	return core.NewStatelessElement(s, nil)
}

func (s Toggle) Key() any {
	return nil
}

func (s Toggle) Build(ctx core.BuildContext) core.Widget {
	// Use field values directly — zero means zero
	activeColor := s.ActiveColor
	inactiveColor := s.InactiveColor
	thumbColor := s.ThumbColor
	width := s.Width
	height := s.Height

	enabled := !s.Disabled && s.OnChanged != nil

	// Apply disabled styling when not enabled (either Disabled=true or OnChanged=nil).
	// This ensures widgets with nil handlers also appear disabled.
	useOpacityFallback := false
	if !enabled {
		if s.DisabledActiveColor != 0 || s.DisabledInactiveColor != 0 || s.DisabledThumbColor != 0 {
			// At least one disabled color set - use explicit disabled styling
			// Apply 50% alpha to any color without an explicit disabled variant
			if s.DisabledActiveColor != 0 {
				activeColor = s.DisabledActiveColor
			} else {
				activeColor = activeColor.WithAlpha(0.5)
			}
			if s.DisabledInactiveColor != 0 {
				inactiveColor = s.DisabledInactiveColor
			} else {
				inactiveColor = inactiveColor.WithAlpha(0.5)
			}
			if s.DisabledThumbColor != 0 {
				thumbColor = s.DisabledThumbColor
			} else {
				thumbColor = thumbColor.WithAlpha(0.5)
			}
		} else {
			// No disabled colors set - use opacity fallback
			useOpacityFallback = true
		}
	}

	var result core.Widget = toggleRender{
		value:         s.Value,
		onChanged:     s.OnChanged,
		enabled:       enabled,
		width:         width,
		height:        height,
		activeColor:   activeColor,
		inactiveColor: inactiveColor,
		thumbColor:    thumbColor,
	}

	// Fall back to opacity if no disabled colors provided
	if useOpacityFallback {
		result = Opacity{Opacity: 0.5, ChildWidget: result}
	}

	return result
}

type toggleRender struct {
	value         bool
	onChanged     func(bool)
	enabled       bool
	width         float64
	height        float64
	activeColor   graphics.Color
	inactiveColor graphics.Color
	thumbColor    graphics.Color
}

func (s toggleRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

func (s toggleRender) Key() any {
	return nil
}

func (s toggleRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderToggle{}
	r.SetSelf(r)
	r.update(s)
	return r
}

func (s toggleRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderToggle); ok {
		r.update(s)
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

type renderToggle struct {
	layout.RenderBoxBase
	value         bool
	onChanged     func(bool)
	enabled       bool
	width         float64
	height        float64
	activeColor   graphics.Color
	inactiveColor graphics.Color
	thumbColor    graphics.Color
	tap           *gestures.TapGestureRecognizer
}

func (r *renderToggle) update(s toggleRender) {
	r.value = s.value
	r.onChanged = s.onChanged
	r.enabled = s.enabled
	r.width = s.width
	r.height = s.height
	r.activeColor = s.activeColor
	r.inactiveColor = s.inactiveColor
	r.thumbColor = s.thumbColor
}

func (r *renderToggle) PerformLayout() {
	constraints := r.Constraints()
	width := r.width
	height := r.height
	width = min(max(width, constraints.MinWidth), constraints.MaxWidth)
	height = min(max(height, constraints.MinHeight), constraints.MaxHeight)
	r.SetSize(graphics.Size{Width: width, Height: height})
}

func (r *renderToggle) Paint(ctx *layout.PaintContext) {
	size := r.Size()
	trackPaint := graphics.DefaultPaint()
	if r.value {
		trackPaint.Color = r.activeColor
	} else {
		trackPaint.Color = r.inactiveColor
	}
	trackRect := graphics.RectFromLTWH(0, 0, size.Width, size.Height)
	trackRadius := size.Height / 2
	trackRRect := graphics.RRectFromRectAndRadius(trackRect, graphics.CircularRadius(trackRadius))
	ctx.Canvas.DrawRRect(trackRRect, trackPaint)

	thumbRadius := (size.Height - 4) / 2
	thumbCenter := graphics.Offset{X: 2 + thumbRadius, Y: size.Height / 2}
	if r.value {
		thumbCenter.X = size.Width - 2 - thumbRadius
	}
	thumbPaint := graphics.DefaultPaint()
	thumbPaint.Color = r.thumbColor
	ctx.Canvas.DrawCircle(thumbCenter, thumbRadius, thumbPaint)
}

func (r *renderToggle) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

func (r *renderToggle) HandlePointer(event gestures.PointerEvent) {
	if !r.enabled {
		return
	}
	if r.tap == nil {
		r.tap = gestures.NewTapGestureRecognizer(gestures.DefaultArena)
		r.tap.OnTap = func() {
			if r.onChanged != nil {
				r.onChanged(!r.value)
			}
		}
	}
	if event.Phase == gestures.PointerPhaseDown {
		r.tap.AddPointer(event)
	} else {
		r.tap.HandleEvent(event)
	}
}

// DescribeSemanticsConfiguration implements SemanticsDescriber for accessibility.
func (r *renderToggle) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	config.IsSemanticBoundary = true
	config.Properties.Role = semantics.SemanticsRoleSwitch

	// Set flags
	flags := semantics.SemanticsHasToggledState | semantics.SemanticsHasEnabledState
	if r.value {
		flags = flags.Set(semantics.SemanticsIsToggled)
	}
	if r.enabled {
		flags = flags.Set(semantics.SemanticsIsEnabled)
	}
	config.Properties.Flags = flags

	// Set value description
	if r.value {
		config.Properties.Value = "On"
	} else {
		config.Properties.Value = "Off"
	}

	// Set hint
	if r.enabled {
		config.Properties.Hint = "Double tap to toggle"
	}

	// Set action
	if r.enabled && r.onChanged != nil {
		config.Actions = semantics.NewSemanticsActions()
		config.Actions.SetHandler(semantics.SemanticsActionTap, func(args any) {
			r.onChanged(!r.value)
		})
	}

	return true
}
