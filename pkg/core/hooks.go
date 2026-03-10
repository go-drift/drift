package core

// Listenable is an interface for types that support untyped change notification.
// [Signal], [Derived], and [Notifier] all satisfy this interface.
//
// [Listenable] is the dependency type accepted by [NewDerived] and [UseDerived].
// AddListener should return an unsubscribe function.
type Listenable interface {
	AddListener(listener func()) func()
}

// Disposable is an interface for types that need cleanup.
type Disposable interface {
	Dispose()
}

// UseDisposable registers a [Disposable] resource for automatic cleanup when
// the state is disposed. Call this once in InitState, not in Build.
//
// Example:
//
//	func (s *myState) InitState() {
//	    s.animation = animation.NewAnimationController(300 * time.Millisecond)
//	    core.UseDisposable(s, s.animation)
//	}
func UseDisposable(s stateBase, d Disposable) {
	s.state().OnDispose(d.Dispose)
}

// UseListenable subscribes to a [Listenable] and triggers rebuilds.
// The subscription is automatically cleaned up when the state is disposed.
//
// Works with [Signal], [Derived], [Notifier], and any custom type that
// implements [Listenable].
//
// The listener callback calls [StateBase.SetState], which is not thread-safe.
// If the source is mutated from background goroutines, wrap the mutation with
// drift.Dispatch to ensure notifications arrive on the UI thread.
//
// Example:
//
//	func (s *myState) InitState() {
//	    s.counter = core.NewSignal(0)
//	    core.UseListenable(s, s.counter)
//	}
//
//	func (s *myState) Build(ctx core.BuildContext) core.Widget {
//	    return widgets.Text{Content: fmt.Sprintf("Count: %d", s.counter.Value())}
//	}
func UseListenable(s stateBase, listenable Listenable) {
	base := s.state()
	unsub := listenable.AddListener(func() {
		base.SetState(nil)
	})
	base.OnDispose(unsub)
}

// UseDerived creates a [Derived], subscribes to it for rebuilds, and
// auto-disposes it when the state is disposed. This is a convenience that
// combines [NewDerived] + [UseListenable] + OnDispose(d.Dispose) in one call.
//
// Call this once in InitState, not in Build. Like [UseListenable], the
// listener calls [StateBase.SetState], so dependencies must only be mutated
// on the UI thread (use drift.Dispatch from goroutines).
//
// Example:
//
//	type myState struct {
//	    core.StateBase
//	    firstName *core.Signal[string]
//	    lastName  *core.Signal[string]
//	    fullName  *core.Derived[string]
//	}
//
//	func (s *myState) InitState() {
//	    s.firstName = core.NewSignal("John")
//	    s.lastName = core.NewSignal("Doe")
//	    s.fullName = core.UseDerived(s, func() string {
//	        return s.firstName.Value() + " " + s.lastName.Value()
//	    }, s.firstName, s.lastName)
//	}
//
//	func (s *myState) Build(ctx core.BuildContext) core.Widget {
//	    return widgets.Text{Content: s.fullName.Value()}
//	}
func UseDerived[T comparable](s stateBase, compute func() T, deps ...Listenable) *Derived[T] {
	d := NewDerived(compute, deps...)
	UseListenable(s, d)
	s.state().OnDispose(d.Dispose)
	return d
}

// UseDerivedWithEquality is like [UseDerived] but accepts a custom equality
// function for comparing computed values. Use this when the derived type is
// non-comparable (slices, maps):
//
//	func (s *myState) InitState() {
//	    s.tags = core.UseDerivedWithEquality(s, func() []string {
//	        return buildTagList(s.source.Value())
//	    }, slices.Equal, s.source)
//	}
func UseDerivedWithEquality[T any](s stateBase, compute func() T, equal func(a, b T) bool, deps ...Listenable) *Derived[T] {
	d := NewDerivedWithEquality(compute, equal, deps...)
	UseListenable(s, d)
	s.state().OnDispose(d.Dispose)
	return d
}

// UseSelector subscribes to a [Listenable] source but only triggers rebuilds
// when the selector returns a different value. The selector extracts the
// relevant portion of state; the widget only rebuilds when that portion changes.
//
// S must be comparable. For non-comparable selected types (slices, maps), use
// [UseSelectorWithEquality] instead.
//
// This is useful when a signal holds a large struct but the widget only
// depends on one field. Without a selector, every notification would trigger a
// rebuild.
//
// Call this once in InitState, not in Build. The subscription is automatically
// cleaned up when the state is disposed.
//
// Like other hooks, the listener callback calls SetState, which is not
// thread-safe. If the source is mutated from background goroutines,
// wrap the Set call with drift.Dispatch to ensure notifications arrive on the
// UI thread.
//
// Example:
//
//	func (s *myState) InitState() {
//	    // Only rebuilds when user.Name changes, ignoring other field updates
//	    core.UseSelector(s, s.user, func() string {
//	        return s.user.Value().Name
//	    })
//	}
func UseSelector[S comparable](
	st stateBase,
	source Listenable,
	selector func() S,
) {
	UseSelectorWithEquality(st, source, selector, func(a, b S) bool { return a == b })
}

// UseSelectorWithEquality is like [UseSelector] but accepts a custom equality
// function for comparing selected values. Use this when the selected type is
// non-comparable (slices, maps) or when you need semantic equality that
// differs from Go's == operator:
//
//	core.UseSelectorWithEquality(s, s.store, func() []string {
//	    return s.store.Value().Tags
//	}, slices.Equal)
func UseSelectorWithEquality[S any](
	st stateBase,
	source Listenable,
	selector func() S,
	equal func(a, b S) bool,
) {
	base := st.state()
	lastSelected := selector()

	unsub := source.AddListener(func() {
		selected := selector()
		if !equal(lastSelected, selected) {
			lastSelected = selected
			base.SetState(nil)
		}
	})
	base.OnDispose(unsub)
}
