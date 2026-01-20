package core

// UseController creates a controller and registers it for automatic disposal.
// The controller will be disposed when the state is disposed.
//
// Example:
//
//	func (s *myState) InitState() {
//	    s.animation = core.UseController(&s.StateBase, func() *animation.AnimationController {
//	        return animation.NewAnimationController(300 * time.Millisecond)
//	    })
//	}
func UseController[C Disposable](base *StateBase, create func() C) C {
	controller := create()
	base.OnDispose(func() {
		controller.Dispose()
	})
	return controller
}

// UseListenable subscribes to a listenable and triggers rebuilds.
// The subscription is automatically cleaned up when the state is disposed.
//
// Example:
//
//	func (s *myState) InitState() {
//	    s.controller = core.UseController(&s.StateBase, func() *MyController {
//	        return NewMyController()
//	    })
//	    core.UseListenable(&s.StateBase, s.controller)
//	}
func UseListenable(base *StateBase, listenable Listenable) {
	unsub := listenable.AddListener(func() {
		base.SetState(nil)
	})
	base.OnDispose(unsub)
}

// UseObservable subscribes to an observable and triggers rebuilds when it changes.
// Call this once in InitState(), not in Build(). The subscription is automatically
// cleaned up when the state is disposed.
//
// Example:
//
//	func (s *myState) InitState() {
//	    s.counter = core.NewObservable(0)
//	    core.UseObservable(&s.StateBase, s.counter)
//	}
//
//	func (s *myState) Build(ctx core.BuildContext) core.Widget {
//	    // Use .Value() in Build to read the current value
//	    return widgets.TextOf(fmt.Sprintf("Count: %d", s.counter.Value()), ...)
//	}
func UseObservable[T any](base *StateBase, obs *Observable[T]) {
	unsub := obs.AddListener(func(T) {
		base.SetState(nil)
	})
	base.OnDispose(unsub)
}

// ManagedState holds a value and triggers rebuilds when it changes.
// Unlike Observable, it is tied to a specific StateBase.
//
// ManagedState is NOT thread-safe. It must only be accessed from the UI thread.
// To update from a background goroutine, use drift.Dispatch:
//
//	go func() {
//	    result := doExpensiveWork()
//	    drift.Dispatch(func() {
//	        s.data.Set(result)  // Safe - runs on UI thread
//	    })
//	}()
//
// Example:
//
//	type myState struct {
//	    core.StateBase
//	    count *core.ManagedState[int]
//	}
//
//	func (s *myState) InitState() {
//	    s.count = core.NewManagedState(&s.StateBase, 0)
//	}
//
//	func (s *myState) Build(ctx core.BuildContext) core.Widget {
//	    return widgets.GestureDetector{
//	        OnTap: func() { s.count.Set(s.count.Get() + 1) },
//	        Child: widgets.TextOf(fmt.Sprintf("Count: %d", s.count.Get()), ...),
//	    }
//	}
type ManagedState[T any] struct {
	base  *StateBase
	value T
}

// NewManagedState creates a new managed state value.
// Changes to this value will automatically trigger a rebuild.
func NewManagedState[T any](base *StateBase, initial T) *ManagedState[T] {
	return &ManagedState[T]{
		base:  base,
		value: initial,
	}
}

// Get returns the current value.
func (m *ManagedState[T]) Get() T {
	return m.value
}

// Set updates the value and triggers a rebuild.
func (m *ManagedState[T]) Set(value T) {
	m.value = value
	m.base.SetState(nil)
}

// Update applies a transformation to the current value and triggers a rebuild.
func (m *ManagedState[T]) Update(transform func(T) T) {
	m.value = transform(m.value)
	m.base.SetState(nil)
}

// Value returns the current value. Alias for Get().
func (m *ManagedState[T]) Value() T {
	return m.value
}
