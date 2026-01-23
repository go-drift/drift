package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/semantics"
	"github.com/go-drift/drift/pkg/theme"
)

// Checkbox displays a toggleable check control.
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
	ActiveColor rendering.Color
	// CheckColor is the checkmark color.
	CheckColor rendering.Color
	// BorderColor is the outline color when unchecked.
	BorderColor rendering.Color
	// BackgroundColor is the fill color when unchecked.
	BackgroundColor rendering.Color
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
	if activeColor == 0 {
		activeColor = checkboxTheme.ActiveColor
	}
	checkColor := c.CheckColor
	if checkColor == 0 {
		checkColor = checkboxTheme.CheckColor
	}
	borderColor := c.BorderColor
	if borderColor == 0 {
		borderColor = checkboxTheme.BorderColor
	}
	backgroundColor := c.BackgroundColor
	if backgroundColor == 0 {
		backgroundColor = checkboxTheme.BackgroundColor
	}
	size := c.Size
	if size == 0 {
		size = checkboxTheme.Size
	}
	borderRadius := c.BorderRadius
	if borderRadius == 0 {
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
	activeColor     rendering.Color
	checkColor      rendering.Color
	borderColor     rendering.Color
	backgroundColor rendering.Color
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
	activeColor     rendering.Color
	checkColor      rendering.Color
	borderColor     rendering.Color
	backgroundColor rendering.Color
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
	r.SetSize(rendering.Size{Width: size, Height: size})
}

func (r *renderCheckbox) Paint(ctx *layout.PaintContext) {
	size := r.Size()
	rect := rendering.RectFromLTWH(0, 0, size.Width, size.Height)

	fillPaint := rendering.DefaultPaint()
	if r.value {
		fillPaint.Color = r.activeColor
	} else {
		fillPaint.Color = r.backgroundColor
	}

	if r.borderRadius > 0 {
		rrect := rendering.RRectFromRectAndRadius(rect, rendering.CircularRadius(r.borderRadius))
		ctx.Canvas.DrawRRect(rrect, fillPaint)
	} else {
		ctx.Canvas.DrawRect(rect, fillPaint)
	}

	borderPaint := rendering.DefaultPaint()
	borderPaint.Color = r.borderColor
	borderPaint.Style = rendering.PaintStyleStroke
	borderPaint.StrokeWidth = 1
	if r.borderRadius > 0 {
		rrect := rendering.RRectFromRectAndRadius(rect, rendering.CircularRadius(r.borderRadius))
		ctx.Canvas.DrawRRect(rrect, borderPaint)
	} else {
		ctx.Canvas.DrawRect(rect, borderPaint)
	}

	if r.value {
		path := rendering.NewPath()
		path.MoveTo(size.Width*0.24, size.Height*0.55)
		path.LineTo(size.Width*0.44, size.Height*0.72)
		path.LineTo(size.Width*0.76, size.Height*0.32)
		checkPaint := rendering.DefaultPaint()
		checkPaint.Color = r.checkColor
		checkPaint.Style = rendering.PaintStyleStroke
		checkPaint.StrokeWidth = max(size.Width*0.12, 2)
		ctx.Canvas.DrawPath(path, checkPaint)
	}
}

func (r *renderCheckbox) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
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
