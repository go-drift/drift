package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
)

// Switch toggles between on/off states.
type Switch struct {
	// Value indicates the current on/off state.
	Value bool
	// OnChanged is called when the switch toggles.
	OnChanged func(bool)
	// Disabled disables interaction when true.
	Disabled bool
	// Width controls the overall width.
	Width float64
	// Height controls the overall height.
	Height float64
	// ActiveColor is the track color when on.
	ActiveColor rendering.Color
	// InactiveColor is the track color when off.
	InactiveColor rendering.Color
	// ThumbColor is the thumb fill color.
	ThumbColor rendering.Color
}

func (s Switch) CreateElement() core.Element {
	return core.NewStatelessElement(s, nil)
}

func (s Switch) Key() any {
	return nil
}

func (s Switch) Build(ctx core.BuildContext) core.Widget {
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

	return switchRender{
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

type switchRender struct {
	value         bool
	onChanged     func(bool)
	enabled       bool
	width         float64
	height        float64
	activeColor   rendering.Color
	inactiveColor rendering.Color
	thumbColor    rendering.Color
}

func (s switchRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

func (s switchRender) Key() any {
	return nil
}

func (s switchRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderSwitch{}
	r.SetSelf(r)
	r.update(s)
	return r
}

func (s switchRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderSwitch); ok {
		r.update(s)
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

type renderSwitch struct {
	layout.RenderBoxBase
	value         bool
	onChanged     func(bool)
	enabled       bool
	width         float64
	height        float64
	activeColor   rendering.Color
	inactiveColor rendering.Color
	thumbColor    rendering.Color
	tap           *gestures.TapGestureRecognizer
}

func (r *renderSwitch) update(s switchRender) {
	r.value = s.value
	r.onChanged = s.onChanged
	r.enabled = s.enabled
	r.width = s.width
	r.height = s.height
	r.activeColor = s.activeColor
	r.inactiveColor = s.inactiveColor
	r.thumbColor = s.thumbColor
}

func (r *renderSwitch) Layout(constraints layout.Constraints) {
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
	r.SetSize(rendering.Size{Width: width, Height: height})
}

func (r *renderSwitch) Paint(ctx *layout.PaintContext) {
	size := r.Size()
	trackPaint := rendering.DefaultPaint()
	if r.value {
		trackPaint.Color = r.activeColor
	} else {
		trackPaint.Color = r.inactiveColor
	}
	trackRect := rendering.RectFromLTWH(0, 0, size.Width, size.Height)
	trackRadius := size.Height / 2
	trackRRect := rendering.RRectFromRectAndRadius(trackRect, rendering.CircularRadius(trackRadius))
	ctx.Canvas.DrawRRect(trackRRect, trackPaint)

	thumbRadius := (size.Height - 4) / 2
	thumbCenter := rendering.Offset{X: 2 + thumbRadius, Y: size.Height / 2}
	if r.value {
		thumbCenter.X = size.Width - 2 - thumbRadius
	}
	thumbPaint := rendering.DefaultPaint()
	thumbPaint.Color = r.thumbColor
	ctx.Canvas.DrawCircle(thumbCenter, thumbRadius, thumbPaint)
}

func (r *renderSwitch) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

func (r *renderSwitch) HandlePointer(event gestures.PointerEvent) {
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
