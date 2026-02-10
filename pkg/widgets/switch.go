package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/semantics"
)

// Switch is a toggle control that uses native platform components
// (UISwitch on iOS, SwitchCompat on Android).
//
// Switch is a controlled component - it displays the Value you provide and
// calls OnChanged when toggled. To change the switch state, update Value in
// your state in response to OnChanged.
//
// # Creation Pattern
//
// Use struct literal (no themed constructor exists for native Switch):
//
//	widgets.Switch{
//	    Value: s.notificationsEnabled,
//	    OnChanged: func(enabled bool) {
//	        s.SetState(func() { s.notificationsEnabled = enabled })
//	    },
//	    OnTintColor: colors.Primary,  // optional
//	}
//
// The native implementation provides platform-appropriate animations and
// haptic feedback automatically.
//
// For a Drift-rendered toggle with full styling control, use [Toggle] instead.
type Switch struct {
	// Value indicates the current on/off state.
	Value bool
	// OnChanged is called when the switch toggles.
	OnChanged func(bool)
	// Disabled disables interaction when true.
	Disabled bool
	// OnTintColor is the track color when on (optional).
	OnTintColor graphics.Color
	// ThumbColor is the thumb color (optional).
	ThumbColor graphics.Color
}

func (s Switch) CreateElement() core.Element {
	return core.NewStatefulElement(s, nil)
}

func (s Switch) Key() any {
	return nil
}

func (s Switch) CreateState() core.State {
	return &switchState{}
}

type switchState struct {
	element      *core.StatefulElement
	platformView *platform.SwitchView
	value        bool
}

func (s *switchState) SetElement(e *core.StatefulElement) {
	s.element = e
}

func (s *switchState) InitState() {
	w := s.element.Widget().(Switch)
	s.value = w.Value
}

func (s *switchState) Dispose() {
	if s.platformView != nil {
		platform.GetPlatformViewRegistry().Dispose(s.platformView.ViewID())
		s.platformView = nil
	}
}

func (s *switchState) DidChangeDependencies() {}

func (s *switchState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	w := s.element.Widget().(Switch)

	// Sync value if it changed from widget
	if w.Value != s.value {
		s.value = w.Value
		if s.platformView != nil {
			s.platformView.SetValue(w.Value)
		}
	}

	// Update config if colors changed
	if s.platformView != nil {
		old := oldWidget.(Switch)
		if w.OnTintColor != old.OnTintColor || w.ThumbColor != old.ThumbColor {
			s.platformView.UpdateConfig(platform.SwitchViewConfig{
				OnTintColor:    uint32(w.OnTintColor),
				ThumbTintColor: uint32(w.ThumbColor),
			})
		}
	}
}

func (s *switchState) SetState(fn func()) {
	fn()
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *switchState) Build(ctx core.BuildContext) core.Widget {
	w := s.element.Widget().(Switch)

	return switchRender{
		state:    s,
		disabled: w.Disabled,
	}
}

// OnValueChanged implements platform.SwitchViewClient.
func (s *switchState) OnValueChanged(value bool) {
	w := s.element.Widget().(Switch)

	s.SetState(func() {
		s.value = value
	})

	if w.OnChanged != nil {
		w.OnChanged(value)
	}
}

func (s *switchState) ensurePlatformView() {
	if s.platformView != nil {
		return
	}

	w := s.element.Widget().(Switch)

	params := map[string]any{
		"value": s.value,
	}

	if w.OnTintColor != 0 {
		params["onTintColor"] = uint32(w.OnTintColor)
	}
	if w.ThumbColor != 0 {
		params["thumbTintColor"] = uint32(w.ThumbColor)
	}

	view, err := platform.GetPlatformViewRegistry().Create("switch", params)
	if err != nil {
		return
	}

	switchView, ok := view.(*platform.SwitchView)
	if !ok {
		return
	}

	s.platformView = switchView
	s.platformView.SetClient(s)
}

type switchRender struct {
	state    *switchState
	disabled bool
}

func (s switchRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(s, nil)
}

func (s switchRender) Key() any {
	return nil
}

func (s switchRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderSwitch{
		state:    s.state,
		disabled: s.disabled,
	}
	r.SetSelf(r)
	return r
}

func (s switchRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderSwitch); ok {
		r.state = s.state
		r.disabled = s.disabled
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

var _ layout.PlatformViewOwner = (*renderSwitch)(nil)

type renderSwitch struct {
	layout.RenderBoxBase
	state    *switchState
	disabled bool
	tap      *gestures.TapGestureRecognizer
}

func (r *renderSwitch) PerformLayout() {
	constraints := r.Constraints()
	// Standard native switch dimensions (iOS: 51x31, Android: ~52x32)
	width := 51.0
	height := 31.0
	width = min(max(width, constraints.MinWidth), constraints.MaxWidth)
	height = min(max(height, constraints.MinHeight), constraints.MaxHeight)
	r.SetSize(graphics.Size{Width: width, Height: height})
}

func (r *renderSwitch) Paint(ctx *layout.PaintContext) {
	r.state.ensurePlatformView() // lazy init — only creates on first paint
	if r.state.platformView != nil {
		ctx.EmbedPlatformView(r.state.platformView.ViewID(), r.Size())
		// Note: SetEnabled is a side effect, but UpdateRenderObject calls
		// MarkNeedsPaint when disabled changes, so this always re-runs.
		r.state.platformView.SetEnabled(!r.disabled)
	}
}

func (r *renderSwitch) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

// PlatformViewID implements PlatformViewOwner.
func (r *renderSwitch) PlatformViewID() int64 {
	if r.state != nil && r.state.platformView != nil {
		if id := r.state.platformView.ViewID(); id != 0 {
			return id
		}
	}
	return -1
}

func (r *renderSwitch) HandlePointer(event gestures.PointerEvent) {
	// Native view handles touch events directly
}

// DescribeSemanticsConfiguration implements SemanticsDescriber for accessibility.
func (r *renderSwitch) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	config.IsSemanticBoundary = true
	config.Properties.Role = semantics.SemanticsRoleSwitch

	// Set flags
	flags := semantics.SemanticsHasToggledState | semantics.SemanticsHasEnabledState
	if r.state != nil && r.state.value {
		flags = flags.Set(semantics.SemanticsIsToggled)
	}
	if !r.disabled {
		flags = flags.Set(semantics.SemanticsIsEnabled)
	}
	config.Properties.Flags = flags

	// Value is needed because HasToggledState isn't mapped to
	// isCheckable on Android, so TalkBack won't auto-announce state.
	if r.state != nil && r.state.value {
		config.Properties.Value = "On"
	} else {
		config.Properties.Value = "Off"
	}

	// No explicit Hint: TalkBack auto-generates "double tap to activate"
	// for clickable items, so a custom hint would duplicate it.

	// Set action
	if !r.disabled && r.state != nil {
		w := r.state.element.Widget().(Switch)
		if w.OnChanged != nil {
			config.Actions = semantics.NewSemanticsActions()
			config.Actions.SetHandler(semantics.SemanticsActionTap, func(args any) {
				w.OnChanged(!r.state.value)
			})
		}
	}

	return true
}
