package core

import "sync"

// Notifier provides listener management and disposal for custom state holders.
// Embed this struct to get reactive notification for free.
//
// Example:
//
//	type AuthNotifier struct {
//	    core.Notifier
//	    isLoggedIn bool
//	}
//
//	func (n *AuthNotifier) SetLoggedIn(v bool) {
//	    n.isLoggedIn = v
//	    n.Notify()
//	}
type Notifier struct {
	listeners map[int]func()
	nextID    int
	disposed  bool
	mu        sync.RWMutex
}

// AddListener adds a callback that fires when Notify() is called.
// Returns an unsubscribe function.
func (c *Notifier) AddListener(fn func()) func() {
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

// Notify calls all registered listeners.
// Safe to call after disposal (becomes a no-op).
func (c *Notifier) Notify() {
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

// Dispose clears all listeners and marks the notifier as disposed.
// Override this method if you need custom cleanup, but always call
// c.Notifier.Dispose() in your override.
func (c *Notifier) Dispose() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disposed = true
	c.listeners = nil
}

// IsDisposed returns true if this notifier has been disposed.
func (c *Notifier) IsDisposed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.disposed
}

// ListenerCount returns the number of registered listeners.
func (c *Notifier) ListenerCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.listeners)
}
