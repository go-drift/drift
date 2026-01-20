package widgets

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/theme"
)

// Radio renders a single radio button in a group.
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
	ActiveColor rendering.Color
	// InactiveColor is the unselected border color.
	InactiveColor rendering.Color
	// BackgroundColor is the fill color when unselected.
	BackgroundColor rendering.Color
}

func (r Radio[T]) CreateElement() core.Element {
	return core.NewStatelessElement(r, nil)
}

func (r Radio[T]) Key() any {
	return nil
}

func (r Radio[T]) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)
	activeColor := r.ActiveColor
	if activeColor == 0 {
		activeColor = colors.Primary
	}
	inactiveColor := r.InactiveColor
	if inactiveColor == 0 {
		inactiveColor = colors.Outline
	}
	backgroundColor := r.BackgroundColor
	if backgroundColor == 0 {
		backgroundColor = colors.Surface
	}

	enabled := !r.Disabled && r.OnChanged != nil
	selected := reflect.DeepEqual(r.Value, r.GroupValue)
	if !enabled {
		activeColor = colors.SurfaceVariant
		inactiveColor = colors.Outline
		backgroundColor = colors.SurfaceVariant
	}

	return radioRender[T]{
		selected:        selected,
		onChanged:       r.OnChanged,
		value:           r.Value,
		enabled:         enabled,
		size:            r.Size,
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
	activeColor     rendering.Color
	inactiveColor   rendering.Color
	backgroundColor rendering.Color
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
	activeColor     rendering.Color
	inactiveColor   rendering.Color
	backgroundColor rendering.Color
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

func (r *renderRadio[T]) Layout(constraints layout.Constraints) {
	size := r.size
	if size == 0 {
		size = 20
	}
	size = min(max(size, constraints.MinWidth), constraints.MaxWidth)
	size = min(max(size, constraints.MinHeight), constraints.MaxHeight)
	r.SetSize(rendering.Size{Width: size, Height: size})
}

func (r *renderRadio[T]) Paint(ctx *layout.PaintContext) {
	size := r.Size()
	center := rendering.Offset{X: size.Width / 2, Y: size.Height / 2}
	radius := size.Width / 2

	fillPaint := rendering.DefaultPaint()
	fillPaint.Color = r.backgroundColor
	ctx.Canvas.DrawCircle(center, radius, fillPaint)

	borderPaint := rendering.DefaultPaint()
	borderPaint.Color = r.inactiveColor
	borderPaint.Style = rendering.PaintStyleStroke
	borderPaint.StrokeWidth = 1
	ctx.Canvas.DrawCircle(center, radius-0.5, borderPaint)

	if r.selected {
		innerPaint := rendering.DefaultPaint()
		innerPaint.Color = r.activeColor
		ctx.Canvas.DrawCircle(center, radius*0.45, innerPaint)
	}
}

func (r *renderRadio[T]) HitTest(position rendering.Offset, result *layout.HitTestResult) bool {
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
