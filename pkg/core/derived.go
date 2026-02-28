package core

import "sync"

// Subscribable is implemented by types that support untyped change
// notification. Both [Observable] and [DerivedObservable] satisfy this
// interface.
//
// Subscribable is the dependency type accepted by [Derive] and [UseDerived].
// The OnChange callback fires after the value has changed; the callback
// receives no arguments (use the source's Value method to read the new value).
// The returned function unsubscribes the callback.
type Subscribable interface {
	OnChange(fn func()) func()
}

// DerivedObservable is a read-only observable that recomputes its value from
// source observables automatically. When any dependency fires its OnChange
// callback, the compute function is re-evaluated. If the new value differs
// from the previous one (checked via equality), all listeners are notified.
//
// DerivedObservable is safe for concurrent use. Reads use a shared lock and
// writes (recomputation) use an exclusive lock, following the same RWMutex
// pattern as [Observable].
//
// DerivedObservable itself satisfies [Subscribable], so derived values can be
// chained: one DerivedObservable can serve as a dependency of another.
//
// # Equality
//
// By default, the old and new values are compared with interface comparison
// (any(old) != any(new)). This works for all comparable types (int, string,
// structs with comparable fields, etc.). For non-comparable types such as
// slices or maps, provide a custom equality function via [DeriveWithEquality]
// to avoid a runtime panic.
//
// # Lifecycle
//
// A DerivedObservable subscribes to its dependencies on creation. Call
// [DerivedObservable.Dispose] to unsubscribe from all dependencies and release
// resources. Inside a stateful widget, use [UseDerived] to create, subscribe,
// and auto-dispose a DerivedObservable in one step.
//
// # Example
//
//	firstName := core.NewObservable("John")
//	lastName := core.NewObservable("Doe")
//	fullName := core.Derive(func() string {
//	    return firstName.Value() + " " + lastName.Value()
//	}, firstName, lastName)
//
//	fmt.Println(fullName.Value()) // "John Doe"
//
//	lastName.Set("Smith")
//	fmt.Println(fullName.Value()) // "John Smith"
type DerivedObservable[T any] struct {
	compute      func() T
	value        T
	listeners    map[int]func(T)
	nextID       int
	mu           sync.RWMutex
	unsubs       []func()
	disposed     bool
	equalityFunc func(a, b T) bool
}

// Derive creates a [DerivedObservable] that recomputes when any dep changes.
// The compute function is called immediately to establish the initial value,
// then called again each time a dependency notifies.
//
// The default equality check uses interface comparison (any(old) != any(new)),
// which works for all comparable types. For non-comparable types (slices,
// maps, structs containing slices), use [DeriveWithEquality] instead.
func Derive[T any](compute func() T, deps ...Subscribable) *DerivedObservable[T] {
	return DeriveWithEquality(compute, nil, deps...)
}

// DeriveWithEquality creates a [DerivedObservable] with a custom equality
// function for comparing old and new computed values. If equal is nil, the
// default interface comparison is used.
//
// Use this when T is non-comparable (slices, maps) or when you need semantic
// equality that differs from Go's == operator:
//
//	items := core.DeriveWithEquality(
//	    func() []string { return buildList(source.Value()) },
//	    slices.Equal,
//	    source,
//	)
func DeriveWithEquality[T any](compute func() T, equal func(a, b T) bool, deps ...Subscribable) *DerivedObservable[T] {
	d := &DerivedObservable[T]{
		compute:      compute,
		value:        compute(),
		listeners:    make(map[int]func(T)),
		equalityFunc: equal,
		unsubs:       make([]func(), 0, len(deps)),
	}
	for _, dep := range deps {
		unsub := dep.OnChange(d.recompute)
		d.unsubs = append(d.unsubs, unsub)
	}
	return d
}

// Value returns the current derived value. Safe for concurrent use.
func (d *DerivedObservable[T]) Value() T {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.value
}

// AddListener adds a typed callback that fires when the derived value changes.
// The callback receives the new value. Returns an unsubscribe function that
// removes the listener. Safe for concurrent use.
func (d *DerivedObservable[T]) AddListener(fn func(T)) func() {
	d.mu.Lock()
	id := d.nextID
	d.nextID++
	d.listeners[id] = fn
	d.mu.Unlock()

	return func() {
		d.mu.Lock()
		delete(d.listeners, id)
		d.mu.Unlock()
	}
}

// OnChange adds an untyped callback that fires when the derived value changes.
// This satisfies the [Subscribable] interface, enabling chained derivations
// where one DerivedObservable is a dependency of another. Returns an
// unsubscribe function.
func (d *DerivedObservable[T]) OnChange(fn func()) func() {
	return d.AddListener(func(T) { fn() })
}

// Dispose unsubscribes from all dependencies and clears listeners. After
// disposal, the DerivedObservable will no longer recompute and any
// outstanding listener references become no-ops. Dispose is safe to call
// multiple times.
func (d *DerivedObservable[T]) Dispose() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.disposed = true
	for _, unsub := range d.unsubs {
		unsub()
	}
	d.unsubs = nil
	d.listeners = nil
}

// ListenerCount returns the number of registered listeners.
func (d *DerivedObservable[T]) ListenerCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.listeners)
}

// recompute re-evaluates the compute function and notifies listeners if changed.
// The compute function is called outside the lock to avoid deadlocks when
// chained DerivedObservables read each other's values under concurrent mutation.
func (d *DerivedObservable[T]) recompute() {
	// Check disposed before computing (quick path, avoids unnecessary work).
	d.mu.RLock()
	disposed := d.disposed
	d.mu.RUnlock()
	if disposed {
		return
	}

	// Compute new value without holding the lock. The compute function
	// typically calls .Value() on source observables, which acquire their
	// own read locks. Running this outside d.mu prevents lock-ordering
	// deadlocks with chained derivations.
	newValue := d.compute()

	d.mu.Lock()
	if d.disposed {
		d.mu.Unlock()
		return
	}

	old := d.value

	if d.equalityFunc != nil {
		if d.equalityFunc(old, newValue) {
			d.mu.Unlock()
			return
		}
	} else {
		if any(old) == any(newValue) {
			d.mu.Unlock()
			return
		}
	}

	d.value = newValue

	listeners := make([]func(T), 0, len(d.listeners))
	for _, fn := range d.listeners {
		listeners = append(listeners, fn)
	}
	d.mu.Unlock()

	for _, fn := range listeners {
		fn(newValue)
	}
}
