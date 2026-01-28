package core

import (
	"testing"

	"github.com/go-drift/drift/pkg/errors"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
)

// testStatelessWidget is a simple stateless widget for testing.
type testStatelessWidget struct {
	buildFn func(BuildContext) Widget
}

func (w testStatelessWidget) CreateElement() Element {
	return NewStatelessElement(w, nil)
}

func (w testStatelessWidget) Key() any {
	return nil
}

func (w testStatelessWidget) Build(ctx BuildContext) Widget {
	if w.buildFn != nil {
		return w.buildFn(ctx)
	}
	return nil
}

// testStatefulWidget is a simple stateful widget for testing.
type testStatefulWidget struct {
	createStateFn func() State
}

func (w testStatefulWidget) CreateElement() Element {
	return NewStatefulElement(w, nil)
}

func (w testStatefulWidget) Key() any {
	return nil
}

func (w testStatefulWidget) CreateState() State {
	if w.createStateFn != nil {
		return w.createStateFn()
	}
	return &testState{}
}

type testState struct {
	StateBase
	buildFn func(BuildContext) Widget
}

func (s *testState) Build(ctx BuildContext) Widget {
	if s.buildFn != nil {
		return s.buildFn(ctx)
	}
	return nil
}

// testErrorHandler captures errors for testing.
type testErrorHandler struct {
	errors.LogHandler
	buildErrors []*errors.BuildError
}

func (h *testErrorHandler) HandleBuildError(err *errors.BuildError) {
	h.buildErrors = append(h.buildErrors, err)
}

func TestStatelessElement_BuildPanic_ReportsError(t *testing.T) {
	handler := &testErrorHandler{}
	errors.SetHandler(handler)
	defer errors.SetHandler(nil)

	widget := testStatelessWidget{
		buildFn: func(ctx BuildContext) Widget {
			panic("test panic in stateless build")
		},
	}

	owner := NewBuildOwner()
	element := NewStatelessElement(widget, owner)
	element.Mount(nil, nil)

	if len(handler.buildErrors) != 1 {
		t.Fatalf("expected 1 build error, got %d", len(handler.buildErrors))
	}

	err := handler.buildErrors[0]
	if err.Recovered != "test panic in stateless build" {
		t.Errorf("expected panic value 'test panic in stateless build', got %v", err.Recovered)
	}
	if err.Widget == "" {
		t.Error("expected Widget type to be set")
	}
	if err.Element == "" {
		t.Error("expected Element type to be set")
	}
	if err.StackTrace == "" {
		t.Error("expected StackTrace to be captured")
	}
}

func TestStatefulElement_BuildPanic_ReportsError(t *testing.T) {
	handler := &testErrorHandler{}
	errors.SetHandler(handler)
	defer errors.SetHandler(nil)

	widget := testStatefulWidget{
		createStateFn: func() State {
			return &testState{
				buildFn: func(ctx BuildContext) Widget {
					panic("test panic in stateful build")
				},
			}
		},
	}

	owner := NewBuildOwner()
	element := NewStatefulElement(widget, owner)
	element.Mount(nil, nil)

	if len(handler.buildErrors) != 1 {
		t.Fatalf("expected 1 build error, got %d", len(handler.buildErrors))
	}

	err := handler.buildErrors[0]
	if err.Recovered != "test panic in stateful build" {
		t.Errorf("expected panic value 'test panic in stateful build', got %v", err.Recovered)
	}
}

func TestSafeBuild_ReturnsErrorPlaceholder_WhenNoBuilder(t *testing.T) {
	// Temporarily clear the error widget builder
	oldBuilder := GetErrorWidgetBuilder()
	SetErrorWidgetBuilder(func(err *errors.BuildError) Widget {
		return nil // Force fallback to errorPlaceholder
	})
	defer SetErrorWidgetBuilder(oldBuilder)

	handler := &testErrorHandler{}
	errors.SetHandler(handler)
	defer errors.SetHandler(nil)

	widget := testStatelessWidget{
		buildFn: func(ctx BuildContext) Widget {
			panic("test panic")
		},
	}

	owner := NewBuildOwner()
	element := NewStatelessElement(widget, owner)
	element.Mount(nil, nil)

	// The child should be an errorPlaceholder
	if element.child == nil {
		t.Fatal("expected child element to be set")
	}

	childWidget := element.child.Widget()
	if _, ok := childWidget.(errorPlaceholder); !ok {
		t.Errorf("expected errorPlaceholder widget, got %T", childWidget)
	}
}

func TestSafeBuild_UsesCustomBuilder(t *testing.T) {
	var capturedErr *errors.BuildError
	customWidget := testStatelessWidget{
		buildFn: func(ctx BuildContext) Widget {
			return nil
		},
	}

	SetErrorWidgetBuilder(func(err *errors.BuildError) Widget {
		capturedErr = err
		return customWidget
	})
	defer SetErrorWidgetBuilder(nil)

	handler := &testErrorHandler{}
	errors.SetHandler(handler)
	defer errors.SetHandler(nil)

	widget := testStatelessWidget{
		buildFn: func(ctx BuildContext) Widget {
			panic("custom builder test")
		},
	}

	owner := NewBuildOwner()
	element := NewStatelessElement(widget, owner)
	element.Mount(nil, nil)

	if capturedErr == nil {
		t.Fatal("expected custom builder to be called")
	}
	if capturedErr.Recovered != "custom builder test" {
		t.Errorf("expected panic value 'custom builder test', got %v", capturedErr.Recovered)
	}
}

func TestErrorPlaceholder_BuildReturnsNil(t *testing.T) {
	placeholder := errorPlaceholder{
		err: &errors.BuildError{Widget: "test"},
	}

	built := placeholder.Build(nil)
	if built != nil {
		t.Errorf("expected errorPlaceholder.Build() to return nil, got %v", built)
	}
}

func TestSetErrorWidgetBuilder_NilRestoresDefault(t *testing.T) {
	SetErrorWidgetBuilder(func(err *errors.BuildError) Widget {
		return testStatelessWidget{}
	})

	// Restore default
	SetErrorWidgetBuilder(nil)

	builder := GetErrorWidgetBuilder()
	if builder == nil {
		t.Fatal("expected non-nil builder after SetErrorWidgetBuilder(nil)")
	}

	// Default builder returns nil
	result := builder(&errors.BuildError{})
	if result != nil {
		t.Errorf("expected default builder to return nil, got %v", result)
	}
}

func TestDebugMode_Default(t *testing.T) {
	if !DebugMode {
		t.Error("expected DebugMode to default to true")
	}
}

func TestSetDebugMode(t *testing.T) {
	original := DebugMode

	SetDebugMode(false)
	if DebugMode {
		t.Error("expected DebugMode to be false")
	}

	SetDebugMode(true)
	if !DebugMode {
		t.Error("expected DebugMode to be true")
	}

	// Restore original
	SetDebugMode(original)
}

func TestStatelessElement_NormalBuild_NoError(t *testing.T) {
	handler := &testErrorHandler{}
	errors.SetHandler(handler)
	defer errors.SetHandler(nil)

	buildCalled := false
	widget := testStatelessWidget{
		buildFn: func(ctx BuildContext) Widget {
			buildCalled = true
			return nil
		},
	}

	owner := NewBuildOwner()
	element := NewStatelessElement(widget, owner)
	element.Mount(nil, nil)

	if !buildCalled {
		t.Error("expected build to be called")
	}
	if len(handler.buildErrors) != 0 {
		t.Errorf("expected no build errors, got %d", len(handler.buildErrors))
	}
}

func TestStatefulElement_NormalBuild_NoError(t *testing.T) {
	handler := &testErrorHandler{}
	errors.SetHandler(handler)
	defer errors.SetHandler(nil)

	buildCalled := false
	widget := testStatefulWidget{
		createStateFn: func() State {
			return &testState{
				buildFn: func(ctx BuildContext) Widget {
					buildCalled = true
					return nil
				},
			}
		},
	}

	owner := NewBuildOwner()
	element := NewStatefulElement(widget, owner)
	element.Mount(nil, nil)

	if !buildCalled {
		t.Error("expected build to be called")
	}
	if len(handler.buildErrors) != 0 {
		t.Errorf("expected no build errors, got %d", len(handler.buildErrors))
	}
}

// --- Slot-Based Render Tree Management Tests ---

// keyedStatelessWidget is a stateless widget with a configurable key.
type keyedStatelessWidget struct {
	key     any
	buildFn func(BuildContext) Widget
}

func (w keyedStatelessWidget) CreateElement() Element {
	return NewStatelessElement(w, nil)
}

func (w keyedStatelessWidget) Key() any {
	return w.key
}

func (w keyedStatelessWidget) Build(ctx BuildContext) Widget {
	if w.buildFn != nil {
		return w.buildFn(ctx)
	}
	return nil
}

// mockRenderObject is a minimal render object for testing.
type mockRenderObject struct {
	layout.RenderBoxBase
	id       string
	children []layout.RenderObject
}

func (r *mockRenderObject) SetChildren(children []layout.RenderObject) {
	r.children = children
}

func (r *mockRenderObject) Layout(constraints layout.Constraints, parentUsesSize bool) {
	r.RenderBoxBase.Layout(constraints, parentUsesSize)
}

func (r *mockRenderObject) Size() graphics.Size {
	return r.RenderBoxBase.Size()
}

func (r *mockRenderObject) Paint(ctx *layout.PaintContext) {}

func (r *mockRenderObject) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	return false
}

// testMultiChildWidget is a render object widget with multiple children.
type testMultiChildWidget struct {
	key         any
	childWidgets []Widget
}

func (w testMultiChildWidget) CreateElement() Element {
	return NewRenderObjectElement(w, nil)
}

func (w testMultiChildWidget) Key() any {
	return w.key
}

func (w testMultiChildWidget) CreateRenderObject(ctx BuildContext) layout.RenderObject {
	return &mockRenderObject{id: "multi"}
}

func (w testMultiChildWidget) UpdateRenderObject(ctx BuildContext, renderObject layout.RenderObject) {}

func (w testMultiChildWidget) Children() []Widget {
	return w.childWidgets
}

// testSingleChildWidget is a render object widget with one child.
type testSingleChildWidget struct {
	key         any
	childWidget Widget
}

func (w testSingleChildWidget) CreateElement() Element {
	return NewRenderObjectElement(w, nil)
}

func (w testSingleChildWidget) Key() any {
	return w.key
}

func (w testSingleChildWidget) CreateRenderObject(ctx BuildContext) layout.RenderObject {
	return &mockRenderObject{id: "single"}
}

func (w testSingleChildWidget) UpdateRenderObject(ctx BuildContext, renderObject layout.RenderObject) {}

func (w testSingleChildWidget) Child() Widget {
	return w.childWidget
}

// testLeafWidget is a render object widget with no children.
type testLeafWidget struct {
	key any
	id  string
}

func (w testLeafWidget) CreateElement() Element {
	return NewRenderObjectElement(w, nil)
}

func (w testLeafWidget) Key() any {
	return w.key
}

func (w testLeafWidget) CreateRenderObject(ctx BuildContext) layout.RenderObject {
	return &mockRenderObject{id: w.id}
}

func (w testLeafWidget) UpdateRenderObject(ctx BuildContext, renderObject layout.RenderObject) {}

func TestSlotThreading_Mount(t *testing.T) {
	owner := NewBuildOwner()
	widget := testStatelessWidget{}
	element := NewStatelessElement(widget, owner)

	slot := IndexedSlot{Index: 5, PreviousSibling: nil}
	element.Mount(nil, slot)

	if element.Slot() != slot {
		t.Errorf("expected slot %v, got %v", slot, element.Slot())
	}
}

func TestSlotThreading_MountWithParent(t *testing.T) {
	owner := NewBuildOwner()

	parent := NewStatelessElement(testStatelessWidget{}, owner)
	parent.Mount(nil, nil)

	child := NewStatelessElement(testStatelessWidget{}, owner)
	slot := IndexedSlot{Index: 3, PreviousSibling: parent}
	child.Mount(parent, slot)

	if child.Slot() != slot {
		t.Errorf("expected slot %v, got %v", slot, child.Slot())
	}

	// Verify parent-child relationship
	if child.Depth() != parent.Depth()+1 {
		t.Errorf("expected child depth %d, got %d", parent.Depth()+1, child.Depth())
	}
}

func TestUpdateSlot_StatelessElement(t *testing.T) {
	owner := NewBuildOwner()
	widget := testStatelessWidget{}
	element := NewStatelessElement(widget, owner)

	element.Mount(nil, IndexedSlot{Index: 0})

	newSlot := IndexedSlot{Index: 5}
	element.UpdateSlot(newSlot)

	if element.Slot() != newSlot {
		t.Errorf("expected slot %v after UpdateSlot, got %v", newSlot, element.Slot())
	}
}

func TestUpdateSlot_StatefulElement(t *testing.T) {
	owner := NewBuildOwner()
	widget := testStatefulWidget{}
	element := NewStatefulElement(widget, owner)

	element.Mount(nil, IndexedSlot{Index: 0})

	newSlot := IndexedSlot{Index: 10}
	element.UpdateSlot(newSlot)

	if element.Slot() != newSlot {
		t.Errorf("expected slot %v after UpdateSlot, got %v", newSlot, element.Slot())
	}
}

func TestUpdateChild_UpdatesSlot(t *testing.T) {
	owner := NewBuildOwner()
	parent := NewStatelessElement(testStatelessWidget{}, owner)
	parent.Mount(nil, nil)

	// Create initial child with slot 0
	widget := testStatelessWidget{}
	child := updateChild(nil, widget, parent, owner, IndexedSlot{Index: 0})

	if child.Slot() != (IndexedSlot{Index: 0}) {
		t.Errorf("expected initial slot {Index: 0}, got %v", child.Slot())
	}

	// Update with new slot
	updatedChild := updateChild(child, widget, parent, owner, IndexedSlot{Index: 5})

	// Should be same element (canUpdateWidget returns true for same type + nil key)
	if updatedChild != child {
		t.Error("expected same element to be reused")
	}

	if child.Slot() != (IndexedSlot{Index: 5}) {
		t.Errorf("expected updated slot {Index: 5}, got %v", child.Slot())
	}
}

func TestUpdateChildren_TopSync(t *testing.T) {
	owner := NewBuildOwner()
	parent := NewStatelessElement(testStatelessWidget{}, owner)
	parent.Mount(nil, nil)

	// Create initial children
	oldWidgets := []Widget{
		testLeafWidget{id: "a"},
		testLeafWidget{id: "b"},
		testLeafWidget{id: "c"},
	}
	oldChildren := make([]Element, len(oldWidgets))
	for i, w := range oldWidgets {
		oldChildren[i] = inflateWidget(w, owner)
		oldChildren[i].Mount(parent, IndexedSlot{Index: i})
	}

	// Update with same widgets - should sync from top
	newWidgets := []Widget{
		testLeafWidget{id: "a"},
		testLeafWidget{id: "b"},
		testLeafWidget{id: "c"},
	}

	newChildren := updateChildren(parent, oldChildren, newWidgets, owner)

	if len(newChildren) != 3 {
		t.Fatalf("expected 3 children, got %d", len(newChildren))
	}

	// Elements should be reused (same type, no key)
	for i := 0; i < 3; i++ {
		if newChildren[i] != oldChildren[i] {
			t.Errorf("expected child %d to be reused", i)
		}
	}
}

func TestUpdateChildren_KeyedReorder(t *testing.T) {
	owner := NewBuildOwner()
	parent := NewStatelessElement(testStatelessWidget{}, owner)
	parent.Mount(nil, nil)

	// Create initial keyed children: [A, B, C]
	oldWidgets := []Widget{
		testLeafWidget{key: "a", id: "a"},
		testLeafWidget{key: "b", id: "b"},
		testLeafWidget{key: "c", id: "c"},
	}
	oldChildren := make([]Element, len(oldWidgets))
	for i, w := range oldWidgets {
		oldChildren[i] = inflateWidget(w, owner)
		oldChildren[i].Mount(parent, IndexedSlot{Index: i})
	}

	elementA := oldChildren[0]
	elementB := oldChildren[1]
	elementC := oldChildren[2]

	// Reorder to [C, A, B] - should move, not unmount/remount
	newWidgets := []Widget{
		testLeafWidget{key: "c", id: "c"},
		testLeafWidget{key: "a", id: "a"},
		testLeafWidget{key: "b", id: "b"},
	}

	newChildren := updateChildren(parent, oldChildren, newWidgets, owner)

	if len(newChildren) != 3 {
		t.Fatalf("expected 3 children, got %d", len(newChildren))
	}

	// Elements should be reused based on keys
	if newChildren[0] != elementC {
		t.Error("expected element C at position 0")
	}
	if newChildren[1] != elementA {
		t.Error("expected element A at position 1")
	}
	if newChildren[2] != elementB {
		t.Error("expected element B at position 2")
	}

	// Verify slots were updated
	if slot, ok := newChildren[0].Slot().(IndexedSlot); !ok || slot.Index != 0 {
		t.Errorf("expected slot index 0 for position 0, got %v", newChildren[0].Slot())
	}
	if slot, ok := newChildren[1].Slot().(IndexedSlot); !ok || slot.Index != 1 {
		t.Errorf("expected slot index 1 for position 1, got %v", newChildren[1].Slot())
	}
	if slot, ok := newChildren[2].Slot().(IndexedSlot); !ok || slot.Index != 2 {
		t.Errorf("expected slot index 2 for position 2, got %v", newChildren[2].Slot())
	}
}

func TestUpdateChildren_KeyRemoved_Unmounts(t *testing.T) {
	owner := NewBuildOwner()
	parent := NewStatelessElement(testStatelessWidget{}, owner)
	parent.Mount(nil, nil)

	// Create initial keyed children: [A, B, C]
	oldWidgets := []Widget{
		testLeafWidget{key: "a", id: "a"},
		testLeafWidget{key: "b", id: "b"},
		testLeafWidget{key: "c", id: "c"},
	}
	oldChildren := make([]Element, len(oldWidgets))
	for i, w := range oldWidgets {
		oldChildren[i] = inflateWidget(w, owner)
		oldChildren[i].Mount(parent, IndexedSlot{Index: i})
	}

	elementB := oldChildren[1].(*RenderObjectElement)

	// Remove B: [A, C]
	newWidgets := []Widget{
		testLeafWidget{key: "a", id: "a"},
		testLeafWidget{key: "c", id: "c"},
	}

	newChildren := updateChildren(parent, oldChildren, newWidgets, owner)

	if len(newChildren) != 2 {
		t.Fatalf("expected 2 children, got %d", len(newChildren))
	}

	// B should be unmounted
	if elementB.isMounted() {
		t.Error("expected element B to be unmounted")
	}
}

func TestUpdateChildren_KeyAdded_Mounts(t *testing.T) {
	owner := NewBuildOwner()
	parent := NewStatelessElement(testStatelessWidget{}, owner)
	parent.Mount(nil, nil)

	// Create initial keyed children: [A, C]
	oldWidgets := []Widget{
		testLeafWidget{key: "a", id: "a"},
		testLeafWidget{key: "c", id: "c"},
	}
	oldChildren := make([]Element, len(oldWidgets))
	for i, w := range oldWidgets {
		oldChildren[i] = inflateWidget(w, owner)
		oldChildren[i].Mount(parent, IndexedSlot{Index: i})
	}

	// Add B in middle: [A, B, C]
	newWidgets := []Widget{
		testLeafWidget{key: "a", id: "a"},
		testLeafWidget{key: "b", id: "b"},
		testLeafWidget{key: "c", id: "c"},
	}

	newChildren := updateChildren(parent, oldChildren, newWidgets, owner)

	if len(newChildren) != 3 {
		t.Fatalf("expected 3 children, got %d", len(newChildren))
	}

	// New B should be mounted at position 1
	newB := newChildren[1].(*RenderObjectElement)
	if !newB.isMounted() {
		t.Error("expected new element B to be mounted")
	}

	// Verify it's a new element (not reused from old)
	if newChildren[1] == oldChildren[0] || newChildren[1] == oldChildren[1] {
		t.Error("expected new element B to be freshly created")
	}
}

func TestUpdateChildren_BottomSync(t *testing.T) {
	owner := NewBuildOwner()
	parent := NewStatelessElement(testStatelessWidget{}, owner)
	parent.Mount(nil, nil)

	// Create initial keyed children: [A, B, C]
	oldWidgets := []Widget{
		testLeafWidget{key: "a", id: "a"},
		testLeafWidget{key: "b", id: "b"},
		testLeafWidget{key: "c", id: "c"},
	}
	oldChildren := make([]Element, len(oldWidgets))
	for i, w := range oldWidgets {
		oldChildren[i] = inflateWidget(w, owner)
		oldChildren[i].Mount(parent, IndexedSlot{Index: i})
	}

	elementB := oldChildren[1]
	elementC := oldChildren[2]

	// Prepend X: [X, A, B, C] - B and C should sync from bottom
	newWidgets := []Widget{
		testLeafWidget{key: "x", id: "x"},
		testLeafWidget{key: "a", id: "a"},
		testLeafWidget{key: "b", id: "b"},
		testLeafWidget{key: "c", id: "c"},
	}

	newChildren := updateChildren(parent, oldChildren, newWidgets, owner)

	if len(newChildren) != 4 {
		t.Fatalf("expected 4 children, got %d", len(newChildren))
	}

	// B and C should be reused
	if newChildren[2] != elementB {
		t.Error("expected element B to be reused at position 2")
	}
	if newChildren[3] != elementC {
		t.Error("expected element C to be reused at position 3")
	}
}

func TestUpdateChildren_MixedKeyedNonKeyed(t *testing.T) {
	owner := NewBuildOwner()
	parent := NewStatelessElement(testStatelessWidget{}, owner)
	parent.Mount(nil, nil)

	// Create mixed keyed/non-keyed children
	oldWidgets := []Widget{
		testLeafWidget{key: "a", id: "keyed-a"},
		testLeafWidget{id: "non-keyed-1"},
		testLeafWidget{key: "b", id: "keyed-b"},
		testLeafWidget{id: "non-keyed-2"},
	}
	oldChildren := make([]Element, len(oldWidgets))
	for i, w := range oldWidgets {
		oldChildren[i] = inflateWidget(w, owner)
		oldChildren[i].Mount(parent, IndexedSlot{Index: i})
	}

	keyedA := oldChildren[0]
	keyedB := oldChildren[2]

	// Reorder keyed, keep non-keyed in order
	newWidgets := []Widget{
		testLeafWidget{key: "b", id: "keyed-b"},
		testLeafWidget{id: "non-keyed-1"},
		testLeafWidget{key: "a", id: "keyed-a"},
		testLeafWidget{id: "non-keyed-2"},
	}

	newChildren := updateChildren(parent, oldChildren, newWidgets, owner)

	if len(newChildren) != 4 {
		t.Fatalf("expected 4 children, got %d", len(newChildren))
	}

	// Keyed elements should be reused based on keys
	if newChildren[0] != keyedB {
		t.Error("expected keyed B at position 0")
	}
	if newChildren[2] != keyedA {
		t.Error("expected keyed A at position 2")
	}
}

func TestUpdateChildren_EmptyToNonEmpty(t *testing.T) {
	owner := NewBuildOwner()
	parent := NewStatelessElement(testStatelessWidget{}, owner)
	parent.Mount(nil, nil)

	// Start with empty
	oldChildren := []Element{}

	// Add children
	newWidgets := []Widget{
		testLeafWidget{key: "a", id: "a"},
		testLeafWidget{key: "b", id: "b"},
	}

	newChildren := updateChildren(parent, oldChildren, newWidgets, owner)

	if len(newChildren) != 2 {
		t.Fatalf("expected 2 children, got %d", len(newChildren))
	}

	for i, child := range newChildren {
		if !child.(*RenderObjectElement).isMounted() {
			t.Errorf("expected child %d to be mounted", i)
		}
	}
}

func TestUpdateChildren_NonEmptyToEmpty(t *testing.T) {
	owner := NewBuildOwner()
	parent := NewStatelessElement(testStatelessWidget{}, owner)
	parent.Mount(nil, nil)

	// Create initial children
	oldWidgets := []Widget{
		testLeafWidget{key: "a", id: "a"},
		testLeafWidget{key: "b", id: "b"},
	}
	oldChildren := make([]Element, len(oldWidgets))
	for i, w := range oldWidgets {
		oldChildren[i] = inflateWidget(w, owner)
		oldChildren[i].Mount(parent, IndexedSlot{Index: i})
	}

	elementA := oldChildren[0].(*RenderObjectElement)
	elementB := oldChildren[1].(*RenderObjectElement)

	// Remove all children
	newWidgets := []Widget{}

	newChildren := updateChildren(parent, oldChildren, newWidgets, owner)

	if len(newChildren) != 0 {
		t.Fatalf("expected 0 children, got %d", len(newChildren))
	}

	// All old elements should be unmounted
	if elementA.isMounted() {
		t.Error("expected element A to be unmounted")
	}
	if elementB.isMounted() {
		t.Error("expected element B to be unmounted")
	}
}

func TestIndexedSlot_PreviousSibling(t *testing.T) {
	owner := NewBuildOwner()
	parent := NewStatelessElement(testStatelessWidget{}, owner)
	parent.Mount(nil, nil)

	// Create keyed children
	widgets := []Widget{
		testLeafWidget{key: "a", id: "a"},
		testLeafWidget{key: "b", id: "b"},
		testLeafWidget{key: "c", id: "c"},
	}

	children := updateChildren(parent, nil, widgets, owner)

	// Check PreviousSibling chain
	slot0 := children[0].Slot().(IndexedSlot)
	if slot0.PreviousSibling != nil {
		t.Error("expected first child to have nil PreviousSibling")
	}

	slot1 := children[1].Slot().(IndexedSlot)
	if slot1.PreviousSibling != children[0] {
		t.Error("expected second child's PreviousSibling to be first child")
	}

	slot2 := children[2].Slot().(IndexedSlot)
	if slot2.PreviousSibling != children[1] {
		t.Error("expected third child's PreviousSibling to be second child")
	}
}

func TestCanUpdateWidget_SameTypeSameKey(t *testing.T) {
	w1 := testLeafWidget{key: "same", id: "1"}
	w2 := testLeafWidget{key: "same", id: "2"}

	if !canUpdateWidget(w1, w2) {
		t.Error("expected canUpdateWidget to return true for same type and key")
	}
}

func TestCanUpdateWidget_SameTypeDifferentKey(t *testing.T) {
	w1 := testLeafWidget{key: "a", id: "1"}
	w2 := testLeafWidget{key: "b", id: "2"}

	if canUpdateWidget(w1, w2) {
		t.Error("expected canUpdateWidget to return false for different keys")
	}
}

func TestCanUpdateWidget_DifferentType(t *testing.T) {
	w1 := testLeafWidget{id: "leaf"}
	w2 := testStatelessWidget{}

	if canUpdateWidget(w1, w2) {
		t.Error("expected canUpdateWidget to return false for different types")
	}
}

func TestIsComparable(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected bool
	}{
		{"nil", nil, true},
		{"string", "hello", true},
		{"int", 42, true},
		{"struct", struct{ x int }{1}, true},
		{"slice", []int{1, 2, 3}, false},
		{"map", map[string]int{"a": 1}, false},
		{"func", func() {}, false},
		{"pointer", new(int), true},
		{"interface with comparable", interface{}("hello"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isComparable(tt.value)
			if result != tt.expected {
				t.Errorf("isComparable(%v) = %v, expected %v", tt.value, result, tt.expected)
			}
		})
	}
}

// sliceKeyWidget is a widget with a non-comparable key (slice).
type sliceKeyWidget struct {
	key []int
	id  string
}

func (w sliceKeyWidget) CreateElement() Element {
	return NewRenderObjectElement(w, nil)
}

func (w sliceKeyWidget) Key() any {
	return w.key // Non-comparable!
}

func (w sliceKeyWidget) CreateRenderObject(ctx BuildContext) layout.RenderObject {
	return &mockRenderObject{id: w.id}
}

func (w sliceKeyWidget) UpdateRenderObject(ctx BuildContext, renderObject layout.RenderObject) {}

func TestUpdateChildren_NonComparableKey_TreatedAsNonKeyed(t *testing.T) {
	owner := NewBuildOwner()
	parent := NewStatelessElement(testStatelessWidget{}, owner)
	parent.Mount(nil, nil)

	// Create children with non-comparable keys (slices)
	oldWidgets := []Widget{
		sliceKeyWidget{key: []int{1}, id: "a"},
		sliceKeyWidget{key: []int{2}, id: "b"},
	}
	oldChildren := make([]Element, len(oldWidgets))
	for i, w := range oldWidgets {
		oldChildren[i] = inflateWidget(w, owner)
		oldChildren[i].Mount(parent, IndexedSlot{Index: i})
	}

	// Update with same non-comparable keys - should not panic
	newWidgets := []Widget{
		sliceKeyWidget{key: []int{2}, id: "b"},
		sliceKeyWidget{key: []int{1}, id: "a"},
	}

	// This should not panic
	newChildren := updateChildren(parent, oldChildren, newWidgets, owner)

	if len(newChildren) != 2 {
		t.Fatalf("expected 2 children, got %d", len(newChildren))
	}

	// Non-comparable keys are treated as non-keyed, so elements should be reused in order
	// (not by key lookup, which would panic)
}
