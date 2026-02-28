package core

import "testing"

// --- Test widgets for GlobalKey ---

type globalKeyTestWidget struct {
	StatefulBase
	key any
}

func (w globalKeyTestWidget) Key() any           { return w.key }
func (w globalKeyTestWidget) CreateState() State { return &globalKeyTestState{} }

type globalKeyTestState struct {
	StateBase
	label string
}

func (s *globalKeyTestState) Build(ctx BuildContext) Widget { return nil }

// --- Tests ---

func TestGlobalKey_NewGlobalKeyUnique(t *testing.T) {
	k1 := NewGlobalKey[*globalKeyTestState]()
	k2 := NewGlobalKey[*globalKeyTestState]()

	if k1.inner == k2.inner {
		t.Error("two NewGlobalKey calls should produce different identities")
	}
}

func TestGlobalKey_CurrentStateBeforeMount(t *testing.T) {
	key := NewGlobalKey[*globalKeyTestState]()
	if key.CurrentState() != nil {
		t.Error("CurrentState should be nil before mount")
	}
}

func TestGlobalKey_CurrentStateAfterMount(t *testing.T) {
	key := NewGlobalKey[*globalKeyTestState]()
	owner := NewBuildOwner()

	widget := globalKeyTestWidget{key: key}
	elem := inflateWidget(widget, owner)
	elem.Mount(nil, nil)

	state := key.CurrentState()
	if state == nil {
		t.Fatal("CurrentState should return non-nil state after mount")
	}
	if _, ok := any(state).(*globalKeyTestState); !ok {
		t.Errorf("expected *globalKeyTestState, got %T", state)
	}
}

func TestGlobalKey_CurrentStateAfterUnmount(t *testing.T) {
	key := NewGlobalKey[*globalKeyTestState]()
	owner := NewBuildOwner()

	widget := globalKeyTestWidget{key: key}
	elem := inflateWidget(widget, owner)
	elem.Mount(nil, nil)

	elem.Unmount()

	if key.CurrentState() != nil {
		t.Error("CurrentState should be nil after unmount")
	}
}

func TestGlobalKey_CurrentElement(t *testing.T) {
	key := NewGlobalKey[*globalKeyTestState]()
	owner := NewBuildOwner()

	widget := globalKeyTestWidget{key: key}
	elem := inflateWidget(widget, owner)
	elem.Mount(nil, nil)

	if key.CurrentElement() == nil {
		t.Error("CurrentElement should be non-nil after mount")
	}
	if key.CurrentElement() != elem {
		t.Error("CurrentElement should return the mounted element")
	}
}

func TestGlobalKey_CurrentContext(t *testing.T) {
	key := NewGlobalKey[*globalKeyTestState]()
	owner := NewBuildOwner()

	widget := globalKeyTestWidget{key: key}
	elem := inflateWidget(widget, owner)
	elem.Mount(nil, nil)

	ctx := key.CurrentContext()
	if ctx == nil {
		t.Error("CurrentContext should be non-nil after mount")
	}
}

func TestGlobalKey_CanUpdateWidget(t *testing.T) {
	key := NewGlobalKey[*globalKeyTestState]()

	w1 := globalKeyTestWidget{key: key}
	w2 := globalKeyTestWidget{key: key}

	if !canUpdateWidget(w1, w2) {
		t.Error("canUpdateWidget should return true for same GlobalKey")
	}
}

func TestGlobalKey_DifferentKeysRejectUpdate(t *testing.T) {
	k1 := NewGlobalKey[*globalKeyTestState]()
	k2 := NewGlobalKey[*globalKeyTestState]()

	w1 := globalKeyTestWidget{key: k1}
	w2 := globalKeyTestWidget{key: k2}

	if canUpdateWidget(w1, w2) {
		t.Error("canUpdateWidget should return false for different GlobalKeys")
	}
}

func TestGlobalKey_BuildOwnerRegistryCleanup(t *testing.T) {
	key := NewGlobalKey[*globalKeyTestState]()
	owner := NewBuildOwner()

	widget := globalKeyTestWidget{key: key}
	elem := inflateWidget(widget, owner)
	elem.Mount(nil, nil)

	owner.mu.Lock()
	count := len(owner.globalKeys)
	owner.mu.Unlock()
	if count != 1 {
		t.Errorf("expected 1 global key registered, got %d", count)
	}

	elem.Unmount()

	owner.mu.Lock()
	count = len(owner.globalKeys)
	owner.mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 global keys after unmount, got %d", count)
	}
}

func TestGlobalKey_StaleUnregisterGuard(t *testing.T) {
	key := NewGlobalKey[*globalKeyTestState]()
	owner := NewBuildOwner()

	// Mount element 1
	w1 := globalKeyTestWidget{key: key}
	elem1 := inflateWidget(w1, owner)
	elem1.Mount(nil, nil)

	// Mount element 2 with same key (overwrites)
	w2 := globalKeyTestWidget{key: key}
	elem2 := inflateWidget(w2, owner)
	elem2.Mount(nil, nil)

	// Unregister with stale element should not remove the key
	owner.UnregisterGlobalKey(key.inner, elem1)

	owner.mu.Lock()
	count := len(owner.globalKeys)
	owner.mu.Unlock()
	if count != 1 {
		t.Errorf("stale unregister should not remove key; expected 1, got %d", count)
	}
}

// Test with a stateless widget to verify non-stateful elements also register.
type globalKeyStatelessWidget struct {
	StatelessBase
	key any
}

func (w globalKeyStatelessWidget) Key() any                      { return w.key }
func (w globalKeyStatelessWidget) Build(ctx BuildContext) Widget { return nil }

func TestGlobalKey_StatelessElement(t *testing.T) {
	key := NewGlobalKey[*globalKeyTestState]()
	owner := NewBuildOwner()

	widget := globalKeyStatelessWidget{key: key}
	elem := inflateWidget(widget, owner)
	elem.Mount(nil, nil)

	if key.CurrentElement() == nil {
		t.Error("GlobalKey should register element for StatelessElement")
	}

	// CurrentState should return nil for a stateless element (no state)
	if key.CurrentState() != nil {
		t.Error("CurrentState should be nil for StatelessElement")
	}

	elem.Unmount()

	if key.CurrentElement() != nil {
		t.Error("GlobalKey element should be nil after unmount")
	}
}
