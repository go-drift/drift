// Package core provides the core widget and element framework.
package core

import "github.com/go-drift/drift/pkg/layout"

// RenderObjectWidget creates a render object directly.
type RenderObjectWidget interface {
	Widget
	CreateRenderObject(ctx BuildContext) layout.RenderObject
	UpdateRenderObject(ctx BuildContext, renderObject layout.RenderObject)
}

// Stateful creates a stateful widget using generics.
// For more control over lifecycle callbacks, use StatefulBuilder instead.
func Stateful[S any](
	init func() S,
	build func(state S, setState func(func(S) S)) Widget,
) Widget {
	return &statefulBuilderWidget[S]{
		config: StatefulBuilder[S]{
			Init: init,
			Build: func(state S, _ BuildContext, setState func(func(S) S)) Widget {
				return build(state, setState)
			},
		},
	}
}

// StatefulBuilder provides a declarative way to create stateful widgets
// with full lifecycle support.
//
// Example:
//
//	core.StatefulBuilder[int]{
//	    Init: func() int { return 0 },
//	    Build: func(count int, ctx core.BuildContext, setState func(func(int) int)) core.Widget {
//	        return widgets.GestureDetector{
//	            OnTap: func() { setState(func(c int) int { return c + 1 }) },
//	            Child: widgets.TextOf(fmt.Sprintf("Count: %d", count), ...),
//	        }
//	    },
//	    Dispose: func(count int) {
//	        // cleanup resources
//	    },
//	}.Widget()
type StatefulBuilder[S any] struct {
	// Init creates the initial state value. Required.
	Init func() S

	// Build creates the widget tree. Required.
	// The setState function updates the state and triggers a rebuild.
	Build func(state S, ctx BuildContext, setState func(func(S) S)) Widget

	// Dispose is called when the widget is removed from the tree. Optional.
	Dispose func(state S)

	// DidChangeDependencies is called when inherited widgets change. Optional.
	DidChangeDependencies func(state S, ctx BuildContext)

	// DidUpdateWidget is called when the widget configuration changes. Optional.
	DidUpdateWidget func(state S, oldWidget StatefulWidget)

	// WidgetKey is an optional key for the widget.
	WidgetKey any
}

// Widget returns a Widget that can be used in the widget tree.
func (b StatefulBuilder[S]) Widget() Widget {
	return &statefulBuilderWidget[S]{config: b}
}

type statefulBuilderWidget[S any] struct {
	config StatefulBuilder[S]
}

func (s *statefulBuilderWidget[S]) CreateElement() Element {
	return NewStatefulElement(s, nil)
}

func (s *statefulBuilderWidget[S]) Key() any {
	return s.config.WidgetKey
}

func (s *statefulBuilderWidget[S]) CreateState() State {
	return &statefulBuilderState[S]{config: s.config}
}

type statefulBuilderState[S any] struct {
	value   S
	config  StatefulBuilder[S]
	element *StatefulElement
}

// SetElement stores the element reference for triggering rebuilds.
func (s *statefulBuilderState[S]) SetElement(element *StatefulElement) {
	s.element = element
}

// InitState initializes the state value using the init function.
func (s *statefulBuilderState[S]) InitState() {
	if s.config.Init != nil {
		s.value = s.config.Init()
	}
}

// Build invokes the build function with the current state and a setState callback.
func (s *statefulBuilderState[S]) Build(ctx BuildContext) Widget {
	if s.config.Build != nil {
		return s.config.Build(s.value, ctx, func(update func(S) S) {
			s.value = update(s.value)
			if s.element != nil {
				s.element.MarkNeedsBuild()
			}
		})
	}
	return nil
}

// SetState executes the given function and schedules a rebuild.
func (s *statefulBuilderState[S]) SetState(fn func()) {
	if fn != nil {
		fn()
	}
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

// Dispose calls the dispose callback if provided.
func (s *statefulBuilderState[S]) Dispose() {
	if s.config.Dispose != nil {
		s.config.Dispose(s.value)
	}
}

// DidChangeDependencies calls the callback if provided.
func (s *statefulBuilderState[S]) DidChangeDependencies() {
	if s.config.DidChangeDependencies != nil && s.element != nil {
		s.config.DidChangeDependencies(s.value, s.element)
	}
}

// DidUpdateWidget calls the callback if provided.
func (s *statefulBuilderState[S]) DidUpdateWidget(oldWidget StatefulWidget) {
	if s.config.DidUpdateWidget != nil {
		s.config.DidUpdateWidget(s.value, oldWidget)
	}
}
