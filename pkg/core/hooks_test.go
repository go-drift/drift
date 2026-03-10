package core

import (
	"slices"
	"testing"
)

// MockDisposable for testing UseDisposable
type mockDisposable struct {
	disposed bool
}

func (m *mockDisposable) Dispose() {
	m.disposed = true
}

func TestUseDisposable(t *testing.T) {
	base := &StateBase{}

	controller := &mockDisposable{}
	UseDisposable(base, controller)

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
	notifier := &Notifier{}

	UseListenable(base, notifier)

	if notifier.ListenerCount() != 1 {
		t.Errorf("Expected 1 listener, got %d", notifier.ListenerCount())
	}

	base.Dispose()

	if notifier.ListenerCount() != 0 {
		t.Errorf("Expected 0 listeners after dispose, got %d", notifier.ListenerCount())
	}
}

func TestUseListenable_WithSignal(t *testing.T) {
	base := &StateBase{}
	sig := NewSignal(42)

	UseListenable(base, sig)

	// Verify we can read the value
	if sig.Value() != 42 {
		t.Errorf("Expected 42, got %d", sig.Value())
	}

	// Verify listener was registered (signal should trigger rebuild on change)
	sig.Set(100)
	// SetState was called (can't easily verify without element, but no panic = good)
}

func TestUseListenable_WithSignal_Cleanup(t *testing.T) {
	base := &StateBase{}
	sig := NewSignal(0)

	UseListenable(base, sig)

	base.Dispose()

	// After dispose, setting the signal should not panic
	sig.Set(999)
}

func TestUseDerivedWithEquality(t *testing.T) {
	base := &StateBase{}
	src := NewSignalWithEquality([]string{"a", "b"}, slices.Equal)

	d := UseDerivedWithEquality(base, func() []string {
		return src.Value()
	}, slices.Equal, src)

	if !slices.Equal(d.Value(), []string{"a", "b"}) {
		t.Errorf("expected [a b], got %v", d.Value())
	}

	// Same content, different slice: should not notify
	callCount := 0
	d.AddListener(func() { callCount++ })
	src.Set([]string{"a", "b"})
	if callCount != 0 {
		t.Errorf("expected 0 notifications for equal slice, got %d", callCount)
	}

	// Different content: should notify
	src.Set([]string{"a", "b", "c"})
	if callCount != 1 {
		t.Errorf("expected 1 notification for changed slice, got %d", callCount)
	}

	// Dispose should clean up
	base.Dispose()
	src.Set([]string{"x"})
	if callCount != 1 {
		t.Errorf("expected no further notifications after dispose, got %d", callCount)
	}
}

// --- UseSelector tests ---

type user struct {
	Name string
	Age  int
}

func TestUseSelector_OnlyRebuildsOnSelectorChange(t *testing.T) {
	base := &StateBase{}
	sig := NewSignal(user{Name: "Alice", Age: 30})

	setStateCalls := 0
	UseSelector(base, sig, func() string { return sig.Value().Name })

	// Add a second listener that counts all signal fires
	allFires := 0
	sig.AddListener(func() { allFires++ })

	// Intercept MarkNeedsBuild by giving base a real element with a buildOwner
	owner := NewBuildOwner()
	elem := &StatefulElement{}
	elem.buildOwner = owner
	elem.self = elem
	base.SetElement(elem)

	// Change age only (Name stays "Alice"): selector output unchanged
	sig.Set(user{Name: "Alice", Age: 31})
	setStateCalls = countDirty(owner)

	if setStateCalls != 0 {
		t.Errorf("expected 0 rebuilds when selector unchanged, got %d", setStateCalls)
	}

	// Change name: selector output changes
	sig.Set(user{Name: "Bob", Age: 31})
	setStateCalls = countDirty(owner)

	if setStateCalls != 1 {
		t.Errorf("expected 1 rebuild when selector changed, got %d", setStateCalls)
	}

	if allFires != 2 {
		t.Errorf("expected 2 total signal fires, got %d", allFires)
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

func TestUseSelector_DoesNotRebuildWhenSelectorSame(t *testing.T) {
	base := &StateBase{}
	sig := NewSignal(user{Name: "Alice", Age: 30})

	selectorCalls := 0
	UseSelector(base, sig, func() string {
		selectorCalls++
		return sig.Value().Name
	})

	// One call happens during setup to compute initial lastSelected
	initialCalls := selectorCalls

	// Change age but not name
	sig.Set(user{Name: "Alice", Age: 99})

	// Selector should have been called once more (to check new value)
	if selectorCalls != initialCalls+1 {
		t.Errorf("expected %d selector calls, got %d", initialCalls+1, selectorCalls)
	}
}

func TestUseSelector_Cleanup(t *testing.T) {
	base := &StateBase{}
	sig := NewSignal(user{Name: "Alice", Age: 30})

	UseSelector(base, sig, func() string { return sig.Value().Name })

	initialCount := sig.ListenerCount()
	if initialCount < 1 {
		t.Errorf("expected at least 1 listener, got %d", initialCount)
	}

	base.Dispose()

	if sig.ListenerCount() != initialCount-1 {
		t.Errorf("expected listener removed after dispose, got %d", sig.ListenerCount())
	}
}

func TestUseSelectorWithEquality(t *testing.T) {
	base := &StateBase{}
	sig := NewSignalWithEquality([]string{"a", "b", "c"}, slices.Equal)

	selectorCalls := 0
	UseSelectorWithEquality(base, sig, func() int {
		selectorCalls++
		return len(sig.Value())
	}, func(a, b int) bool {
		return a == b
	})

	initialCalls := selectorCalls

	// Same length, different content: should not trigger rebuild
	sig.Set([]string{"x", "y", "z"})
	if selectorCalls != initialCalls+1 {
		t.Errorf("expected %d selector calls, got %d", initialCalls+1, selectorCalls)
	}
}

func TestUseSelector_WithDerived(t *testing.T) {
	base := &StateBase{}
	src := NewSignal(user{Name: "Alice", Age: 30})
	derived := NewDerived(func() user { return src.Value() }, src)
	defer derived.Dispose()

	selectorCalls := 0
	UseSelector(base, derived, func() int {
		selectorCalls++
		return derived.Value().Age
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
