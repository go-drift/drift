package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/semantics"
	"github.com/go-drift/drift/pkg/theme"
)

// Toggle is a Skia-rendered toggle switch for on/off states.
type Toggle struct {
	// Value indicates the current on/off state.
	Value bool
	// OnChanged is called when the toggle switches.
	OnChanged func(bool)
	// Disabled disables interaction when true.
	Disabled bool
	// Width controls the overall width.
	Width float64
	// Height controls the overall height.
	Height float64
	// ActiveColor is the track color when on.
	ActiveColor graphics.Color
	// InactiveColor is the track color when off.
	InactiveColor graphics.Color
	// ThumbColor is the thumb fill color.
	ThumbColor graphics.Color
}

// ToggleOf creates a toggle switch with the given value and change handler.
// This is a convenience helper equivalent to:
//
//	Toggle{Value: value, OnChanged: onChanged}
func ToggleOf(value bool, onChanged func(bool)) Toggle {
	return Toggle{Value: value, OnChanged: onChanged}
}

func (s Toggle) CreateElement() core.Element {
	return core.NewStatelessElement(s, nil)
}

func (s Toggle) Key() any {
	return nil
}

func (s Toggle) Build(ctx core.BuildContext) core.Widget {
	themeData := theme.ThemeOf(ctx)
	switchTheme := themeData.SwitchThemeOf()

	activeColor := s.ActiveColor
	if activeColor == 0 {
		activeColor = switchTheme.ActiveTrackColor
	}
	inactiveColor := s.InactiveColor
	if inactiveColor == 0 {
		inactiveColor = switchTheme.InactiveTrackColor
	}
	thumbColor := s.ThumbColor
	if thumbColor == 0 {
		thumbColor = switchTheme.ThumbColor
	}
	width := s.Width
	if width == 0 {
		width = switchTheme.Width
	}
	height := s.Height
	if height == 0 {
		height = switchTheme.Height
	}

	enabled := !s.Disabled && s.OnChanged != nil
	if !enabled {
		activeColor = switchTheme.DisabledActiveTrackColor
		inactiveColor = switchTheme.DisabledInactiveTrackColor
		thumbColor = switchTheme.DisabledThumbColor
	}

	return toggleRender{
		value:         s.Value,
		onChanged:     s.OnChanged,
		enabled:       enabled,
		width:         width,
		height:        height,
		activeColor:   activeColor,
		inactiveColor: inactiveColor,
		thumbColor:    thumbColor,
	}
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
	if width == 0 {
		width = 44
	}
	if height == 0 {
		height = 26
	}
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
