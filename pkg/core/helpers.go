// Package core provides the core widget and element framework.
package core

import (
	"github.com/go-drift/drift/pkg/layout"
)

// StatelessBase provides default CreateElement and Key implementations for
// stateless widgets. Embed it in your widget struct to satisfy the Widget
// interface without boilerplate:
//
//	type Greeting struct {
//	    core.StatelessBase
//	    Name string
//	}
//
//	func (g Greeting) Build(ctx core.BuildContext) core.Widget {
//	    return widgets.Text{Content: "Hello, " + g.Name}
//	}
type StatelessBase struct{}

// CreateElement returns a new StatelessElement.
func (StatelessBase) CreateElement() Element { return NewStatelessElement() }

// Key returns nil (no key).
func (StatelessBase) Key() any { return nil }

// StatefulBase provides default CreateElement and Key implementations for
// stateful widgets. Embed it in your widget struct to satisfy the Widget
// interface without boilerplate:
//
//	type Counter struct {
//	    core.StatefulBase
//	}
//
//	func (Counter) CreateState() core.State { return &counterState{} }
type StatefulBase struct{}

// CreateElement returns a new StatefulElement.
func (StatefulBase) CreateElement() Element { return NewStatefulElement() }

// Key returns nil (no key).
func (StatefulBase) Key() any { return nil }

// InheritedBase provides default CreateElement and Key implementations for
// inherited widgets. Embed it in your widget struct along with a Child field
// and implement [InheritedWidget.UpdateShouldNotify] and
// [InheritedWidget.ChildWidget]:
//
//	type UserScope struct {
//	    core.InheritedBase
//	    User  *User
//	    Child core.Widget
//	}
//
//	func (u UserScope) ChildWidget() core.Widget { return u.Child }
//
//	func (u UserScope) UpdateShouldNotify(old core.InheritedWidget) bool {
//	    return u.User != old.(UserScope).User
//	}
type InheritedBase struct{}

// CreateElement returns a new InheritedElement.
func (InheritedBase) CreateElement() Element { return NewInheritedElement() }

// Key returns nil (no key).
func (InheritedBase) Key() any { return nil }

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
// Managed, or UseController, embed [StatefulBase] in a named struct instead.
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
	return NewStatefulElement()
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

func (s *inlineStatefulState[S]) Dispose()                         {}
func (s *inlineStatefulState[S]) DidChangeDependencies()           {}
func (s *inlineStatefulState[S]) DidUpdateWidget(_ StatefulWidget) {}
