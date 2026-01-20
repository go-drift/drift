package core

import "sync"

// Listenable is an interface for types that can be listened to.
// AddListener should return an unsubscribe function.
type Listenable interface {
	AddListener(listener func()) func()
}

// Disposable is an interface for types that need cleanup.
type Disposable interface {
	Dispose()
}

// ControllerBase provides common functionality for controllers.
// Embed this struct in your controllers to get listener management for free.
//
// Example:
//
//	type MyController struct {
//	    core.ControllerBase
//	    value int
//	}
//
//	func (c *MyController) SetValue(v int) {
//	    c.value = v
//	    c.NotifyListeners()
//	}
type ControllerBase struct {
	listeners map[int]func()
	nextID    int
	disposed  bool
	mu        sync.RWMutex
}

// AddListener adds a callback that fires when NotifyListeners() is called.
// Returns an unsubscribe function.
func (c *ControllerBase) AddListener(fn func()) func() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.disposed {
		return func() {}
	}

	if c.listeners == nil {
		c.listeners = make(map[int]func())
	}

	id := c.nextID
	c.nextID++
	c.listeners[id] = fn

	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.listeners, id)
	}
}

// NotifyListeners calls all registered listeners.
// Safe to call after disposal (becomes a no-op).
func (c *ControllerBase) NotifyListeners() {
	c.mu.RLock()
	if c.disposed || c.listeners == nil {
		c.mu.RUnlock()
		return
	}
	// Copy listeners to avoid holding lock during callbacks
	listeners := make([]func(), 0, len(c.listeners))
	for _, fn := range c.listeners {
		listeners = append(listeners, fn)
	}
	c.mu.RUnlock()

	for _, fn := range listeners {
		fn()
	}
}

// Dispose clears all listeners and marks the controller as disposed.
// Override this method if you need custom cleanup, but always call
// c.ControllerBase.Dispose() in your override.
func (c *ControllerBase) Dispose() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disposed = true
	c.listeners = nil
}

// IsDisposed returns true if this controller has been disposed.
func (c *ControllerBase) IsDisposed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.disposed
}

// ListenerCount returns the number of registered listeners.
func (c *ControllerBase) ListenerCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.listeners)
}
