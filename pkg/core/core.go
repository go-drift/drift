package core

import "reflect"

// Widget is an immutable description of part of the UI.
type Widget interface {
	CreateElement() Element
	Key() any
}

// StatelessWidget builds UI without mutable state.
type StatelessWidget interface {
	Widget
	Build(ctx BuildContext) Widget
}

// StatefulWidget has mutable state that can change over time.
type StatefulWidget interface {
	Widget
	CreateState() State
}

// State holds mutable state for a StatefulWidget.
type State interface {
	InitState()
	Build(ctx BuildContext) Widget
	SetState(fn func())
	Dispose()
	DidChangeDependencies()
	DidUpdateWidget(oldWidget StatefulWidget)
}

// InheritedWidget provides data to descendants without explicit passing.
// Widgets can depend on an InheritedWidget via BuildContext.DependOnInherited.
type InheritedWidget interface {
	Widget
	// Child returns the child widget.
	Child() Widget
	// UpdateShouldNotify returns true if dependents should rebuild
	// when this widget is updated.
	UpdateShouldNotify(oldWidget InheritedWidget) bool
}

// BuildContext provides access to the element tree during build.
type BuildContext interface {
	Widget() Widget
	FindAncestor(predicate func(Element) bool) Element
	DependOnInherited(inheritedType reflect.Type) any
}

// Element is the instantiation of a Widget at a particular location in the tree.
type Element interface {
	Widget() Widget
	Mount(parent Element, slot any)
	Update(newWidget Widget)
	Unmount()
	MarkNeedsBuild()
	RebuildIfNeeded()
	VisitChildren(visitor func(Element) bool)
	Depth() int
}
