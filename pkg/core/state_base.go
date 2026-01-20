package core

import "sync"

// StateBase provides common functionality for stateful widget states.
// Embed this struct in your state to eliminate boilerplate.
//
// Example:
//
//	type myState struct {
//	    core.StateBase
//	    count int
//	}
//
//	func (s *myState) InitState() {
//	    // No need to implement SetElement, SetState, Dispose, etc.
//	}
type StateBase struct {
	element   *StatefulElement
	disposers []func()
	disposed  bool
	mu        sync.Mutex
}

// setElement stores the element reference for triggering rebuilds.
// This method is called automatically by the framework.
func (s *StateBase) setElement(element *StatefulElement) {
	s.element = element
}

// SetElement stores the element reference for triggering rebuilds.
// This method is called automatically by the framework.
func (s *StateBase) SetElement(element *StatefulElement) {
	s.element = element
}

// Element returns the element associated with this state.
// Returns nil if the state has been disposed or not yet mounted.
func (s *StateBase) Element() *StatefulElement {
	return s.element
}

// SetState executes the given function and schedules a rebuild.
// Safe to call even after disposal (becomes a no-op).
//
// SetState is NOT thread-safe. It must only be called from the UI thread.
// To update state from a background goroutine, use drift.Dispatch.
func (s *StateBase) SetState(fn func()) {
	if s.disposed {
		return
	}
	if fn != nil {
		fn()
	}
	if s.element != nil {
		s.element.MarkNeedsBuild()
	}
}

// OnDispose registers a cleanup function to be called when the state is disposed.
// Returns an unregister function that can be called to remove the disposer.
// The cleanup function will only be called once.
func (s *StateBase) OnDispose(cleanup func()) func() {
	if cleanup == nil {
		return func() {}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.disposed {
		// Already disposed, run cleanup immediately
		cleanup()
		return func() {}
	}

	index := len(s.disposers)
	s.disposers = append(s.disposers, cleanup)

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if index < len(s.disposers) {
			s.disposers[index] = nil
		}
	}
}

// RunDisposers executes all registered disposers in reverse order.
// This is called automatically by Dispose().
func (s *StateBase) RunDisposers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.disposed {
		return
	}
	s.disposed = true

	// Run disposers in reverse order (LIFO)
	for i := len(s.disposers) - 1; i >= 0; i-- {
		if s.disposers[i] != nil {
			s.disposers[i]()
		}
	}
	s.disposers = nil
}

// Dispose cleans up resources. Override this method if you need custom cleanup,
// but always call s.RunDisposers() or s.StateBase.Dispose() in your override.
func (s *StateBase) Dispose() {
	s.RunDisposers()
}

// InitState is a no-op default implementation.
// Override this method to initialize your state.
func (s *StateBase) InitState() {}

// Build is a no-op default implementation that returns nil.
// Override this method to build your widget tree.
func (s *StateBase) Build(ctx BuildContext) Widget {
	return nil
}

// DidChangeDependencies is a no-op default implementation.
// Override this method to respond to inherited widget changes.
func (s *StateBase) DidChangeDependencies() {}

// DidUpdateWidget is a no-op default implementation.
// Override this method to respond to widget configuration changes.
func (s *StateBase) DidUpdateWidget(oldWidget StatefulWidget) {}

// IsDisposed returns true if this state has been disposed.
func (s *StateBase) IsDisposed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disposed
}
