// Package core provides the widget and element framework interfaces and lifecycle.
//
// This package defines the foundational types for building reactive user interfaces:
// Widget, Element, State, and BuildContext. It follows a declarative UI model
// where widgets describe what the UI should look like, and the framework
// efficiently updates the actual UI to match.
//
// # Core Types
//
// Widget is an immutable description of part of the UI. Widgets are lightweight
// configuration objects that can be created frequently without performance concerns.
//
// Element is the instantiation of a Widget at a particular location in the tree.
// Elements manage the lifecycle and identity of widgets.
//
// # Stateful Widgets
//
// For widgets that need mutable state, embed StateBase in your state struct:
//
//	type myState struct {
//	    core.StateBase
//	    count int
//	}
//
//	func (s *myState) InitState() {
//	    // Initialize state here
//	}
//
//	func (s *myState) Build(ctx core.BuildContext) core.Widget {
//	    return widgets.Text{Content: fmt.Sprintf("Count: %d", s.count)}
//	}
//
// # State Management
//
// ManagedState provides automatic rebuild triggering:
//
//	s.count = core.NewManagedState(&s.StateBase, 0)
//	s.count.Set(s.count.Get() + 1) // Automatically triggers rebuild
//
// Observable provides thread-safe reactive values:
//
//	counter := core.NewObservable(0)
//	core.UseObservable(&s.StateBase, counter) // Subscribe to changes
//
// # Hooks
//
// UseController, UseListenable, and UseObservable help manage resources
// and subscriptions with automatic cleanup on disposal.
package core
