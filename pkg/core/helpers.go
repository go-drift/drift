// Package core provides the core widget and element framework.
package core

import (
	"github.com/go-drift/drift/pkg/layout"
)

// NewStatefulWidget creates a StatefulWidget from a state constructor function,
// eliminating the need for a dedicated widget struct and its boilerplate methods
// (CreateElement, Key, CreateState).
//
// This is useful for stateful widgets whose widget struct would otherwise be empty.
// Widgets that carry configuration fields still need the manual pattern.
//
// Usage:
//
//	core.NewStatefulWidget(func() *myState { return &myState{} })
//
// With an optional key:
//
//	core.NewStatefulWidget(func() *myState { return &myState{} }, "my-key")
func NewStatefulWidget[S State](createState func() S, key ...any) StatefulWidget {
	if len(key) > 1 {
		panic("NewStatefulWidget accepts at most one key")
	}
	var k any
	if len(key) > 0 {
		k = key[0]
	}
	return &statefulWidget[S]{
		createStateFn: createState,
		widgetKey:     k,
	}
}

type statefulWidget[S State] struct {
	createStateFn func() S
	widgetKey     any
}

func (w *statefulWidget[S]) CreateElement() Element {
	return NewStatefulElement(w, nil)
}

func (w *statefulWidget[S]) Key() any {
	return w.widgetKey
}

func (w *statefulWidget[S]) CreateState() State {
	return w.createStateFn()
}

// RenderObjectWidget creates a render object directly.
type RenderObjectWidget interface {
	Widget
	CreateRenderObject(ctx BuildContext) layout.RenderObject
	UpdateRenderObject(ctx BuildContext, renderObject layout.RenderObject)
}

// Stateful creates an inline stateful widget using closures.
// Use this for quick, self-contained UI fragments that don't need
// lifecycle hooks or StateBase features.
//
// For complex widgets with many state fields, lifecycle methods,
// ManagedState, or UseController, use NewStatefulWidget instead.
func Stateful[S any](
	init func() S,
	build func(state S, ctx BuildContext, setState func(func(S) S)) Widget,
) Widget {
	return &inlineStatefulWidget[S]{
		initFn:  init,
		buildFn: build,
	}
}

type inlineStatefulWidget[S any] struct {
	initFn  func() S
	buildFn func(state S, ctx BuildContext, setState func(func(S) S)) Widget
}

func (w *inlineStatefulWidget[S]) CreateElement() Element {
	return NewStatefulElement(w, nil)
}

func (w *inlineStatefulWidget[S]) Key() any { return nil }

func (w *inlineStatefulWidget[S]) CreateState() State {
	return &inlineStatefulState[S]{
		initFn:  w.initFn,
		buildFn: w.buildFn,
	}
}

type inlineStatefulState[S any] struct {
	value   S
	initFn  func() S
	buildFn func(state S, ctx BuildContext, setState func(func(S) S)) Widget
	element *StatefulElement
}

func (s *inlineStatefulState[S]) SetElement(element *StatefulElement) {
	s.element = element
}

func (s *inlineStatefulState[S]) InitState() {
	s.value = s.initFn()
}

func (s *inlineStatefulState[S]) Build(ctx BuildContext) Widget {
	return s.buildFn(s.value, ctx, func(update func(S) S) {
		s.value = update(s.value)
		if s.element != nil {
			s.element.MarkNeedsBuild()
		}
	})
}

func (s *inlineStatefulState[S]) SetState(fn func()) {
	if fn != nil {
		fn()
	}
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

func (s *inlineStatefulState[S]) Dispose()                               {}
func (s *inlineStatefulState[S]) DidChangeDependencies()                 {}
func (s *inlineStatefulState[S]) DidUpdateWidget(_ StatefulWidget) {}
