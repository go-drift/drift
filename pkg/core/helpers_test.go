package core

import (
	"reflect"
	"testing"
)

// --- StatelessBase tests ---

type testStatelessBaseWidget struct {
	StatelessBase
	label string
}

func (w testStatelessBaseWidget) Build(ctx BuildContext) Widget { return nil }

func TestStatelessBase_SatisfiesInterface(t *testing.T) {
	var w any = testStatelessBaseWidget{label: "hello"}
	if _, ok := w.(StatelessWidget); !ok {
		t.Error("widget embedding StatelessBase should satisfy StatelessWidget")
	}
}

func TestStatelessBase_DefaultKey(t *testing.T) {
	w := testStatelessBaseWidget{}
	if w.Key() != nil {
		t.Errorf("expected nil key, got %v", w.Key())
	}
}

func TestStatelessBase_CreateElement(t *testing.T) {
	w := testStatelessBaseWidget{}
	elem := w.CreateElement()
	if elem == nil {
		t.Fatal("CreateElement should return non-nil element")
	}
	if _, ok := elem.(*StatelessElement); !ok {
		t.Errorf("expected *StatelessElement, got %T", elem)
	}
}

type keyedStatelessBaseWidget struct {
	StatelessBase
	myKey string
}

func (w keyedStatelessBaseWidget) Build(ctx BuildContext) Widget { return nil }
func (w keyedStatelessBaseWidget) Key() any                      { return w.myKey }

func TestStatelessBase_KeyOverride(t *testing.T) {
	w := keyedStatelessBaseWidget{myKey: "custom"}
	if w.Key() != "custom" {
		t.Errorf("expected key 'custom', got %v", w.Key())
	}
}

// --- StatefulBase tests ---

type testStatefulBaseWidget struct {
	StatefulBase
}

type testStateA struct {
	StateBase
}

func (s *testStateA) Build(ctx BuildContext) Widget { return nil }

func (testStatefulBaseWidget) CreateState() State { return &testStateA{} }

func TestStatefulBase_SatisfiesInterface(t *testing.T) {
	var w any = testStatefulBaseWidget{}
	if _, ok := w.(StatefulWidget); !ok {
		t.Error("widget embedding StatefulBase should satisfy StatefulWidget")
	}
}

func TestStatefulBase_DefaultKey(t *testing.T) {
	w := testStatefulBaseWidget{}
	if w.Key() != nil {
		t.Errorf("expected nil key, got %v", w.Key())
	}
}

func TestStatefulBase_CreateElement(t *testing.T) {
	w := testStatefulBaseWidget{}
	elem := w.CreateElement()
	if elem == nil {
		t.Fatal("CreateElement should return non-nil element")
	}
	if _, ok := elem.(*StatefulElement); !ok {
		t.Errorf("expected *StatefulElement, got %T", elem)
	}
}

type keyedStatefulBaseWidget struct {
	StatefulBase
	myKey string
}

func (keyedStatefulBaseWidget) CreateState() State { return &testStateA{} }
func (w keyedStatefulBaseWidget) Key() any         { return w.myKey }

func TestStatefulBase_KeyOverride(t *testing.T) {
	w := keyedStatefulBaseWidget{myKey: "my-key"}
	if w.Key() != "my-key" {
		t.Errorf("expected key 'my-key', got %v", w.Key())
	}
}

type testStateB struct {
	StateBase
}

func (s *testStateB) Build(ctx BuildContext) Widget { return nil }

func TestStatefulBase_DifferentOuterTypes(t *testing.T) {
	type widgetA struct {
		StatefulBase
	}
	type widgetB struct {
		StatefulBase
	}

	typeA := reflect.TypeOf(widgetA{})
	typeB := reflect.TypeOf(widgetB{})

	if typeA == typeB {
		t.Error("different outer struct types should produce different reflect.TypeOf results")
	}
}

// --- InheritedBase tests ---

type testInheritedBaseWidget struct {
	InheritedBase
	value int
	child Widget
}

func (w testInheritedBaseWidget) ChildWidget() Widget { return w.child }
func (w testInheritedBaseWidget) UpdateShouldNotify(old InheritedWidget) bool {
	return w.value != old.(testInheritedBaseWidget).value
}

func TestInheritedBase_SatisfiesInterface(t *testing.T) {
	var w any = testInheritedBaseWidget{value: 1}
	if _, ok := w.(InheritedWidget); !ok {
		t.Error("widget embedding InheritedBase should satisfy InheritedWidget")
	}
}

func TestInheritedBase_DefaultKey(t *testing.T) {
	w := testInheritedBaseWidget{}
	if w.Key() != nil {
		t.Errorf("expected nil key, got %v", w.Key())
	}
}

func TestInheritedBase_CreateElement(t *testing.T) {
	w := testInheritedBaseWidget{}
	elem := w.CreateElement()
	if elem == nil {
		t.Fatal("CreateElement should return non-nil element")
	}
	if _, ok := elem.(*InheritedElement); !ok {
		t.Errorf("expected *InheritedElement, got %T", elem)
	}
}

type keyedInheritedBaseWidget struct {
	InheritedBase
	myKey string
	child Widget
}

func (w keyedInheritedBaseWidget) Key() any              { return w.myKey }
func (w keyedInheritedBaseWidget) ChildWidget() Widget   { return w.child }
func (w keyedInheritedBaseWidget) UpdateShouldNotify(InheritedWidget) bool { return false }

func TestInheritedBase_KeyOverride(t *testing.T) {
	w := keyedInheritedBaseWidget{myKey: "custom"}
	if w.Key() != "custom" {
		t.Errorf("expected key 'custom', got %v", w.Key())
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

func (m *mockBuildContext) Widget() Widget                                               { return nil }
func (m *mockBuildContext) FindAncestor(predicate func(Element) bool) Element            { return nil }
func (m *mockBuildContext) DependOnInherited(inheritedType reflect.Type, aspect any) any { return nil }
func (m *mockBuildContext) DependOnInheritedWithAspects(inheritedType reflect.Type, aspects ...any) any {
	return nil
}
