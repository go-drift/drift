package core

import "sync"

// Observable holds a value and notifies listeners when it changes.
// It is safe for concurrent use.
//
// Example:
//
//	count := core.NewObservable(0)
//	unsub := count.AddListener(func(value int) {
//	    fmt.Println("Count changed to:", value)
//	})
//	count.Set(5) // prints: Count changed to: 5
//	unsub()      // stop listening
type Observable[T any] struct {
	value        T
	listeners    map[int]func(T)
	nextID       int
	mu           sync.RWMutex
	equalityFunc func(a, b T) bool
}

// NewObservable creates a new observable with the given initial value.
func NewObservable[T any](initial T) *Observable[T] {
	return &Observable[T]{
		value:     initial,
		listeners: make(map[int]func(T)),
	}
}

// NewObservableWithEquality creates a new observable with a custom equality function.
// The equality function is used to determine if the value has changed.
func NewObservableWithEquality[T any](initial T, equalityFunc func(a, b T) bool) *Observable[T] {
	return &Observable[T]{
		value:        initial,
		listeners:    make(map[int]func(T)),
		equalityFunc: equalityFunc,
	}
}

// Value returns the current value.
func (o *Observable[T]) Value() T {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.value
}

// Set updates the value and notifies all listeners if the value changed.
// If a custom equality function was provided, it is used to determine if the value changed.
func (o *Observable[T]) Set(value T) {
	o.mu.Lock()
	if o.equalityFunc != nil && o.equalityFunc(o.value, value) {
		o.mu.Unlock()
		return
	}
	o.value = value
	// Copy listeners to avoid holding lock during callbacks
	listeners := make([]func(T), 0, len(o.listeners))
	for _, fn := range o.listeners {
		listeners = append(listeners, fn)
	}
	o.mu.Unlock()

	for _, fn := range listeners {
		fn(value)
	}
}

// Update applies a transformation to the current value.
// This is useful for complex updates that depend on the current value.
func (o *Observable[T]) Update(transform func(T) T) {
	o.mu.Lock()
	newValue := transform(o.value)
	if o.equalityFunc != nil && o.equalityFunc(o.value, newValue) {
		o.mu.Unlock()
		return
	}
	o.value = newValue
	// Copy listeners to avoid holding lock during callbacks
	listeners := make([]func(T), 0, len(o.listeners))
	for _, fn := range o.listeners {
		listeners = append(listeners, fn)
	}
	o.mu.Unlock()

	for _, fn := range listeners {
		fn(newValue)
	}
}

// AddListener adds a callback that fires whenever the value changes.
// Returns an unsubscribe function.
func (o *Observable[T]) AddListener(fn func(T)) func() {
	o.mu.Lock()
	id := o.nextID
	o.nextID++
	o.listeners[id] = fn
	o.mu.Unlock()

	return func() {
		o.mu.Lock()
		delete(o.listeners, id)
		o.mu.Unlock()
	}
}

// ListenerCount returns the number of registered listeners.
func (o *Observable[T]) ListenerCount() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.listeners)
}

// Notifier triggers callbacks when Notify() is called.
// Unlike Observable, it doesn't hold a value.
// It is safe for concurrent use.
//
// Notifier implements the Listenable interface, so it can be used with UseListenable.
//
// Example:
//
//	refresh := core.NewNotifier()
//	unsub := refresh.AddListener(func() {
//	    fmt.Println("Refresh triggered!")
//	})
//	refresh.Notify() // prints: Refresh triggered!
//	unsub()          // stop listening
type Notifier struct {
	listeners map[int]func()
	nextID    int
	mu        sync.RWMutex
}

// NewNotifier creates a new notifier.
func NewNotifier() *Notifier {
	return &Notifier{
		listeners: make(map[int]func()),
	}
}

// Notify triggers all registered listeners.
func (n *Notifier) Notify() {
	n.mu.RLock()
	// Copy listeners to avoid holding lock during callbacks
	listeners := make([]func(), 0, len(n.listeners))
	for _, fn := range n.listeners {
		listeners = append(listeners, fn)
	}
	n.mu.RUnlock()

	for _, fn := range listeners {
		fn()
	}
}

// AddListener adds a callback that fires when Notify() is called.
// Returns an unsubscribe function.
func (n *Notifier) AddListener(fn func()) func() {
	n.mu.Lock()
	id := n.nextID
	n.nextID++
	n.listeners[id] = fn
	n.mu.Unlock()

	return func() {
		n.mu.Lock()
		delete(n.listeners, id)
		n.mu.Unlock()
	}
}

// ListenerCount returns the number of registered listeners.
func (n *Notifier) ListenerCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.listeners)
}
