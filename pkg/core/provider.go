package core

import "reflect"

// InheritedProvider is a generic inherited widget that eliminates boilerplate
// for simple data-down-the-tree patterns.
//
// Example usage:
//
//	// Provide a value
//	core.InheritedProvider[*User]{
//	    Value:       currentUser,
//	    ChildWidget: MainContent{},
//	}
//
//	// Consume from anywhere in the subtree
//	if user, ok := core.ProviderOf[*User](ctx); ok {
//	    // use user
//	}
//
// For custom comparison logic, set ShouldNotify:
//
//	core.InheritedProvider[*User]{
//	    Value:       currentUser,
//	    ChildWidget: MainContent{},
//	    ShouldNotify: func(old, new *User) bool {
//	        return old.ID != new.ID  // Only notify on ID change
//	    },
//	}
//
// Note: The default comparison uses == which panics for non-comparable types
// (slices, maps, funcs). For these types, you must provide a ShouldNotify function.
//
// For more advanced use cases like aspect-based tracking, implement a custom
// InheritedWidget instead.
type InheritedProvider[T any] struct {
	// Value is the data to provide to descendants.
	Value T

	// ChildWidget is the child widget tree.
	ChildWidget Widget

	// WidgetKey is an optional key for widget identity.
	WidgetKey any

	// ShouldNotify is an optional function to customize when dependents rebuild.
	// If nil, defaults to value inequality (any(old) != any(new)).
	// Required for non-comparable types (slices, maps, funcs) to avoid panics.
	ShouldNotify func(old, new T) bool
}

// CreateElement implements Widget.
func (p InheritedProvider[T]) CreateElement() Element {
	return NewInheritedElement(p, nil)
}

// Key implements Widget.
func (p InheritedProvider[T]) Key() any {
	return p.WidgetKey
}

// Child implements InheritedWidget.
func (p InheritedProvider[T]) Child() Widget {
	return p.ChildWidget
}

// UpdateShouldNotify implements InheritedWidget.
// Returns true if dependents should rebuild when this widget is updated.
func (p InheritedProvider[T]) UpdateShouldNotify(oldWidget InheritedWidget) bool {
	old, ok := oldWidget.(InheritedProvider[T])
	if !ok {
		return true
	}

	if p.ShouldNotify != nil {
		return p.ShouldNotify(old.Value, p.Value)
	}

	// Default comparison: use any() to compare values.
	// For pointers, this is pointer equality.
	// For value types, this is value equality.
	return any(old.Value) != any(p.Value)
}

// UpdateShouldNotifyDependent implements InheritedWidget.
// InheritedProvider does not support aspect-based tracking, so this
// delegates to UpdateShouldNotify.
func (p InheritedProvider[T]) UpdateShouldNotifyDependent(oldWidget InheritedWidget, _ map[any]struct{}) bool {
	return p.UpdateShouldNotify(oldWidget)
}

// ProviderOf finds and depends on the nearest ancestor InheritedProvider[T].
// Returns the value and true if found, or the zero value and false if not found.
//
// Example:
//
//	if user, ok := core.ProviderOf[*User](ctx); ok {
//	    fmt.Println("Hello,", user.Name)
//	}
func ProviderOf[T any](ctx BuildContext) (T, bool) {
	providerType := reflect.TypeOf(InheritedProvider[T]{})
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

// MustProviderOf finds and depends on the nearest ancestor InheritedProvider[T].
// Panics if not found in the ancestor chain.
//
// Example:
//
//	user := core.MustProviderOf[*User](ctx)
//	fmt.Println("Hello,", user.Name)
func MustProviderOf[T any](ctx BuildContext) T {
	value, ok := ProviderOf[T](ctx)
	if !ok {
		panic("MustProviderOf: no InheritedProvider[" + reflect.TypeFor[T]().String() + "] found in ancestors")
	}
	return value
}
