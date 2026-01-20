package core

import (
	"sync"
	"testing"
)

func TestControllerBase_AddListener(t *testing.T) {
	c := &ControllerBase{}

	called := false
	c.AddListener(func() {
		called = true
	})

	c.NotifyListeners()

	if !called {
		t.Error("Listener should be called on NotifyListeners")
	}
}

func TestControllerBase_AddListener_Unsubscribe(t *testing.T) {
	c := &ControllerBase{}

	callCount := 0
	unsub := c.AddListener(func() {
		callCount++
	})

	c.NotifyListeners()
	unsub()
	c.NotifyListeners()

	if callCount != 1 {
		t.Errorf("Listener should only be called once before unsubscribe, called %d times", callCount)
	}
}

func TestControllerBase_MultipleListeners(t *testing.T) {
	c := &ControllerBase{}

	var count1, count2 int
	c.AddListener(func() { count1++ })
	c.AddListener(func() { count2++ })

	c.NotifyListeners()

	if count1 != 1 || count2 != 1 {
		t.Errorf("All listeners should be called, got %d and %d", count1, count2)
	}
}

func TestControllerBase_Dispose(t *testing.T) {
	c := &ControllerBase{}

	callCount := 0
	c.AddListener(func() {
		callCount++
	})

	c.NotifyListeners()
	c.Dispose()
	c.NotifyListeners()

	if callCount != 1 {
		t.Errorf("Listener should not be called after Dispose, called %d times", callCount)
	}
}

func TestControllerBase_AddListener_AfterDispose(t *testing.T) {
	c := &ControllerBase{}
	c.Dispose()

	called := false
	c.AddListener(func() {
		called = true
	})

	c.NotifyListeners()

	if called {
		t.Error("Listener added after Dispose should not be called")
	}
}

func TestControllerBase_IsDisposed(t *testing.T) {
	c := &ControllerBase{}

	if c.IsDisposed() {
		t.Error("Should not be disposed initially")
	}

	c.Dispose()

	if !c.IsDisposed() {
		t.Error("Should be disposed after Dispose()")
	}
}

func TestControllerBase_ListenerCount(t *testing.T) {
	c := &ControllerBase{}

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

func TestControllerBase_ConcurrentAccess(t *testing.T) {
	c := &ControllerBase{}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			unsub := c.AddListener(func() {})
			unsub()
		}()
		go func() {
			defer wg.Done()
			c.NotifyListeners()
		}()
	}
	wg.Wait()
}

// Test that ControllerBase implements Listenable
func TestControllerBase_ImplementsListenable(t *testing.T) {
	var _ Listenable = &ControllerBase{}
}

// Test that ControllerBase implements Disposable
func TestControllerBase_ImplementsDisposable(t *testing.T) {
	var _ Disposable = &ControllerBase{}
}
