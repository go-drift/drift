package core

import "sync"

// Derived is a read-only reactive value that recomputes from source signals
// automatically. When any dependency fires its [Listenable] callback, the compute
// function is re-evaluated. If the new value differs from the previous one
// (checked via equality), all listeners are notified.
//
// Derived is safe for concurrent use. Reads use a shared lock and writes
// (recomputation) use an exclusive lock, following the same RWMutex pattern
// as [Signal].
//
// Derived satisfies [Listenable] via [Derived.AddListener].
// Derived values can be chained: one Derived can serve as a dependency
// of another.
//
// # Equality
//
// By default, the old and new values are compared with interface comparison
// (any(old) != any(new)). This works for all comparable types (int, string,
// structs with comparable fields, etc.). For non-comparable types such as
// slices or maps, provide a custom equality function via [NewDerivedWithEquality]
// to avoid a runtime panic.
//
// # Lifecycle
//
// A Derived subscribes to its dependencies on creation. Call [Derived.Dispose]
// to unsubscribe from all dependencies and release resources. Inside a
// stateful widget, use [UseDerived] to create, subscribe, and auto-dispose a
// Derived in one step.
//
// # Example
//
//	firstName := core.NewSignal("John")
//	lastName := core.NewSignal("Doe")
//	fullName := core.NewDerived(func() string {
//	    return firstName.Value() + " " + lastName.Value()
//	}, firstName, lastName)
//
//	fmt.Println(fullName.Value()) // "John Doe"
//
//	lastName.Set("Smith")
//	fmt.Println(fullName.Value()) // "John Smith"
type Derived[T any] struct {
	compute      func() T
	value        T
	listeners    map[int]func()
	nextID       int
	mu           sync.RWMutex
	unsubs       []func()
	disposed     bool
	equalityFunc func(a, b T) bool
}

// NewDerived creates a [Derived] that recomputes when any dep changes.
// The compute function is called immediately to establish the initial value,
// then called again each time a dependency notifies.
//
// Values are compared with == to skip redundant notifications. For
// non-comparable types (slices, maps), use [NewDerivedWithEquality] instead;
// the comparable constraint here ensures a compile-time error rather than a
// runtime panic.
func NewDerived[T comparable](compute func() T, deps ...Listenable) *Derived[T] {
	return NewDerivedWithEquality(compute, nil, deps...)
}

// NewDerivedWithEquality creates a [Derived] with a custom equality function for
// comparing old and new computed values. If equal is nil, the default
// interface comparison is used.
//
// Use this when T is non-comparable (slices, maps) or when you need semantic
// equality that differs from Go's == operator:
//
//	items := core.NewDerivedWithEquality(
//	    func() []string { return buildList(source.Value()) },
//	    slices.Equal,
//	    source,
//	)
func NewDerivedWithEquality[T any](compute func() T, equal func(a, b T) bool, deps ...Listenable) *Derived[T] {
	d := &Derived[T]{
		compute:      compute,
		value:        compute(),
		listeners:    make(map[int]func()),
		equalityFunc: equal,
		unsubs:       make([]func(), 0, len(deps)),
	}
	for _, dep := range deps {
		unsub := dep.AddListener(d.recompute)
		d.unsubs = append(d.unsubs, unsub)
	}
	return d
}

// Value returns the current derived value. Safe for concurrent use.
func (d *Derived[T]) Value() T {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.value
}

// AddListener adds a callback that fires when the derived value changes.
// Use [Derived.Value] to read the new value inside the callback. This method
// satisfies the [Listenable] interface, enabling chained derivations where
// one Derived is a dependency of another. Returns an unsubscribe function.
func (d *Derived[T]) AddListener(fn func()) func() {
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

// Dispose unsubscribes from all dependencies and clears listeners. After
// disposal, the Derived will no longer recompute and any outstanding listener
// references become no-ops. Dispose is safe to call multiple times.
func (d *Derived[T]) Dispose() {
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
func (d *Derived[T]) ListenerCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.listeners)
}

// recompute re-evaluates the compute function and notifies listeners if changed.
// The compute function is called outside the lock to avoid deadlocks when
// chained Derived values read each other's values under concurrent mutation.
func (d *Derived[T]) recompute() {
	// Check disposed before computing (quick path, avoids unnecessary work).
	d.mu.RLock()
	disposed := d.disposed
	d.mu.RUnlock()
	if disposed {
		return
	}

	// Compute new value without holding the lock. The compute function
	// typically calls .Value() on source signals, which acquire their
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

	listeners := make([]func(), 0, len(d.listeners))
	for _, fn := range d.listeners {
		listeners = append(listeners, fn)
	}
	d.mu.Unlock()

	for _, fn := range listeners {
		fn()
	}
}
