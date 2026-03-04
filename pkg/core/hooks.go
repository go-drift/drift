package core

// UseController creates a controller and registers it for automatic disposal.
// The controller will be disposed when the state is disposed.
//
// Example:
//
//	func (s *myState) InitState() {
//	    s.animation = core.UseController(s, func() *animation.AnimationController {
//	        return animation.NewAnimationController(300 * time.Millisecond)
//	    })
//	}
func UseController[C Disposable](s stateBase, create func() C) C {
	base := s.state()
	controller := create()
	base.OnDispose(func() {
		controller.Dispose()
	})
	return controller
}

// UseSubscription registers a subscribe/unsubscribe pair with automatic cleanup.
// The subscribe function is called immediately and must return an unsubscribe
// function. That unsubscribe function runs automatically when the state is
// disposed (in LIFO order along with other disposers).
//
// Call this once in InitState, not in Build.
//
// UseSubscription is a general-purpose building block for any service that
// follows the "subscribe returns unsubscribe" pattern:
//
//	func (s *myState) InitState() {
//	    core.UseSubscription(s, func() func() {
//	        return eventBus.Subscribe("user.updated", func(e Event) {
//	            s.SetState(func() { s.user = e.Payload.(User) })
//	        })
//	    })
//	}
func UseSubscription(s stateBase, subscribe func() func()) {
	base := s.state()
	unsub := subscribe()
	base.OnDispose(unsub)
}

// UseListenable subscribes to a listenable and triggers rebuilds.
// The subscription is automatically cleaned up when the state is disposed.
//
// Example:
//
//	func (s *myState) InitState() {
//	    s.controller = core.UseController(s, func() *MyController {
//	        return NewMyController()
//	    })
//	    core.UseListenable(s, s.controller)
//	}
func UseListenable(s stateBase, listenable Listenable) {
	base := s.state()
	unsub := listenable.AddListener(func() {
		base.SetState(nil)
	})
	base.OnDispose(unsub)
}

// ObservableValue is the common interface for readable, listenable value
// holders. Both [*Observable] and [*DerivedObservable] satisfy it.
//
// Hooks that accept ObservableValue ([UseObservable], [UseObservableSelector])
// work uniformly with either type without requiring the caller to distinguish
// between them.
type ObservableValue[T any] interface {
	// Value returns the current value.
	Value() T
	// AddListener registers a typed callback and returns an unsubscribe function.
	AddListener(func(T)) func()
}

// UseObservable subscribes to an observable and triggers rebuilds when it changes.
// Call this once in InitState(), not in Build(). The subscription is automatically
// cleaned up when the state is disposed.
//
// Works with both *Observable[T] and *DerivedObservable[T].
//
// Example:
//
//	func (s *myState) InitState() {
//	    s.counter = core.NewObservable(0)
//	    core.UseObservable(s, s.counter)
//	}
//
//	func (s *myState) Build(ctx core.BuildContext) core.Widget {
//	    // Use .Value() in Build to read the current value
//	    return widgets.Text{Content: fmt.Sprintf("Count: %d", s.counter.Value()), ...}
//	}
func UseObservable[T any](s stateBase, obs ObservableValue[T]) {
	base := s.state()
	unsub := obs.AddListener(func(T) {
		base.SetState(nil)
	})
	base.OnDispose(unsub)
}

// UseDerived creates a [DerivedObservable], subscribes to it for rebuilds,
// and auto-disposes it when the state is disposed. This is a convenience that
// combines [Derive] + [UseObservable] + OnDispose(d.Dispose) in one call.
//
// Call this once in InitState, not in Build.
//
// Example:
//
//	type myState struct {
//	    core.StateBase
//	    firstName *core.Observable[string]
//	    lastName  *core.Observable[string]
//	    fullName  *core.DerivedObservable[string]
//	}
//
//	func (s *myState) InitState() {
//	    s.firstName = core.NewObservable("John")
//	    s.lastName = core.NewObservable("Doe")
//	    s.fullName = core.UseDerived(s, func() string {
//	        return s.firstName.Value() + " " + s.lastName.Value()
//	    }, s.firstName, s.lastName)
//	}
//
//	func (s *myState) Build(ctx core.BuildContext) core.Widget {
//	    return widgets.Text{Content: s.fullName.Value()}
//	}
func UseDerived[T any](s stateBase, compute func() T, deps ...Subscribable) *DerivedObservable[T] {
	d := Derive(compute, deps...)
	UseObservable(s, d)
	s.state().OnDispose(d.Dispose)
	return d
}

// UseObservableSelector subscribes to an observable but only triggers rebuilds
// when a selected slice of state changes. The selector function extracts the
// relevant portion from the full observable value; the widget only rebuilds
// when the selector's return value differs from the previous one.
//
// This is useful when an observable holds a large struct but the widget only
// depends on one field. Without a selector, every field change would trigger a
// rebuild.
//
// Call this once in InitState, not in Build. The subscription is automatically
// cleaned up when the state is disposed.
//
// Uses interface comparison for equality (works for comparable types). For
// non-comparable selected types (slices, maps), use
// [UseObservableSelectorWithEquality].
//
// Example:
//
//	func (s *myState) InitState() {
//	    // Only rebuild when the user's name changes, not on every field change.
//	    core.UseObservableSelector(s, s.user, func(u User) string {
//	        return u.Name
//	    })
//	}
func UseObservableSelector[T, S any](s stateBase, obs ObservableValue[T], selector func(T) S) {
	UseObservableSelectorWithEquality(s, obs, selector, nil)
}

// UseObservableSelectorWithEquality is like [UseObservableSelector] but with a
// custom equality function for the selected value. If equal is nil, the
// default interface comparison is used.
//
// Like other hooks, the listener callback calls SetState, which is not
// thread-safe. If the source observable is mutated from background goroutines,
// wrap the Set call with drift.Dispatch to ensure notifications arrive on the
// UI thread.
//
// Use this when the selected type is non-comparable or when you need semantic
// equality that differs from Go's == operator:
//
//	core.UseObservableSelectorWithEquality(s, s.store, func(st Store) []string {
//	    return st.Tags
//	}, slices.Equal)
func UseObservableSelectorWithEquality[T, S any](
	st stateBase,
	obs ObservableValue[T],
	selector func(T) S,
	equal func(a, b S) bool,
) {
	base := st.state()
	lastSelected := selector(obs.Value())

	unsub := obs.AddListener(func(newVal T) {
		selected := selector(newVal)
		changed := false
		if equal != nil {
			changed = !equal(lastSelected, selected)
		} else {
			changed = any(lastSelected) != any(selected)
		}
		if changed {
			lastSelected = selected
			base.SetState(nil)
		}
	})
	base.OnDispose(unsub)
}

// Managed holds a value and triggers rebuilds when it changes.
// Unlike Observable, it is tied to a specific StateBase.
//
// Managed is NOT thread-safe. It must only be accessed from the UI thread.
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
//	    count *core.Managed[int]
//	}
//
//	func (s *myState) InitState() {
//	    s.count = core.NewManaged(s, 0)
//	}
//
//	func (s *myState) Build(ctx core.BuildContext) core.Widget {
//	    return widgets.GestureDetector{
//	        OnTap: func() { s.count.Set(s.count.Value() + 1) },
//	        Child: widgets.Text{Content: fmt.Sprintf("Count: %d", s.count.Value()), ...},
//	    }
//	}
type Managed[T any] struct {
	base  *StateBase
	value T
}

// NewManaged creates a new managed state value.
// Changes to this value will automatically trigger a rebuild.
func NewManaged[T any](s stateBase, initial T) *Managed[T] {
	return &Managed[T]{
		base:  s.state(),
		value: initial,
	}
}

// Value returns the current value.
func (m *Managed[T]) Value() T {
	return m.value
}

// Set updates the value and triggers a rebuild.
func (m *Managed[T]) Set(value T) {
	m.value = value
	m.base.SetState(nil)
}

// Update applies a transformation to the current value and triggers a rebuild.
func (m *Managed[T]) Update(transform func(T) T) {
	m.value = transform(m.value)
	m.base.SetState(nil)
}
