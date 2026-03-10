package core

import "sync"

// Signal holds a value and notifies listeners when it changes.
// It is safe for concurrent use.
//
// Signal satisfies [Listenable] via [Signal.AddListener].
//
// Example:
//
//	count := core.NewSignal(0)
//	unsub := count.AddListener(func() {
//	    fmt.Println("Count changed to:", count.Value())
//	})
//	count.Set(5) // prints: Count changed to: 5
//	unsub()      // stop listening
type Signal[T any] struct {
	value        T
	listeners    map[int]func()
	nextID       int
	disposed     bool
	mu           sync.RWMutex
	equalityFunc func(a, b T) bool
}

// NewSignal creates a new signal with the given initial value.
// [Signal.Set] skips notification when the new value equals the old via ==.
// For non-comparable types (slices, maps), use [NewSignalWithEquality] instead;
// the comparable constraint here ensures a compile-time error rather than a
// runtime panic.
func NewSignal[T comparable](initial T) *Signal[T] {
	return &Signal[T]{
		value:     initial,
		listeners: make(map[int]func()),
	}
}

// NewSignalWithEquality creates a new signal with a custom equality function.
// Use this for non-comparable types (slices, maps) or when the default
// interface comparison is not appropriate (e.g. comparing only specific fields).
func NewSignalWithEquality[T any](initial T, equalityFunc func(a, b T) bool) *Signal[T] {
	return &Signal[T]{
		value:        initial,
		listeners:    make(map[int]func()),
		equalityFunc: equalityFunc,
	}
}

// Value returns the current value.
func (s *Signal[T]) Value() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

// Set updates the value and notifies all listeners. Notification is skipped
// when the old and new values are equal. By default, values are compared with
// interface comparison (any(old) == any(new)), which works for all comparable
// types. For non-comparable types (slices, maps), provide a custom equality
// function via [NewSignalWithEquality] to avoid a runtime panic.
func (s *Signal[T]) Set(value T) {
	s.mu.Lock()
	if s.disposed {
		s.mu.Unlock()
		return
	}
	if s.equalityFunc != nil {
		if s.equalityFunc(s.value, value) {
			s.mu.Unlock()
			return
		}
	} else if any(s.value) == any(value) {
		s.mu.Unlock()
		return
	}
	s.value = value
	// Copy listeners to avoid holding lock during callbacks
	listeners := make([]func(), 0, len(s.listeners))
	for _, fn := range s.listeners {
		listeners = append(listeners, fn)
	}
	s.mu.Unlock()

	for _, fn := range listeners {
		fn()
	}
}

// Update applies a transformation to the current value.
// This is useful for complex updates that depend on the current value.
// Like [Signal.Set], notification is skipped when the value does not change.
func (s *Signal[T]) Update(transform func(T) T) {
	s.mu.Lock()
	if s.disposed {
		s.mu.Unlock()
		return
	}
	newValue := transform(s.value)
	if s.equalityFunc != nil {
		if s.equalityFunc(s.value, newValue) {
			s.mu.Unlock()
			return
		}
	} else if any(s.value) == any(newValue) {
		s.mu.Unlock()
		return
	}
	s.value = newValue
	// Copy listeners to avoid holding lock during callbacks
	listeners := make([]func(), 0, len(s.listeners))
	for _, fn := range s.listeners {
		listeners = append(listeners, fn)
	}
	s.mu.Unlock()

	for _, fn := range listeners {
		fn()
	}
}

// AddListener adds a callback that fires when the value changes. Use
// [Signal.Value] to read the new value inside the callback. This method
// satisfies the [Listenable] interface. Returns an unsubscribe function.
func (s *Signal[T]) AddListener(fn func()) func() {
	s.mu.Lock()
	if s.disposed {
		s.mu.Unlock()
		return func() {}
	}
	id := s.nextID
	s.nextID++
	s.listeners[id] = fn
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		delete(s.listeners, id)
		s.mu.Unlock()
	}
}

// ListenerCount returns the number of registered listeners.
func (s *Signal[T]) ListenerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.listeners)
}

// Dispose clears all listeners and marks the signal as disposed.
// After disposal, [Signal.Set], [Signal.Update], and [Signal.AddListener]
// become no-ops. [Signal.Value] continues to return the last set value.
// Dispose is safe to call multiple times.
func (s *Signal[T]) Dispose() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disposed = true
	s.listeners = nil
}

// IsDisposed returns true if this signal has been disposed.
func (s *Signal[T]) IsDisposed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.disposed
}
