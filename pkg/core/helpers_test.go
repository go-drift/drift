package core

import (
	"reflect"
	"testing"
)

// testStateA is a minimal State implementation for testing.
type testStateA struct {
	StateBase
}

func (s *testStateA) Build(ctx BuildContext) Widget { return nil }

// testStateB is a different State type for reconciliation testing.
type testStateB struct {
	StateBase
}

func (s *testStateB) Build(ctx BuildContext) Widget { return nil }

func TestNewStatefulWidget_SatisfiesInterface(t *testing.T) {
	var w any = NewStatefulWidget(func() *testStateA { return &testStateA{} })

	if _, ok := w.(StatefulWidget); !ok {
		t.Error("NewStatefulWidget should return a StatefulWidget")
	}
}

func TestNewStatefulWidget_CreateState(t *testing.T) {
	w := NewStatefulWidget(func() *testStateA { return &testStateA{} })

	state1 := w.CreateState()
	if state1 == nil {
		t.Fatal("CreateState should return non-nil state")
	}
	if _, ok := state1.(*testStateA); !ok {
		t.Errorf("CreateState should return *testStateA, got %T", state1)
	}

	state2 := w.CreateState()
	if state1 == state2 {
		t.Error("CreateState should return a new instance on each call")
	}
}

func TestNewStatefulWidget_CreateElement(t *testing.T) {
	w := NewStatefulWidget(func() *testStateA { return &testStateA{} })

	elem := w.CreateElement()
	if elem == nil {
		t.Fatal("CreateElement should return non-nil element")
	}
}

func TestNewStatefulWidget_KeyDefault(t *testing.T) {
	w := NewStatefulWidget(func() *testStateA { return &testStateA{} })

	if w.Key() != nil {
		t.Errorf("Key should be nil by default, got %v", w.Key())
	}
}

func TestNewStatefulWidget_KeyProvided(t *testing.T) {
	w := NewStatefulWidget(func() *testStateA { return &testStateA{} }, "my-key")

	if w.Key() != "my-key" {
		t.Errorf("Key should be 'my-key', got %v", w.Key())
	}
}

func TestNewStatefulWidget_DifferentTypesProduceDifferentReflectTypes(t *testing.T) {
	wA := NewStatefulWidget(func() *testStateA { return &testStateA{} })
	wB := NewStatefulWidget(func() *testStateB { return &testStateB{} })

	typeA := reflect.TypeOf(wA)
	typeB := reflect.TypeOf(wB)

	if typeA == typeB {
		t.Errorf("Different state types should produce different reflect.TypeOf results: %v vs %v", typeA, typeB)
	}
}

// --- Stateful helper tests ---

func TestStateful_ReturnsStatefulWidget(t *testing.T) {
	w := Stateful(
		func() int { return 0 },
		func(state int, ctx BuildContext, setState func(func(int) int)) Widget { return nil },
	)
	if _, ok := w.(StatefulWidget); !ok {
		t.Error("Stateful should return a StatefulWidget")
	}
}

func TestStateful_InitSetsState(t *testing.T) {
	sw := Stateful(
		func() int { return 42 },
		func(state int, ctx BuildContext, setState func(func(int) int)) Widget { return nil },
	).(StatefulWidget)

	state := sw.CreateState().(*inlineStatefulState[int])
	state.InitState()

	if state.value != 42 {
		t.Errorf("expected initial state 42, got %d", state.value)
	}
}

func TestStateful_BuildReceivesStateAndContext(t *testing.T) {
	var gotState int
	var gotCtx BuildContext

	sw := Stateful(
		func() int { return 7 },
		func(state int, ctx BuildContext, setState func(func(int) int)) Widget {
			gotState = state
			gotCtx = ctx
			return nil
		},
	).(StatefulWidget)

	state := sw.CreateState().(*inlineStatefulState[int])
	state.InitState()

	var sentinel BuildContext = &mockBuildContext{}
	state.Build(sentinel)

	if gotState != 7 {
		t.Errorf("expected state 7, got %d", gotState)
	}
	if gotCtx != sentinel {
		t.Error("expected BuildContext to be passed through")
	}
}

func TestStateful_SetStateUpdatesValue(t *testing.T) {
	var setStateFn func(func(int) int)

	sw := Stateful(
		func() int { return 0 },
		func(state int, ctx BuildContext, setState func(func(int) int)) Widget {
			setStateFn = setState
			return nil
		},
	).(StatefulWidget)

	state := sw.CreateState().(*inlineStatefulState[int])
	state.InitState()

	elem := &StatefulElement{}
	state.SetElement(elem)

	state.Build(nil) // captures setState

	setStateFn(func(v int) int { return v + 10 })

	if state.value != 10 {
		t.Errorf("expected state 10 after setState, got %d", state.value)
	}
}

func TestStateful_KeyIsNil(t *testing.T) {
	w := Stateful(
		func() int { return 0 },
		func(state int, ctx BuildContext, setState func(func(int) int)) Widget { return nil },
	)
	if w.(StatefulWidget).Key() != nil {
		t.Error("Stateful widget key should be nil")
	}
}

// mockBuildContext satisfies BuildContext for testing.
type mockBuildContext struct{}

func (m *mockBuildContext) Widget() Widget                                                    { return nil }
func (m *mockBuildContext) FindAncestor(predicate func(Element) bool) Element                 { return nil }
func (m *mockBuildContext) DependOnInherited(inheritedType reflect.Type, aspect any) any       { return nil }
func (m *mockBuildContext) DependOnInheritedWithAspects(inheritedType reflect.Type, aspects ...any) any {
	return nil
}
