package core

import (
	"sync"
	"testing"
)

func TestStateBase_SetState(t *testing.T) {
	s := &StateBase{}

	called := false
	s.SetState(func() {
		called = true
	})

	if !called {
		t.Error("SetState should call the provided function")
	}
}

func TestStateBase_SetState_NilFunction(t *testing.T) {
	s := &StateBase{}

	// Should not panic
	s.SetState(nil)
}

func TestStateBase_SetState_AfterDispose(t *testing.T) {
	s := &StateBase{}
	s.Dispose()

	called := false
	s.SetState(func() {
		called = true
	})

	if called {
		t.Error("SetState should not call function after disposal")
	}
}

func TestStateBase_OnDispose(t *testing.T) {
	s := &StateBase{}

	called := false
	s.OnDispose(func() {
		called = true
	})

	s.Dispose()

	if !called {
		t.Error("OnDispose callback should be called on Dispose")
	}
}

func TestStateBase_OnDispose_MultipleCallbacks(t *testing.T) {
	s := &StateBase{}

	order := make([]int, 0, 3)
	s.OnDispose(func() { order = append(order, 1) })
	s.OnDispose(func() { order = append(order, 2) })
	s.OnDispose(func() { order = append(order, 3) })

	s.Dispose()

	// Should be called in reverse order (LIFO)
	if len(order) != 3 {
		t.Fatalf("Expected 3 callbacks, got %d", len(order))
	}
	if order[0] != 3 || order[1] != 2 || order[2] != 1 {
		t.Errorf("Expected LIFO order [3,2,1], got %v", order)
	}
}

func TestStateBase_OnDispose_Unregister(t *testing.T) {
	s := &StateBase{}

	called := false
	unregister := s.OnDispose(func() {
		called = true
	})

	unregister()
	s.Dispose()

	if called {
		t.Error("Unregistered callback should not be called")
	}
}

func TestStateBase_OnDispose_AfterDispose(t *testing.T) {
	s := &StateBase{}
	s.Dispose()

	called := false
	s.OnDispose(func() {
		called = true
	})

	if !called {
		t.Error("OnDispose should immediately call cleanup if already disposed")
	}
}

func TestStateBase_Dispose_OnlyOnce(t *testing.T) {
	s := &StateBase{}

	callCount := 0
	s.OnDispose(func() {
		callCount++
	})

	s.Dispose()
	s.Dispose()
	s.Dispose()

	if callCount != 1 {
		t.Errorf("Disposer should only be called once, called %d times", callCount)
	}
}

func TestStateBase_IsDisposed(t *testing.T) {
	s := &StateBase{}

	if s.IsDisposed() {
		t.Error("Should not be disposed initially")
	}

	s.Dispose()

	if !s.IsDisposed() {
		t.Error("Should be disposed after Dispose()")
	}
}

func TestStateBase_ConcurrentDispose(t *testing.T) {
	s := &StateBase{}

	var callCount int
	var mu sync.Mutex
	s.OnDispose(func() {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Dispose()
		}()
	}
	wg.Wait()

	if callCount != 1 {
		t.Errorf("Disposer should only be called once even with concurrent Dispose calls, called %d times", callCount)
	}
}

func TestStateBase_OnDispose_NilCallback(t *testing.T) {
	s := &StateBase{}

	// Should not panic
	unregister := s.OnDispose(nil)
	unregister()
	s.Dispose()
}
