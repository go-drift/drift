package layout

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
)

// mockRenderObject is a minimal render object for testing typeCountsForList.
type mockRenderObject struct {
	RenderBoxBase
}

func (r *mockRenderObject) Layout(constraints Constraints, parentUsesSize bool) {}
func (r *mockRenderObject) Paint(ctx *PaintContext)                             {}
func (r *mockRenderObject) HitTest(position graphics.Offset, result *HitTestResult) bool {
	return false
}

// anotherRenderObject is a second type for testing sort order.
type anotherRenderObject struct {
	RenderBoxBase
}

func (r *anotherRenderObject) Layout(constraints Constraints, parentUsesSize bool) {}
func (r *anotherRenderObject) Paint(ctx *PaintContext)                             {}
func (r *anotherRenderObject) HitTest(position graphics.Offset, result *HitTestResult) bool {
	return false
}

func TestTypeCountsForList_NilAndEmpty(t *testing.T) {
	if got := typeCountsForList(nil, 5); got != nil {
		t.Errorf("expected nil for nil input, got %v", got)
	}
	if got := typeCountsForList([]RenderObject{}, 5); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

func TestTypeCountsForList_LimitEnforcement(t *testing.T) {
	// Create 3 different types, limit to 2
	items := []RenderObject{
		&mockRenderObject{},
		&anotherRenderObject{},
		&plainTestRenderObject{},
	}
	got := typeCountsForList(items, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
}

func TestTypeCountsForList_DefaultLimit(t *testing.T) {
	items := []RenderObject{&mockRenderObject{}}
	// Zero limit should use default (5)
	got := typeCountsForList(items, 0)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}

	// Negative limit should also use default
	got = typeCountsForList(items, -1)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
}

func TestTypeCountsForList_SortOrder(t *testing.T) {
	// 3x mockRenderObject, 1x anotherRenderObject
	items := []RenderObject{
		&mockRenderObject{},
		&mockRenderObject{},
		&mockRenderObject{},
		&anotherRenderObject{},
	}
	got := typeCountsForList(items, 10)
	if len(got) != 2 {
		t.Fatalf("expected 2 types, got %d", len(got))
	}
	// Highest count first
	if got[0].Type != "mockRenderObject" || got[0].Count != 3 {
		t.Errorf("expected first=mockRenderObject:3, got %s:%d", got[0].Type, got[0].Count)
	}
	if got[1].Type != "anotherRenderObject" || got[1].Count != 1 {
		t.Errorf("expected second=anotherRenderObject:1, got %s:%d", got[1].Type, got[1].Count)
	}
}

func TestTypeCountsForList_TieBreakByName(t *testing.T) {
	// Equal counts: should sort alphabetically by type name
	items := []RenderObject{
		&mockRenderObject{},
		&anotherRenderObject{},
	}
	got := typeCountsForList(items, 10)
	if len(got) != 2 {
		t.Fatalf("expected 2 types, got %d", len(got))
	}
	// "anotherRenderObject" < "mockRenderObject" alphabetically
	if got[0].Type != "anotherRenderObject" {
		t.Errorf("expected first=anotherRenderObject (alphabetical tie-break), got %s", got[0].Type)
	}
	if got[1].Type != "mockRenderObject" {
		t.Errorf("expected second=mockRenderObject, got %s", got[1].Type)
	}
}

func TestRenderTypeName_Nil(t *testing.T) {
	got := renderTypeName(nil)
	if got != "<nil>" {
		t.Errorf("expected <nil>, got %s", got)
	}
}

func TestRenderTypeName_PointerUnwrapped(t *testing.T) {
	obj := &mockRenderObject{}
	got := renderTypeName(obj)
	if got != "mockRenderObject" {
		t.Errorf("expected mockRenderObject, got %s", got)
	}
}

// plainTestRenderObject is a third type for testing limit enforcement.
type plainTestRenderObject struct {
	RenderBoxBase
}

func (r *plainTestRenderObject) Layout(constraints Constraints, parentUsesSize bool) {}
func (r *plainTestRenderObject) Paint(ctx *PaintContext)                             {}
func (r *plainTestRenderObject) HitTest(position graphics.Offset, result *HitTestResult) bool {
	return false
}
