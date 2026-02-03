package widgets

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// Radio renders a single radio button that is part of a mutually exclusive group.
//
// # Styling Model
//
// Radio is explicit by default — all visual properties use their struct field
// values directly. A zero value means zero, not "use theme default." For example:
//
//   - ActiveColor: 0 means transparent inner dot when selected
//   - Size: 0 means zero size (not rendered)
//
// For theme-styled radio buttons, use [theme.RadioOf] which pre-fills visual
// properties from the current theme's [theme.RadioThemeData].
//
// # Creation Patterns
//
// Explicit with struct literal (full control):
//
//	widgets.Radio[string]{
//	    Value:       "small",
//	    GroupValue:  selectedSize,
//	    OnChanged:   func(v string) { s.SetState(func() { s.selectedSize = v }) },
//	    ActiveColor: graphics.RGB(33, 150, 243),
//	    Size:        24,
//	}
//
// Themed (reads from current theme):
//
//	theme.RadioOf(ctx, "small", selectedSize, onChanged)
//	// Pre-filled with theme colors and size
//
// Radio is a generic widget where T is the type of the selection value. Each Radio
// in a group has its own Value, and all share the same GroupValue (the current
// selection). When a Radio is tapped, OnChanged is called with that Radio's Value.
//
// Example (string values):
//
//	var selected string = "small"
//
//	Column{Children: []core.Widget{
//	    Row{Children: []core.Widget{
//	        Radio[string]{Value: "small", GroupValue: selected, OnChanged: onSelect},
//	        Text{Content: "Small"},
//	    }},
//	    Row{Children: []core.Widget{
//	        Radio[string]{Value: "medium", GroupValue: selected, OnChanged: onSelect},
//	        Text{Content: "Medium"},
//	    }},
//	    Row{Children: []core.Widget{
//	        Radio[string]{Value: "large", GroupValue: selected, OnChanged: onSelect},
//	        Text{Content: "Large"},
//	    }},
//	}}
type Radio[T any] struct {
	// Value is the value for this radio.
	Value T
	// GroupValue is the current group selection.
	GroupValue T
	// OnChanged is called when this radio is selected.
	OnChanged func(T)
	// Disabled disables interaction when true.
	Disabled bool
	// Size controls the radio diameter. Zero means zero size (not rendered).
	Size float64
	// ActiveColor is the selected inner dot color. Zero means transparent.
	ActiveColor graphics.Color
	// InactiveColor is the unselected border color. Zero means transparent.
	InactiveColor graphics.Color
	// BackgroundColor is the fill color when unselected. Zero means transparent.
	BackgroundColor graphics.Color

	// DisabledActiveColor is the selected inner dot color when disabled.
	// If zero, falls back to 0.5 opacity on the normal colors.
	DisabledActiveColor graphics.Color

	// DisabledInactiveColor is the unselected border color when disabled.
	// If zero, falls back to 0.5 opacity on the normal colors.
	DisabledInactiveColor graphics.Color
}

func (r Radio[T]) CreateElement() core.Element {
	return core.NewStatelessElement(r, nil)
}

func (r Radio[T]) Key() any {
	return nil
}

func (r Radio[T]) Build(ctx core.BuildContext) core.Widget {
	// Use field values directly — zero means zero
	activeColor := r.ActiveColor
	inactiveColor := r.InactiveColor
	backgroundColor := r.BackgroundColor
	size := r.Size

	enabled := !r.Disabled && r.OnChanged != nil
	selected := reflect.DeepEqual(r.Value, r.GroupValue)

	// Apply disabled styling when not enabled (either Disabled=true or OnChanged=nil).
	// This ensures widgets with nil handlers also appear disabled.
	useOpacityFallback := false
	if !enabled {
		if r.DisabledActiveColor != 0 || r.DisabledInactiveColor != 0 {
			// At least one disabled color set - use explicit disabled styling
			// Apply 50% alpha to any color without an explicit disabled variant
			if r.DisabledActiveColor != 0 {
				activeColor = r.DisabledActiveColor
			} else {
				activeColor = activeColor.WithAlpha(0.5)
			}
			if r.DisabledInactiveColor != 0 {
				inactiveColor = r.DisabledInactiveColor
			} else {
				inactiveColor = inactiveColor.WithAlpha(0.5)
			}
		} else {
			// No disabled colors set - use opacity fallback
			useOpacityFallback = true
		}
	}

	var result core.Widget = radioRender[T]{
		selected:        selected,
		onChanged:       r.OnChanged,
		value:           r.Value,
		enabled:         enabled,
		size:            size,
		activeColor:     activeColor,
		inactiveColor:   inactiveColor,
		backgroundColor: backgroundColor,
	}

	// Fall back to opacity if no disabled colors provided
	if useOpacityFallback {
		result = Opacity{Opacity: 0.5, Child: result}
	}

	return result
}

type radioRender[T any] struct {
	selected        bool
	value           T
	onChanged       func(T)
	enabled         bool
	size            float64
	activeColor     graphics.Color
	inactiveColor   graphics.Color
	backgroundColor graphics.Color
}

func (r radioRender[T]) CreateElement() core.Element {
	return core.NewRenderObjectElement(r, nil)
}

func (r radioRender[T]) Key() any {
	return nil
}

func (r radioRender[T]) CreateRenderObject(ctx core.BuildContext) layout.RenderObject {
	obj := &renderRadio[T]{}
	obj.SetSelf(obj)
	obj.update(r)
	return obj
}

func (r radioRender[T]) UpdateRenderObject(ctx core.BuildContext, renderObject layout.RenderObject) {
	if obj, ok := renderObject.(*renderRadio[T]); ok {
		obj.update(r)
		obj.MarkNeedsLayout()
		obj.MarkNeedsPaint()
	}
}

type renderRadio[T any] struct {
	layout.RenderBoxBase
	selected        bool
	value           T
	onChanged       func(T)
	enabled         bool
	size            float64
	activeColor     graphics.Color
	inactiveColor   graphics.Color
	backgroundColor graphics.Color
	tap             *gestures.TapGestureRecognizer
}

func (r *renderRadio[T]) update(c radioRender[T]) {
	r.selected = c.selected
	r.value = c.value
	r.onChanged = c.onChanged
	r.enabled = c.enabled
	r.size = c.size
	r.activeColor = c.activeColor
	r.inactiveColor = c.inactiveColor
	r.backgroundColor = c.backgroundColor
}

func (r *renderRadio[T]) PerformLayout() {
	constraints := r.Constraints()
	size := r.size
	size = min(max(size, constraints.MinWidth), constraints.MaxWidth)
	size = min(max(size, constraints.MinHeight), constraints.MaxHeight)
	r.SetSize(graphics.Size{Width: size, Height: size})
}

func (r *renderRadio[T]) Paint(ctx *layout.PaintContext) {
	size := r.Size()
	center := graphics.Offset{X: size.Width / 2, Y: size.Height / 2}
	radius := size.Width / 2

	fillPaint := graphics.DefaultPaint()
	fillPaint.Color = r.backgroundColor
	ctx.Canvas.DrawCircle(center, radius, fillPaint)

	borderPaint := graphics.DefaultPaint()
	borderPaint.Color = r.inactiveColor
	borderPaint.Style = graphics.PaintStyleStroke
	borderPaint.StrokeWidth = 1
	ctx.Canvas.DrawCircle(center, radius-0.5, borderPaint)

	if r.selected {
		innerPaint := graphics.DefaultPaint()
		innerPaint.Color = r.activeColor
		ctx.Canvas.DrawCircle(center, radius*0.45, innerPaint)
	}
}

func (r *renderRadio[T]) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	if !withinBounds(position, r.Size()) {
		return false
	}
	result.Add(r)
	return true
}

func (r *renderRadio[T]) HandlePointer(event gestures.PointerEvent) {
	if !r.enabled {
		return
	}
	if r.tap == nil {
		r.tap = gestures.NewTapGestureRecognizer(gestures.DefaultArena)
		r.tap.OnTap = func() {
			if r.onChanged != nil {
				r.onChanged(r.value)
			}
		}
	}
	if event.Phase == gestures.PointerPhaseDown {
		r.tap.AddPointer(event)
	} else {
		r.tap.HandleEvent(event)
	}
}
