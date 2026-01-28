package widgets

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
)

// Radio renders a single radio button that is part of a mutually exclusive group.
//
// Radio is a generic widget where T is the type of the selection value. Each Radio
// in a group has its own Value, and all share the same GroupValue (the current
// selection). When a Radio is tapped, OnChanged is called with that Radio's Value.
//
// Example (string values):
//
//	var selected string = "small"
//
//	Column{ChildrenWidgets: []core.Widget{
//	    Row{ChildrenWidgets: []core.Widget{
//	        Radio[string]{Value: "small", GroupValue: selected, OnChanged: onSelect},
//	        Text{Content: "Small"},
//	    }},
//	    Row{ChildrenWidgets: []core.Widget{
//	        Radio[string]{Value: "medium", GroupValue: selected, OnChanged: onSelect},
//	        Text{Content: "Medium"},
//	    }},
//	    Row{ChildrenWidgets: []core.Widget{
//	        Radio[string]{Value: "large", GroupValue: selected, OnChanged: onSelect},
//	        Text{Content: "Large"},
//	    }},
//	}}
//
// The radio automatically uses colors from the current [theme.RadioTheme].
type Radio[T any] struct {
	// Value is the value for this radio.
	Value T
	// GroupValue is the current group selection.
	GroupValue T
	// OnChanged is called when this radio is selected.
	OnChanged func(T)
	// Disabled disables interaction when true.
	Disabled bool
	// Size controls the radio diameter.
	Size float64
	// ActiveColor is the selected fill color.
	ActiveColor graphics.Color
	// InactiveColor is the unselected border color.
	InactiveColor graphics.Color
	// BackgroundColor is the fill color when unselected.
	BackgroundColor graphics.Color
}

func (r Radio[T]) CreateElement() core.Element {
	return core.NewStatelessElement(r, nil)
}

func (r Radio[T]) Key() any {
	return nil
}

func (r Radio[T]) Build(ctx core.BuildContext) core.Widget {
	themeData := theme.ThemeOf(ctx)
	radioTheme := themeData.RadioThemeOf()

	activeColor := r.ActiveColor
	if activeColor == 0 {
		activeColor = radioTheme.ActiveColor
	}
	inactiveColor := r.InactiveColor
	if inactiveColor == 0 {
		inactiveColor = radioTheme.InactiveColor
	}
	backgroundColor := r.BackgroundColor
	if backgroundColor == 0 {
		backgroundColor = radioTheme.BackgroundColor
	}
	size := r.Size
	if size == 0 {
		size = radioTheme.Size
	}

	enabled := !r.Disabled && r.OnChanged != nil
	selected := reflect.DeepEqual(r.Value, r.GroupValue)
	if !enabled {
		activeColor = radioTheme.DisabledActiveColor
		inactiveColor = radioTheme.DisabledInactiveColor
		// backgroundColor stays as-is for unselected state
	}

	return radioRender[T]{
		selected:        selected,
		onChanged:       r.OnChanged,
		value:           r.Value,
		enabled:         enabled,
		size:            size,
		activeColor:     activeColor,
		inactiveColor:   inactiveColor,
		backgroundColor: backgroundColor,
	}
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
	if size == 0 {
		size = 20
	}
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
