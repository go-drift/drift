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
	// UpdateShouldNotifyDependent returns true if a specific dependent should rebuild
	// based on the aspects it registered. This enables granular rebuild optimization
	// where dependents only rebuild when their specific aspects change.
	UpdateShouldNotifyDependent(oldWidget InheritedWidget, aspects map[any]struct{}) bool
}

// BuildContext provides access to the element tree during build.
type BuildContext interface {
	Widget() Widget
	FindAncestor(predicate func(Element) bool) Element
	// DependOnInherited finds and depends on an ancestor InheritedWidget of the given type.
	// The aspect parameter enables granular dependency tracking: when non-nil, only changes
	// affecting that aspect will trigger rebuilds. Pass nil to depend on all changes.
	DependOnInherited(inheritedType reflect.Type, aspect any) any
	// DependOnInheritedWithAspects is like DependOnInherited but registers multiple aspects
	// in a single tree walk. More efficient when depending on multiple aspects.
	DependOnInheritedWithAspects(inheritedType reflect.Type, aspects ...any) any
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
