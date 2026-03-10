package core

import "reflect"

// InheritedProvider is a generic inherited widget that eliminates boilerplate
// for simple data-down-the-tree patterns.
//
// Example usage:
//
//	// Provide a value
//	core.InheritedProvider[*User]{
//	    Value: currentUser,
//	    Child: MainContent{},
//	}
//
//	// Consume from anywhere in the subtree
//	if user, ok := core.Provide[*User](ctx); ok {
//	    // use user
//	}
//
// For custom comparison logic, set ShouldRebuild:
//
//	core.InheritedProvider[*User]{
//	    Value: currentUser,
//	    Child: MainContent{},
//	    ShouldRebuild: func(old, new *User) bool {
//	        return old.ID != new.ID  // Only rebuild on ID change
//	    },
//	}
//
// Note: The default comparison uses == which panics for non-comparable types
// (slices, maps, funcs). For these types, you must provide a ShouldRebuild function.
//
// For more advanced use cases like aspect-based tracking, implement a custom
// InheritedWidget instead.
type InheritedProvider[T any] struct {
	// Value is the data to provide to descendants.
	Value T

	// Child is the child widget tree.
	Child Widget

	// WidgetKey is an optional key for widget identity.
	WidgetKey any

	// ShouldRebuild is an optional function to customize when dependents rebuild.
	// If nil, defaults to value inequality (any(old) != any(new)).
	// Required for non-comparable types (slices, maps, funcs) to avoid panics.
	ShouldRebuild func(old, new T) bool
}

// CreateElement implements Widget.
func (p InheritedProvider[T]) CreateElement() Element {
	return NewInheritedElement()
}

// Key implements Widget.
func (p InheritedProvider[T]) Key() any {
	return p.WidgetKey
}

// ChildWidget implements InheritedWidget.
func (p InheritedProvider[T]) ChildWidget() Widget {
	return p.Child
}

// ShouldRebuildDependents implements InheritedWidget.
// Returns true if dependents should rebuild when this widget is updated.
func (p InheritedProvider[T]) ShouldRebuildDependents(oldWidget InheritedWidget) bool {
	old, ok := oldWidget.(InheritedProvider[T])
	if !ok {
		return true
	}

	if p.ShouldRebuild != nil {
		return p.ShouldRebuild(old.Value, p.Value)
	}

	// Default comparison: use any() to compare values.
	// For pointers, this is pointer equality.
	// For value types, this is value equality.
	return any(old.Value) != any(p.Value)
}

// Provide finds and depends on the nearest ancestor InheritedProvider[T].
// Returns the value and true if found, or the zero value and false if not found.
//
// Example:
//
//	if user, ok := core.Provide[*User](ctx); ok {
//	    fmt.Println("Hello,", user.Name)
//	}
func Provide[T any](ctx BuildContext) (T, bool) {
	providerType := reflect.TypeFor[InheritedProvider[T]]()
	widget := ctx.DependOnInherited(providerType, nil)
	if widget == nil {
		var zero T
		return zero, false
	}
	if provider, ok := widget.(InheritedProvider[T]); ok {
		return provider.Value, true
	}
	var zero T
	return zero, false
}

// MustProvide finds and depends on the nearest ancestor InheritedProvider[T].
// Panics if not found in the ancestor chain.
//
// Example:
//
//	user := core.MustProvide[*User](ctx)
//	fmt.Println("Hello,", user.Name)
func MustProvide[T any](ctx BuildContext) T {
	value, ok := Provide[T](ctx)
	if !ok {
		panic("MustProvide: no InheritedProvider[" + reflect.TypeFor[T]().String() + "] found in ancestors")
	}
	return value
}
