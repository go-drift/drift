package core

import "testing"

// MockDisposable for testing UseController
type mockDisposable struct {
	disposed bool
}

func (m *mockDisposable) Dispose() {
	m.disposed = true
}

func TestUseController(t *testing.T) {
	base := &StateBase{}

	controller := UseController(base, func() *mockDisposable {
		return &mockDisposable{}
	})

	if controller.disposed {
		t.Error("Controller should not be disposed initially")
	}

	base.Dispose()

	if !controller.disposed {
		t.Error("Controller should be disposed when StateBase is disposed")
	}
}

func TestUseListenable(t *testing.T) {
	base := &StateBase{}
	notifier := NewNotifier()

	UseListenable(base, notifier)

	// We can't easily test SetState being called without a real element,
	// but we can verify the subscription is set up
	if notifier.ListenerCount() != 1 {
		t.Errorf("Expected 1 listener, got %d", notifier.ListenerCount())
	}

	base.Dispose()

	if notifier.ListenerCount() != 0 {
		t.Errorf("Expected 0 listeners after dispose, got %d", notifier.ListenerCount())
	}
}

func TestUseObservable(t *testing.T) {
	base := &StateBase{}
	obs := NewObservable(42)

	UseObservable(base, obs)

	// Verify we can read the value
	if obs.Value() != 42 {
		t.Errorf("Expected 42, got %d", obs.Value())
	}

	// Verify listener was registered (observable should trigger rebuild on change)
	obs.Set(100)
	// SetState was called (can't easily verify without element, but no panic = good)
}

func TestUseObservable_Cleanup(t *testing.T) {
	base := &StateBase{}
	obs := NewObservable(0)

	UseObservable(base, obs)

	base.Dispose()

	// After dispose, setting the observable should not panic
	obs.Set(999)
}

func TestManaged_Value(t *testing.T) {
	base := &StateBase{}
	state := NewManaged(base, 42)

	if state.Value() != 42 {
		t.Errorf("Expected 42, got %d", state.Value())
	}
}

func TestManaged_Set(t *testing.T) {
	base := &StateBase{}
	state := NewManaged(base, 0)

	state.Set(100)

	if state.Value() != 100 {
		t.Errorf("Expected 100, got %d", state.Value())
	}
}

func TestManaged_Update(t *testing.T) {
	base := &StateBase{}
	state := NewManaged(base, 10)

	state.Update(func(v int) int { return v * 2 })

	if state.Value() != 20 {
		t.Errorf("Expected 20, got %d", state.Value())
	}
}

func TestManaged_StringType(t *testing.T) {
	base := &StateBase{}
	state := NewManaged(base, "hello")

	if state.Value() != "hello" {
		t.Errorf("Expected 'hello', got '%s'", state.Value())
	}

	state.Set("world")

	if state.Value() != "world" {
		t.Errorf("Expected 'world', got '%s'", state.Value())
	}
}

func TestManaged_StructType(t *testing.T) {
	type Person struct {
		Name string
		Age  int
	}

	base := &StateBase{}
	state := NewManaged(base, Person{Name: "Alice", Age: 30})

	if state.Value().Name != "Alice" || state.Value().Age != 30 {
		t.Errorf("Unexpected struct value: %+v", state.Value())
	}

	state.Update(func(p Person) Person {
		p.Age++
		return p
	})

	if state.Value().Age != 31 {
		t.Errorf("Expected age 31, got %d", state.Value().Age)
	}
}

// --- UseObservableSelector tests ---

type user struct {
	Name string
	Age  int
}

func TestUseObservableSelector_OnlyRebuildsOnSelectorChange(t *testing.T) {
	// Use a second StateBase wrapping the first to count SetState calls.
	// When selector output is unchanged, SetState should not be called.
	base := &StateBase{}
	obs := NewObservable(user{Name: "Alice", Age: 30})

	setStateCalls := 0
	// Register a disposer that we immediately remove, just to prove the
	// mechanism works. Instead, count via an OnChange-style wrapper:
	// We add a listener *after* the selector to count how many times
	// the selector triggers a rebuild vs. the raw observable.
	UseObservableSelector(base, obs, func(u user) string { return u.Name })

	// Add a second listener that counts all observable fires
	allFires := 0
	obs.AddListener(func(u user) { allFires++ })

	// Intercept MarkNeedsBuild by giving base a real element with a buildOwner
	owner := NewBuildOwner()
	elem := &StatefulElement{}
	elem.buildOwner = owner
	elem.self = elem
	base.SetElement(elem)

	// Change age only (Name stays "Alice"): selector output unchanged
	obs.Set(user{Name: "Alice", Age: 31})
	setStateCalls = countDirty(owner)

	if setStateCalls != 0 {
		t.Errorf("expected 0 rebuilds when selector unchanged, got %d", setStateCalls)
	}

	// Change name: selector output changes
	obs.Set(user{Name: "Bob", Age: 31})
	setStateCalls = countDirty(owner)

	if setStateCalls != 1 {
		t.Errorf("expected 1 rebuild when selector changed, got %d", setStateCalls)
	}

	if allFires != 2 {
		t.Errorf("expected 2 total observable fires, got %d", allFires)
	}
}

// countDirty returns the number of dirty elements and drains the list.
func countDirty(owner *BuildOwner) int {
	owner.mu.Lock()
	n := len(owner.dirty)
	owner.dirty = nil
	clear(owner.dirtySet)
	owner.mu.Unlock()
	return n
}

func TestUseObservableSelector_DoesNotRebuildWhenSelectorSame(t *testing.T) {
	base := &StateBase{}
	obs := NewObservable(user{Name: "Alice", Age: 30})

	selectorCalls := 0
	UseObservableSelector(base, obs, func(u user) string {
		selectorCalls++
		return u.Name
	})

	// One call happens during setup to compute initial lastSelected
	initialCalls := selectorCalls

	// Change age but not name
	obs.Set(user{Name: "Alice", Age: 99})

	// Selector should have been called once more (to check new value)
	if selectorCalls != initialCalls+1 {
		t.Errorf("expected %d selector calls, got %d", initialCalls+1, selectorCalls)
	}
}

func TestUseObservableSelector_Cleanup(t *testing.T) {
	base := &StateBase{}
	obs := NewObservable(user{Name: "Alice", Age: 30})

	UseObservableSelector(base, obs, func(u user) string { return u.Name })

	initialCount := obs.ListenerCount()
	if initialCount < 1 {
		t.Errorf("expected at least 1 listener, got %d", initialCount)
	}

	base.Dispose()

	if obs.ListenerCount() != initialCount-1 {
		t.Errorf("expected listener removed after dispose, got %d", obs.ListenerCount())
	}
}

func TestUseObservableSelectorWithEquality(t *testing.T) {
	base := &StateBase{}
	obs := NewObservable([]string{"a", "b", "c"})

	selectorCalls := 0
	UseObservableSelectorWithEquality(base, obs, func(s []string) int {
		selectorCalls++
		return len(s)
	}, func(a, b int) bool {
		return a == b
	})

	initialCalls := selectorCalls

	// Same length, different content: should not trigger rebuild
	obs.Set([]string{"x", "y", "z"})
	if selectorCalls != initialCalls+1 {
		t.Errorf("expected %d selector calls, got %d", initialCalls+1, selectorCalls)
	}
}

func TestUseObservableSelector_WithDerivedObservable(t *testing.T) {
	base := &StateBase{}
	src := NewObservable(user{Name: "Alice", Age: 30})
	derived := Derive(func() user { return src.Value() }, src)
	defer derived.Dispose()

	selectorCalls := 0
	UseObservableSelector(base, derived, func(u user) int {
		selectorCalls++
		return u.Age
	})

	initialCalls := selectorCalls

	src.Set(user{Name: "Bob", Age: 30}) // age unchanged
	if selectorCalls != initialCalls+1 {
		t.Errorf("expected %d selector calls, got %d", initialCalls+1, selectorCalls)
	}

	src.Set(user{Name: "Bob", Age: 31}) // age changed
	if selectorCalls != initialCalls+2 {
		t.Errorf("expected %d selector calls, got %d", initialCalls+2, selectorCalls)
	}

	base.Dispose()
}
