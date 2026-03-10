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
// Signal provides thread-safe reactive values with automatic rebuild:
//
//	s.counter = core.NewSignal(0)
//	core.UseListenable(s, s.counter) // Subscribe for rebuilds
//	s.counter.Set(5)                 // Triggers rebuild
//
// # Hooks
//
// UseDisposable and UseListenable help manage resources and subscriptions
// with automatic cleanup on disposal.
//
// # Constructor Conventions
//
// Controllers and services use NewX() constructors returning pointers:
//
//	ctrl := animation.NewAnimationController(time.Second)
//	channel := platform.NewMethodChannel("app.channel")
//
// This distinguishes long-lived, mutable objects (controllers) from
// immutable configuration objects (widgets, which use struct literals
// or XxxOf() helpers).
package core
