package core

import (
	"sync"
	"testing"
)

func TestObservable_Value(t *testing.T) {
	obs := NewObservable(42)

	if obs.Value() != 42 {
		t.Errorf("Expected 42, got %d", obs.Value())
	}
}

func TestObservable_Set(t *testing.T) {
	obs := NewObservable(0)

	obs.Set(100)

	if obs.Value() != 100 {
		t.Errorf("Expected 100, got %d", obs.Value())
	}
}

func TestObservable_AddListener(t *testing.T) {
	obs := NewObservable(0)

	var received int
	obs.AddListener(func(value int) {
		received = value
	})

	obs.Set(42)

	if received != 42 {
		t.Errorf("Listener should receive new value 42, got %d", received)
	}
}

func TestObservable_AddListener_Unsubscribe(t *testing.T) {
	obs := NewObservable(0)

	callCount := 0
	unsub := obs.AddListener(func(value int) {
		callCount++
	})

	obs.Set(1)
	unsub()
	obs.Set(2)

	if callCount != 1 {
		t.Errorf("Listener should only be called once before unsubscribe, called %d times", callCount)
	}
}

func TestObservable_MultipleListeners(t *testing.T) {
	obs := NewObservable(0)

	var received1, received2 int
	obs.AddListener(func(v int) { received1 = v })
	obs.AddListener(func(v int) { received2 = v })

	obs.Set(99)

	if received1 != 99 || received2 != 99 {
		t.Errorf("All listeners should receive the value, got %d and %d", received1, received2)
	}
}

func TestObservable_Update(t *testing.T) {
	obs := NewObservable(10)

	obs.Update(func(v int) int { return v * 2 })

	if obs.Value() != 20 {
		t.Errorf("Expected 20 after update, got %d", obs.Value())
	}
}

func TestObservable_Update_NotifiesListeners(t *testing.T) {
	obs := NewObservable(5)

	var received int
	obs.AddListener(func(v int) { received = v })

	obs.Update(func(v int) int { return v + 10 })

	if received != 15 {
		t.Errorf("Listener should receive updated value 15, got %d", received)
	}
}

func TestObservable_WithEquality(t *testing.T) {
	obs := NewObservableWithEquality(42, func(a, b int) bool {
		return a == b
	})

	callCount := 0
	obs.AddListener(func(v int) { callCount++ })

	obs.Set(42) // Same value
	obs.Set(42) // Same value
	obs.Set(43) // Different value

	if callCount != 1 {
		t.Errorf("Listener should only be called once for changed value, called %d times", callCount)
	}
}

func TestObservable_ConcurrentAccess(t *testing.T) {
	obs := NewObservable(0)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(v int) {
			defer wg.Done()
			obs.Set(v)
		}(i)
		go func() {
			defer wg.Done()
			_ = obs.Value()
		}()
	}
	wg.Wait()
}

func TestNotifier_Notify(t *testing.T) {
	n := NewNotifier()

	called := false
	n.AddListener(func() {
		called = true
	})

	n.Notify()

	if !called {
		t.Error("Listener should be called on Notify")
	}
}

func TestNotifier_AddListener_Unsubscribe(t *testing.T) {
	n := NewNotifier()

	callCount := 0
	unsub := n.AddListener(func() {
		callCount++
	})

	n.Notify()
	unsub()
	n.Notify()

	if callCount != 1 {
		t.Errorf("Listener should only be called once before unsubscribe, called %d times", callCount)
	}
}

func TestNotifier_MultipleListeners(t *testing.T) {
	n := NewNotifier()

	var count1, count2 int
	n.AddListener(func() { count1++ })
	n.AddListener(func() { count2++ })

	n.Notify()
	n.Notify()

	if count1 != 2 || count2 != 2 {
		t.Errorf("All listeners should be called twice, got %d and %d", count1, count2)
	}
}

func TestNotifier_ListenerCount(t *testing.T) {
	n := NewNotifier()

	if n.ListenerCount() != 0 {
		t.Errorf("Expected 0 listeners, got %d", n.ListenerCount())
	}

	unsub1 := n.AddListener(func() {})
	unsub2 := n.AddListener(func() {})

	if n.ListenerCount() != 2 {
		t.Errorf("Expected 2 listeners, got %d", n.ListenerCount())
	}

	unsub1()

	if n.ListenerCount() != 1 {
		t.Errorf("Expected 1 listener, got %d", n.ListenerCount())
	}

	unsub2()

	if n.ListenerCount() != 0 {
		t.Errorf("Expected 0 listeners, got %d", n.ListenerCount())
	}
}

func TestNotifier_ConcurrentAccess(t *testing.T) {
	n := NewNotifier()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			unsub := n.AddListener(func() {})
			unsub()
		}()
		go func() {
			defer wg.Done()
			n.Notify()
		}()
	}
	wg.Wait()
}

// Compile-time check that Notifier implements Listenable
var _ Listenable = &Notifier{}
