package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/semantics"
	"github.com/go-drift/drift/pkg/theme"
)

// Checkbox displays a toggleable check control with theme-aware styling.
//
// Checkbox is a controlled component - it displays the Value you provide and
// calls OnChanged when tapped. To toggle the checkbox, update Value in your
// state in response to OnChanged.
//
// Example:
//
//	Checkbox{
//	    Value: isChecked,
//	    OnChanged: func(checked bool) {
//	        s.SetState(func() { s.isChecked = checked })
//	    },
//	}
//
// For form integration with validation, wrap in a [FormField]:
//
//	FormField[bool]{
//	    InitialValue: false,
//	    Validator: func(v bool) string {
//	        if !v { return "Must accept terms" }
//	        return ""
//	    },
//	    Builder: func(state *FormFieldState[bool]) core.Widget {
//	        return Checkbox{Value: state.Value(), OnChanged: state.DidChange}
//	    },
//	}
//
// The checkbox automatically uses colors from the current [theme.CheckboxTheme].
// Visual properties fall back to theme defaults when their value is zero and
// they have not been explicitly set via a WithX method. Use the WithX methods
// to set a value that should be used even when it equals zero (e.g.,
// [Checkbox.WithBorderRadius](0) for sharp corners).
type Checkbox struct {
	// Value indicates whether the checkbox is checked.
	Value bool
	// OnChanged is called when the checkbox is toggled.
	OnChanged func(bool)
	// Disabled disables interaction when true.
	Disabled bool
	// Size controls the checkbox square size.
	Size float64
	// BorderRadius controls the checkbox corner radius.
	BorderRadius float64
	// ActiveColor is the fill color when checked.
	ActiveColor graphics.Color
	// CheckColor is the checkmark color.
	CheckColor graphics.Color
	// BorderColor is the outline color when unchecked.
	BorderColor graphics.Color
	// BackgroundColor is the fill color when unchecked.
	BackgroundColor graphics.Color
	// overrides tracks which fields were explicitly set via WithX methods.
	overrides checkboxOverrides
}

type checkboxOverrides uint16

const (
	checkboxOverrideActiveColor     checkboxOverrides = 1 << iota
	checkboxOverrideCheckColor
	checkboxOverrideBorderColor
	checkboxOverrideBackgroundColor
	checkboxOverrideSize
	checkboxOverrideBorderRadius
)

// CheckboxOf creates a checkbox with the given value and change handler.
// This is a convenience helper equivalent to:
//
//	Checkbox{Value: value, OnChanged: onChanged}
func CheckboxOf(value bool, onChanged func(bool)) Checkbox {
	return Checkbox{Value: value, OnChanged: onChanged}
}

// WithColors returns a copy of the checkbox with the specified active fill and
// checkmark colors. The values are marked as explicitly set, bypassing theme
// defaults even when zero.
func (c Checkbox) WithColors(activeColor, checkColor graphics.Color) Checkbox {
	c.ActiveColor = activeColor
	c.CheckColor = checkColor
	c.overrides |= checkboxOverrideActiveColor | checkboxOverrideCheckColor
	return c
}

// WithSize returns a copy of the checkbox with the specified square size.
// The value is marked as explicitly set, bypassing theme defaults even when zero.
func (c Checkbox) WithSize(size float64) Checkbox {
	c.Size = size
	c.overrides |= checkboxOverrideSize
	return c
}

// WithBorderRadius returns a copy of the checkbox with the specified corner radius.
// The value is marked as explicitly set, so even zero (sharp corners) will be
// used instead of falling back to the theme default.
func (c Checkbox) WithBorderRadius(radius float64) Checkbox {
	c.BorderRadius = radius
	c.overrides |= checkboxOverrideBorderRadius
	return c
}

// WithBorderColor returns a copy of the checkbox with the specified outline color.
// The value is marked as explicitly set, bypassing theme defaults even when zero.
func (c Checkbox) WithBorderColor(color graphics.Color) Checkbox {
	c.BorderColor = color
	c.overrides |= checkboxOverrideBorderColor
	return c
}

// WithBackgroundColor returns a copy of the checkbox with the specified unchecked
// fill color. The value is marked as explicitly set, bypassing theme defaults
// even when zero.
func (c Checkbox) WithBackgroundColor(color graphics.Color) Checkbox {
	c.BackgroundColor = color
	c.overrides |= checkboxOverrideBackgroundColor
	return c
}

func (c Checkbox) CreateElement() core.Element {
	return core.NewStatelessElement(c, nil)
}

func (c Checkbox) Key() any {
	return nil
}

func (c Checkbox) Build(ctx core.BuildContext) core.Widget {
	themeData := theme.ThemeOf(ctx)
	checkboxTheme := themeData.CheckboxThemeOf()

	activeColor := c.ActiveColor
	if c.overrides&checkboxOverrideActiveColor == 0 && activeColor == 0 {
		activeColor = checkboxTheme.ActiveColor
	}
	checkColor := c.CheckColor
	if c.overrides&checkboxOverrideCheckColor == 0 && checkColor == 0 {
		checkColor = checkboxTheme.CheckColor
	}
	borderColor := c.BorderColor
	if c.overrides&checkboxOverrideBorderColor == 0 && borderColor == 0 {
		borderColor = checkboxTheme.BorderColor
	}
	backgroundColor := c.BackgroundColor
	if c.overrides&checkboxOverrideBackgroundColor == 0 && backgroundColor == 0 {
		backgroundColor = checkboxTheme.BackgroundColor
	}
	size := c.Size
	if c.overrides&checkboxOverrideSize == 0 && size == 0 {
		size = checkboxTheme.Size
	}
	borderRadius := c.BorderRadius
	if c.overrides&checkboxOverrideBorderRadius == 0 && borderRadius == 0 {
		borderRadius = checkboxTheme.BorderRadius
	}

	enabled := !c.Disabled && c.OnChanged != nil
	if !enabled {
		activeColor = checkboxTheme.DisabledActiveColor
		checkColor = checkboxTheme.DisabledCheckColor
		// backgroundColor stays as-is for unchecked state
	}

	return checkboxRender{
		value:           c.Value,
		onChanged:       c.OnChanged,
		enabled:         enabled,
		size:            size,
		borderRadius:    borderRadius,
		activeColor:     activeColor,
		checkColor:      checkColor,
		borderColor:     borderColor,
		backgroundColor: backgroundColor,
	}
}

type checkboxRender struct {
	value           bool
	onChanged       func(bool)
	enabled         bool
	size            float64
	borderRadius    float64
	activeColor     graphics.Color
	checkColor      graphics.Color
	borderColor     graphics.Color
	backgroundColor graphics.Color
}

func (c checkboxRender) CreateElement() core.Element {
	return core.NewRenderObjectElement(c, nil)
}

func (c checkboxRender) Key() any {
	return nil
}

func (c checkboxRender) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	r := &renderCheckbox{}
	r.SetSelf(r)
	r.update(c)
	return r
}

func (c checkboxRender) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if r, ok := renderObject.(*renderCheckbox); ok {
		r.update(c)
		r.MarkNeedsLayout()
		r.MarkNeedsPaint()
	}
}

type renderCheckbox struct {
	layout.RenderBoxBase
	value           bool
	onChanged       func(bool)
	enabled         bool
	size            float64
	borderRadius    float64
	activeColor     graphics.Color
	checkColor      graphics.Color
	borderColor     graphics.Color
	backgroundColor graphics.Color
	tap             *gestures.TapGestureRecognizer
}

func (r *renderCheckbox) update(c checkboxRender) {
	r.value = c.value
	r.onChanged = c.onChanged
	r.enabled = c.enabled
	r.size = c.size
	r.borderRadius = c.borderRadius
	r.activeColor = c.activeColor
	r.checkColor = c.checkColor
	r.borderColor = c.borderColor
	r.backgroundColor = c.backgroundColor
}

func (r *renderCheckbox) PerformLayout() {
	constraints := r.Constraints()
	size := r.size
	if size == 0 {
		size = 20
	}
	size = min(max(size, constraints.MinWidth), constraints.MaxWidth)
	size = min(max(size, constraints.MinHeight), constraints.MaxHeight)
	r.SetSize(graphics.Size{Width: size, Height: size})
}

func (r *renderCheckbox) Paint(ctx *layout.PaintContext) {
	size := r.Size()
	rect := graphics.RectFromLTWH(0, 0, size.Width, size.Height)

	fillPaint := graphics.DefaultPaint()
	if r.value {
		fillPaint.Color = r.activeColor
	} else {
		fillPaint.Color = r.backgroundColor
	}

	if r.borderRadius > 0 {
		rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(r.borderRadius))
		ctx.Canvas.DrawRRect(rrect, fillPaint)
	} else {
		ctx.Canvas.DrawRect(rect, fillPaint)
	}

	borderPaint := graphics.DefaultPaint()
	borderPaint.Color = r.borderColor
	borderPaint.Style = graphics.PaintStyleStroke
	borderPaint.StrokeWidth = 1
	if r.borderRadius > 0 {
		rrect := graphics.RRectFromRectAndRadius(rect, graphics.CircularRadius(r.borderRadius))
		ctx.Canvas.DrawRRect(rrect, borderPaint)
	} else {
		ctx.Canvas.DrawRect(rect, borderPaint)
	}

	if r.value {
		path := graphics.NewPath()
		path.MoveTo(size.Width*0.24, size.Height*0.55)
		path.LineTo(size.Width*0.44, size.Height*0.72)
		path.LineTo(size.Width*0.76, size.Height*0.32)
		checkPaint := graphics.DefaultPaint()
		checkPaint.Color = r.checkColor
		checkPaint.Style = graphics.PaintStyleStroke
		checkPaint.StrokeWidth = max(size.Width*0.12, 2)
		ctx.Canvas.DrawPath(path, checkPaint)
	}
}

func (r *renderCheckbox) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

func (r *renderCheckbox) HandlePointer(event gestures.PointerEvent) {
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
func (r *renderCheckbox) DescribeSemanticsConfiguration(config *semantics.SemanticsConfiguration) bool {
	config.IsSemanticBoundary = true
	config.Properties.Role = semantics.SemanticsRoleCheckbox

	// Set flags
	flags := semantics.SemanticsHasCheckedState | semantics.SemanticsHasEnabledState
	if r.value {
		flags = flags.Set(semantics.SemanticsIsChecked)
	}
	if r.enabled {
		flags = flags.Set(semantics.SemanticsIsEnabled)
	}
	config.Properties.Flags = flags

	// Set value description
	if r.value {
		config.Properties.Value = "Checked"
	} else {
		config.Properties.Value = "Not checked"
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
