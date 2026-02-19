package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/semantics"
)

// Checkbox displays a toggleable check control with customizable styling.
//
// # Styling Model
//
// Checkbox is explicit by default — all visual properties use their struct field
// values directly. A zero value means zero, not "use theme default." For example:
//
//   - ActiveColor: 0 means transparent fill when checked
//   - Size: 0 means zero size (not rendered)
//   - BorderRadius: 0 means sharp corners
//
// For theme-styled checkboxes, use [theme.CheckboxOf] which pre-fills visual
// properties from the current theme's [theme.CheckboxThemeData].
//
// # Creation Patterns
//
// Struct literal (full control):
//
//	widgets.Checkbox{
//	    Value:       isChecked,
//	    OnChanged:   func(v bool) { s.SetState(func() { s.isChecked = v }) },
//	    ActiveColor: graphics.RGB(33, 150, 243),
//	    CheckColor:  graphics.ColorWhite,
//	    Size:        24,
//	}
//
// Themed (reads from current theme):
//
//	theme.CheckboxOf(ctx, isChecked, onChanged)
//	// Pre-filled with theme colors, size, border radius
//
// Checkbox is a controlled component - it displays the Value you provide and
// calls OnChanged when tapped. To toggle the checkbox, update Value in your
// state in response to OnChanged.
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
type Checkbox struct {
	core.StatelessBase

	// Value indicates whether the checkbox is checked.
	Value bool

	// OnChanged is called when the checkbox is toggled.
	OnChanged func(bool)

	// Disabled disables interaction when true.
	Disabled bool

	// Size controls the checkbox square size. Zero means zero size (not rendered).
	Size float64

	// BorderRadius controls the checkbox corner radius. Zero means sharp corners.
	BorderRadius float64

	// ActiveColor is the fill color when checked. Zero means transparent.
	ActiveColor graphics.Color

	// CheckColor is the checkmark color. Zero means transparent (invisible check).
	CheckColor graphics.Color

	// BorderColor is the outline color. Zero means no border.
	BorderColor graphics.Color

	// BackgroundColor is the fill color when unchecked. Zero means transparent.
	BackgroundColor graphics.Color

	// DisabledActiveColor is the fill color when checked and disabled.
	// If zero, falls back to 0.5 opacity on the normal colors.
	DisabledActiveColor graphics.Color

	// DisabledCheckColor is the checkmark color when disabled.
	// If zero, falls back to 0.5 opacity on the normal colors.
	DisabledCheckColor graphics.Color
}

// WithColors returns a copy of the checkbox with the specified active fill and
// checkmark colors.
func (c Checkbox) WithColors(activeColor, checkColor graphics.Color) Checkbox {
	c.ActiveColor = activeColor
	c.CheckColor = checkColor
	return c
}

// WithSize returns a copy of the checkbox with the specified square size.
func (c Checkbox) WithSize(size float64) Checkbox {
	c.Size = size
	return c
}

// WithBorderRadius returns a copy of the checkbox with the specified corner radius.
func (c Checkbox) WithBorderRadius(radius float64) Checkbox {
	c.BorderRadius = radius
	return c
}

// WithBorderColor returns a copy of the checkbox with the specified outline color.
func (c Checkbox) WithBorderColor(color graphics.Color) Checkbox {
	c.BorderColor = color
	return c
}

// WithBackgroundColor returns a copy of the checkbox with the specified unchecked
// fill color.
func (c Checkbox) WithBackgroundColor(color graphics.Color) Checkbox {
	c.BackgroundColor = color
	return c
}

func (c Checkbox) Build(ctx core.BuildContext) core.Widget {
	// Use field values directly — zero means zero.
	activeColor := c.ActiveColor
	checkColor := c.CheckColor
	borderColor := c.BorderColor
	backgroundColor := c.BackgroundColor
	size := c.Size
	borderRadius := c.BorderRadius

	enabled := !c.Disabled && c.OnChanged != nil

	// Apply disabled styling when not enabled (either Disabled=true or OnChanged=nil).
	// This ensures widgets with nil handlers also appear disabled.
	useOpacityFallback := false
	if !enabled {
		if c.DisabledActiveColor != 0 || c.DisabledCheckColor != 0 {
			// At least one disabled color set - use explicit disabled styling
			// Apply 50% alpha to any color without an explicit disabled variant
			if c.DisabledActiveColor != 0 {
				activeColor = c.DisabledActiveColor
			} else {
				activeColor = activeColor.WithAlpha(0.5)
			}
			if c.DisabledCheckColor != 0 {
				checkColor = c.DisabledCheckColor
			} else {
				checkColor = checkColor.WithAlpha(0.5)
			}
		} else {
			// No disabled colors set - use opacity fallback
			useOpacityFallback = true
		}
	}

	var result core.Widget = checkboxRender{
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

	// Fall back to opacity if no disabled colors provided
	if useOpacityFallback {
		result = Opacity{Opacity: 0.5, Child: result}
	}

	return result
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
	return core.NewRenderObjectElement()
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

	// No explicit Value or Hint: TalkBack/VoiceOver derive "checked"/
	// "not checked" from HasCheckedState and "double tap to toggle"
	// from the clickable+checkable combination automatically.

	// Set action
	if r.enabled && r.onChanged != nil {
		config.Actions = semantics.NewSemanticsActions()
		config.Actions.SetHandler(semantics.SemanticsActionTap, func(args any) {
			r.onChanged(!r.value)
		})
	}

	return true
}
