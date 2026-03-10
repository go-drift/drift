package core

import (
	"sync"
	"testing"
)

func TestNotifier_AddListener(t *testing.T) {
	c := &Notifier{}

	called := false
	c.AddListener(func() {
		called = true
	})

	c.Notify()

	if !called {
		t.Error("Listener should be called on Notify")
	}
}

func TestNotifier_AddListener_Unsubscribe(t *testing.T) {
	c := &Notifier{}

	callCount := 0
	unsub := c.AddListener(func() {
		callCount++
	})

	c.Notify()
	unsub()
	c.Notify()

	if callCount != 1 {
		t.Errorf("Listener should only be called once before unsubscribe, called %d times", callCount)
	}
}

func TestNotifier_MultipleListeners(t *testing.T) {
	c := &Notifier{}

	var count1, count2 int
	c.AddListener(func() { count1++ })
	c.AddListener(func() { count2++ })

	c.Notify()
	c.Notify()

	if count1 != 2 || count2 != 2 {
		t.Errorf("All listeners should be called twice, got %d and %d", count1, count2)
	}
}

func TestNotifier_Dispose(t *testing.T) {
	c := &Notifier{}

	callCount := 0
	c.AddListener(func() {
		callCount++
	})

	c.Notify()
	c.Dispose()
	c.Notify()

	if callCount != 1 {
		t.Errorf("Listener should not be called after Dispose, called %d times", callCount)
	}
}

func TestNotifier_AddListener_AfterDispose(t *testing.T) {
	c := &Notifier{}
	c.Dispose()

	called := false
	c.AddListener(func() {
		called = true
	})

	c.Notify()

	if called {
		t.Error("Listener added after Dispose should not be called")
	}
}

func TestNotifier_IsDisposed(t *testing.T) {
	c := &Notifier{}

	if c.IsDisposed() {
		t.Error("Should not be disposed initially")
	}

	c.Dispose()

	if !c.IsDisposed() {
		t.Error("Should be disposed after Dispose()")
	}
}

func TestNotifier_ListenerCount(t *testing.T) {
	c := &Notifier{}

	if c.ListenerCount() != 0 {
		t.Errorf("Expected 0 listeners, got %d", c.ListenerCount())
	}

	unsub1 := c.AddListener(func() {})
	unsub2 := c.AddListener(func() {})

	if c.ListenerCount() != 2 {
		t.Errorf("Expected 2 listeners, got %d", c.ListenerCount())
	}

	unsub1()

	if c.ListenerCount() != 1 {
		t.Errorf("Expected 1 listener, got %d", c.ListenerCount())
	}

	unsub2()

	if c.ListenerCount() != 0 {
		t.Errorf("Expected 0 listeners, got %d", c.ListenerCount())
	}
}

func TestNotifier_ConcurrentAccess(t *testing.T) {
	c := &Notifier{}

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			unsub := c.AddListener(func() {})
			unsub()
		}()
		go func() {
			defer wg.Done()
			c.Notify()
		}()
	}
	wg.Wait()
}

// Test that Notifier implements Listenable and Disposable
func TestNotifier_ImplementsListenable(t *testing.T) {
	var _ Listenable = &Notifier{}
}

func TestNotifier_ImplementsDisposable(t *testing.T) {
	var _ Disposable = &Notifier{}
}
