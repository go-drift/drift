package core

import (
	"sync"
	"testing"
)

func TestSignal_Value(t *testing.T) {
	sig := NewSignal(42)

	if sig.Value() != 42 {
		t.Errorf("Expected 42, got %d", sig.Value())
	}
}

func TestSignal_Set(t *testing.T) {
	sig := NewSignal(0)

	sig.Set(100)

	if sig.Value() != 100 {
		t.Errorf("Expected 100, got %d", sig.Value())
	}
}

func TestSignal_AddListener(t *testing.T) {
	sig := NewSignal(0)

	var received int
	sig.AddListener(func() {
		received = sig.Value()
	})

	sig.Set(42)

	if received != 42 {
		t.Errorf("Listener should receive new value 42, got %d", received)
	}
}

func TestSignal_AddListener_Unsubscribe(t *testing.T) {
	sig := NewSignal(0)

	callCount := 0
	unsub := sig.AddListener(func() {
		callCount++
	})

	sig.Set(1)
	unsub()
	sig.Set(2)

	if callCount != 1 {
		t.Errorf("Listener should only be called once before unsubscribe, called %d times", callCount)
	}
}

func TestSignal_MultipleListeners(t *testing.T) {
	sig := NewSignal(0)

	var received1, received2 int
	sig.AddListener(func() { received1 = sig.Value() })
	sig.AddListener(func() { received2 = sig.Value() })

	sig.Set(99)

	if received1 != 99 || received2 != 99 {
		t.Errorf("All listeners should receive the value, got %d and %d", received1, received2)
	}
}

func TestSignal_Update(t *testing.T) {
	sig := NewSignal(10)

	sig.Update(func(v int) int { return v * 2 })

	if sig.Value() != 20 {
		t.Errorf("Expected 20 after update, got %d", sig.Value())
	}
}

func TestSignal_Update_NotifiesListeners(t *testing.T) {
	sig := NewSignal(5)

	var received int
	sig.AddListener(func() { received = sig.Value() })

	sig.Update(func(v int) int { return v + 10 })

	if received != 15 {
		t.Errorf("Listener should receive updated value 15, got %d", received)
	}
}

func TestSignal_SkipsEqualValues(t *testing.T) {
	sig := NewSignal(42)

	callCount := 0
	sig.AddListener(func() { callCount++ })

	sig.Set(42) // Same value - should skip
	sig.Set(42) // Same value - should skip
	sig.Set(43) // Different value - should notify

	if callCount != 1 {
		t.Errorf("Listener should only be called once for changed value, called %d times", callCount)
	}
}

func TestSignal_UpdateSkipsEqualValues(t *testing.T) {
	sig := NewSignal(10)

	callCount := 0
	sig.AddListener(func() { callCount++ })

	sig.Update(func(v int) int { return v })     // Same value - should skip
	sig.Update(func(v int) int { return v * 2 }) // Different value - should notify

	if callCount != 1 {
		t.Errorf("Listener should only be called once for changed value, called %d times", callCount)
	}
}

func TestSignal_WithCustomEquality(t *testing.T) {
	type user struct {
		ID   int
		Name string
	}

	sig := NewSignalWithEquality(user{ID: 1, Name: "Alice"}, func(a, b user) bool {
		return a.ID == b.ID // Only compare by ID
	})

	callCount := 0
	sig.AddListener(func() { callCount++ })

	sig.Set(user{ID: 1, Name: "Alice Updated"}) // Same ID - should skip
	sig.Set(user{ID: 2, Name: "Bob"})           // Different ID - should notify

	if callCount != 1 {
		t.Errorf("Listener should only be called once for changed ID, called %d times", callCount)
	}
}

func TestSignal_Dispose(t *testing.T) {
	sig := NewSignal(42)

	callCount := 0
	sig.AddListener(func() { callCount++ })

	sig.Dispose()

	// Set after dispose is a no-op
	sig.Set(100)
	if callCount != 0 {
		t.Error("listener should not fire after dispose")
	}

	// Value still returns last value
	if sig.Value() != 42 {
		t.Errorf("expected value 42 after dispose, got %d", sig.Value())
	}

	// AddListener after dispose returns no-op unsub
	unsub := sig.AddListener(func() { callCount++ })
	unsub() // should not panic

	if sig.ListenerCount() != 0 {
		t.Errorf("expected 0 listeners after dispose, got %d", sig.ListenerCount())
	}

	if !sig.IsDisposed() {
		t.Error("expected IsDisposed to return true")
	}

	// Double dispose should not panic
	sig.Dispose()
}

func TestSignal_UpdateAfterDispose(t *testing.T) {
	sig := NewSignal(10)
	sig.Dispose()

	sig.Update(func(v int) int { return v * 2 })
	if sig.Value() != 10 {
		t.Errorf("expected value unchanged after disposed Update, got %d", sig.Value())
	}
}

func TestSignal_ConcurrentAccess(t *testing.T) {
	sig := NewSignal(0)

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(2)
		go func(v int) {
			defer wg.Done()
			sig.Set(v)
		}(i)
		go func() {
			defer wg.Done()
			_ = sig.Value()
		}()
	}
	wg.Wait()
}
