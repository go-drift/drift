package focus

import (
	"testing"
)

// --- FocusRect ---

func TestFocusRect_Center(t *testing.T) {
	r := FocusRect{Left: 0, Top: 0, Right: 100, Bottom: 200}
	x, y := r.Center()
	if x != 50 || y != 100 {
		t.Errorf("Center() = (%v, %v), want (50, 100)", x, y)
	}
}

func TestFocusRect_Center_Offset(t *testing.T) {
	r := FocusRect{Left: 10, Top: 20, Right: 30, Bottom: 40}
	x, y := r.Center()
	if x != 20 || y != 30 {
		t.Errorf("Center() = (%v, %v), want (20, 30)", x, y)
	}
}

func TestFocusRect_IsValid(t *testing.T) {
	tests := []struct {
		name string
		rect FocusRect
		want bool
	}{
		{"positive dimensions", FocusRect{0, 0, 100, 100}, true},
		{"zero width", FocusRect{0, 0, 0, 100}, false},
		{"zero height", FocusRect{0, 0, 100, 0}, false},
		{"zero rect", FocusRect{}, false},
		{"negative width", FocusRect{100, 0, 50, 100}, false},
		{"negative height", FocusRect{0, 100, 100, 50}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rect.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- FocusNode ---

func TestFocusNode_canReceiveFocus(t *testing.T) {
	tests := []struct {
		name string
		node *FocusNode
		want bool
	}{
		{"nil node", nil, false},
		{"default node", &FocusNode{}, false},
		{"can request", &FocusNode{CanRequestFocus: true}, true},
		{"skip traversal", &FocusNode{CanRequestFocus: true, SkipTraversal: true}, false},
		{"skip without request", &FocusNode{SkipTraversal: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.node.canReceiveFocus(); got != tt.want {
				t.Errorf("canReceiveFocus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func resetFocusManager() {
	focusManager.PrimaryFocus = nil
	focusManager.RootScope = &FocusScopeNode{}
}

func TestFocusNode_RequestFocus(t *testing.T) {
	resetFocusManager()

	node := &FocusNode{CanRequestFocus: true}
	node.RequestFocus()

	if !node.HasPrimaryFocus() {
		t.Error("expected HasPrimaryFocus after RequestFocus")
	}
	if !node.HasFocus() {
		t.Error("expected HasFocus after RequestFocus")
	}
	if GetFocusManager().PrimaryFocus != node {
		t.Error("expected manager.PrimaryFocus to be set")
	}
}

func TestFocusNode_RequestFocus_Unfocusable(t *testing.T) {
	resetFocusManager()

	node := &FocusNode{CanRequestFocus: false}
	node.RequestFocus()

	if node.HasPrimaryFocus() {
		t.Error("unfocusable node should not gain focus")
	}
	if GetFocusManager().PrimaryFocus != nil {
		t.Error("manager should have no primary focus")
	}
}

func TestFocusNode_Unfocus(t *testing.T) {
	resetFocusManager()

	node := &FocusNode{CanRequestFocus: true}
	node.RequestFocus()
	node.Unfocus()

	if node.HasPrimaryFocus() {
		t.Error("expected no primary focus after Unfocus")
	}
	if node.HasFocus() {
		t.Error("expected no focus after Unfocus")
	}
	if GetFocusManager().PrimaryFocus != nil {
		t.Error("manager.PrimaryFocus should be nil after Unfocus")
	}
}

func TestFocusNode_Unfocus_NotPrimary(t *testing.T) {
	resetFocusManager()

	a := &FocusNode{CanRequestFocus: true}
	b := &FocusNode{CanRequestFocus: true}
	a.RequestFocus()

	// Unfocusing b should not affect a
	b.Unfocus()
	if !a.HasPrimaryFocus() {
		t.Error("a should still have focus")
	}
}

func TestFocusNode_OnFocusChange(t *testing.T) {
	resetFocusManager()

	var calls []bool
	node := &FocusNode{
		CanRequestFocus: true,
		OnFocusChange:   func(hasFocus bool) { calls = append(calls, hasFocus) },
	}

	node.RequestFocus()
	node.Unfocus()

	if len(calls) != 2 {
		t.Fatalf("expected 2 callback calls, got %d", len(calls))
	}
	if calls[0] != true {
		t.Error("first callback should be true (gained focus)")
	}
	if calls[1] != false {
		t.Error("second callback should be false (lost focus)")
	}
}

func TestFocusNode_SwitchFocus(t *testing.T) {
	resetFocusManager()

	a := &FocusNode{CanRequestFocus: true}
	b := &FocusNode{CanRequestFocus: true}

	a.RequestFocus()
	if !a.HasPrimaryFocus() {
		t.Error("a should have focus")
	}

	b.RequestFocus()
	if a.HasPrimaryFocus() {
		t.Error("a should have lost focus")
	}
	if !b.HasPrimaryFocus() {
		t.Error("b should have focus")
	}
}

// --- FocusScopeNode ---

func TestFocusScopeNode_SetFirstFocus(t *testing.T) {
	resetFocusManager()

	a := &FocusNode{CanRequestFocus: false}
	b := &FocusNode{CanRequestFocus: true}
	c := &FocusNode{CanRequestFocus: true}

	scope := &FocusScopeNode{Children: []*FocusNode{a, b, c}}
	scope.SetFirstFocus()

	if !b.HasPrimaryFocus() {
		t.Error("first focusable child (b) should have focus")
	}
	if scope.FocusedChild != b {
		t.Error("FocusedChild should be b")
	}
}

func TestFocusScopeNode_SetFirstFocus_Empty(t *testing.T) {
	resetFocusManager()
	scope := &FocusScopeNode{}
	scope.SetFirstFocus() // should not panic
}

func TestFocusScopeNode_SetFirstFocus_AllSkipped(t *testing.T) {
	resetFocusManager()

	a := &FocusNode{CanRequestFocus: true, SkipTraversal: true}
	b := &FocusNode{CanRequestFocus: false}
	scope := &FocusScopeNode{Children: []*FocusNode{a, b}}
	scope.SetFirstFocus()

	if GetFocusManager().PrimaryFocus != nil {
		t.Error("no node should have focus when all are skipped/unfocusable")
	}
}

func TestFocusScopeNode_SetFirstFocus_Nil(t *testing.T) {
	var scope *FocusScopeNode
	scope.SetFirstFocus() // should not panic
}

// --- FocusManager.MoveFocus ---

func TestFocusManager_MoveFocus_Forward(t *testing.T) {
	resetFocusManager()

	a := &FocusNode{CanRequestFocus: true}
	b := &FocusNode{CanRequestFocus: true}
	c := &FocusNode{CanRequestFocus: true}

	m := GetFocusManager()
	m.RootScope.Children = []*FocusNode{a, b, c}
	m.setPrimaryFocus(a)

	if !m.MoveFocus(1) {
		t.Error("MoveFocus(1) should succeed")
	}
	if !b.HasPrimaryFocus() {
		t.Error("b should have focus after MoveFocus(1)")
	}
}

func TestFocusManager_MoveFocus_Backward(t *testing.T) {
	resetFocusManager()

	a := &FocusNode{CanRequestFocus: true}
	b := &FocusNode{CanRequestFocus: true}
	c := &FocusNode{CanRequestFocus: true}

	m := GetFocusManager()
	m.RootScope.Children = []*FocusNode{a, b, c}
	m.setPrimaryFocus(b)

	if !m.MoveFocus(-1) {
		t.Error("MoveFocus(-1) should succeed")
	}
	if !a.HasPrimaryFocus() {
		t.Error("a should have focus after MoveFocus(-1)")
	}
}

func TestFocusManager_MoveFocus_Wraps(t *testing.T) {
	resetFocusManager()

	a := &FocusNode{CanRequestFocus: true}
	b := &FocusNode{CanRequestFocus: true}

	m := GetFocusManager()
	m.RootScope.Children = []*FocusNode{a, b}
	m.setPrimaryFocus(b)

	if !m.MoveFocus(1) {
		t.Error("MoveFocus should wrap around")
	}
	if !a.HasPrimaryFocus() {
		t.Error("a should have focus after wrapping")
	}
}

func TestFocusManager_MoveFocus_SkipsUnfocusable(t *testing.T) {
	resetFocusManager()

	a := &FocusNode{CanRequestFocus: true}
	skip := &FocusNode{CanRequestFocus: false}
	c := &FocusNode{CanRequestFocus: true}

	m := GetFocusManager()
	m.RootScope.Children = []*FocusNode{a, skip, c}
	m.setPrimaryFocus(a)

	if !m.MoveFocus(1) {
		t.Error("MoveFocus should succeed")
	}
	if !c.HasPrimaryFocus() {
		t.Error("c should have focus, skipping unfocusable node")
	}
}

func TestFocusManager_MoveFocus_AllUnfocusable(t *testing.T) {
	resetFocusManager()

	a := &FocusNode{CanRequestFocus: false}
	b := &FocusNode{CanRequestFocus: false}

	m := GetFocusManager()
	m.RootScope.Children = []*FocusNode{a, b}

	if m.MoveFocus(1) {
		t.Error("MoveFocus should return false when no focusable nodes exist")
	}
}

func TestFocusManager_MoveFocus_EmptyScope(t *testing.T) {
	resetFocusManager()

	m := GetFocusManager()
	if m.MoveFocus(1) {
		t.Error("MoveFocus should return false on empty scope")
	}
}

func TestFocusManager_MoveFocus_NilScope(t *testing.T) {
	resetFocusManager()
	m := GetFocusManager()
	m.RootScope = nil
	if m.MoveFocus(1) {
		t.Error("MoveFocus should return false on nil scope")
	}
}

// --- FocusScopeNode.FocusInDirection ---

type staticRect struct{ rect FocusRect }

func (s staticRect) FocusRect() FocusRect { return s.rect }

func TestFocusScopeNode_FocusInDirection(t *testing.T) {
	resetFocusManager()

	// Layout:
	//   top (50, 25)
	//   left (25, 75)  center (50, 75)  right (75, 75)
	//   bottom (50, 125)
	center := &FocusNode{
		CanRequestFocus: true,
		Rect:            staticRect{FocusRect{40, 65, 60, 85}},
	}
	top := &FocusNode{
		CanRequestFocus: true,
		Rect:            staticRect{FocusRect{40, 15, 60, 35}},
	}
	bottom := &FocusNode{
		CanRequestFocus: true,
		Rect:            staticRect{FocusRect{40, 115, 60, 135}},
	}
	left := &FocusNode{
		CanRequestFocus: true,
		Rect:            staticRect{FocusRect{15, 65, 35, 85}},
	}
	right := &FocusNode{
		CanRequestFocus: true,
		Rect:            staticRect{FocusRect{65, 65, 85, 85}},
	}

	scope := &FocusScopeNode{
		Children: []*FocusNode{center, top, bottom, left, right},
	}

	m := GetFocusManager()
	m.RootScope = scope
	m.setPrimaryFocus(center)

	// Up should go to top
	scope.FocusInDirection(TraversalDirectionUp)
	if !top.HasPrimaryFocus() {
		t.Error("up from center should focus top")
	}

	m.setPrimaryFocus(center)
	scope.FocusInDirection(TraversalDirectionDown)
	if !bottom.HasPrimaryFocus() {
		t.Error("down from center should focus bottom")
	}

	m.setPrimaryFocus(center)
	scope.FocusInDirection(TraversalDirectionLeft)
	if !left.HasPrimaryFocus() {
		t.Error("left from center should focus left")
	}

	m.setPrimaryFocus(center)
	scope.FocusInDirection(TraversalDirectionRight)
	if !right.HasPrimaryFocus() {
		t.Error("right from center should focus right")
	}
}

func TestFocusScopeNode_FocusInDirection_NoCurrent(t *testing.T) {
	resetFocusManager()

	a := &FocusNode{CanRequestFocus: true}
	scope := &FocusScopeNode{Children: []*FocusNode{a}}
	m := GetFocusManager()
	m.RootScope = scope

	scope.FocusInDirection(TraversalDirectionDown)

	// With no current focus, SetFirstFocus should be called
	if !a.HasPrimaryFocus() {
		t.Error("expected first child to gain focus when no current focus")
	}
}

// --- Helper functions ---

func TestIsInDirection(t *testing.T) {
	source := FocusRect{40, 40, 60, 60}

	tests := []struct {
		name      string
		target    FocusRect
		direction TraversalDirection
		want      bool
	}{
		{"up yes", FocusRect{40, 10, 60, 30}, TraversalDirectionUp, true},
		{"up no", FocusRect{40, 60, 60, 80}, TraversalDirectionUp, false},
		{"down yes", FocusRect{40, 60, 60, 80}, TraversalDirectionDown, true},
		{"down no", FocusRect{40, 10, 60, 30}, TraversalDirectionDown, false},
		{"left yes", FocusRect{10, 40, 30, 60}, TraversalDirectionLeft, true},
		{"left no", FocusRect{60, 40, 80, 60}, TraversalDirectionLeft, false},
		{"right yes", FocusRect{60, 40, 80, 60}, TraversalDirectionRight, true},
		{"right no", FocusRect{10, 40, 30, 60}, TraversalDirectionRight, false},
		{"same center", FocusRect{40, 40, 60, 60}, TraversalDirectionUp, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInDirection(source, tt.target, tt.direction); got != tt.want {
				t.Errorf("isInDirection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirectionalScore_PreferAligned(t *testing.T) {
	source := FocusRect{40, 40, 60, 60}

	// Aligned target (same x center, below)
	aligned := FocusRect{40, 80, 60, 100}
	// Offset target (different x center, below, same primary distance)
	offset := FocusRect{80, 80, 100, 100}

	scoreAligned := directionalScore(source, aligned, TraversalDirectionDown)
	scoreOffset := directionalScore(source, offset, TraversalDirectionDown)

	if scoreAligned >= scoreOffset {
		t.Errorf("aligned score (%v) should be less than offset score (%v)", scoreAligned, scoreOffset)
	}
}

func TestWrapIndex(t *testing.T) {
	tests := []struct {
		index, count, want int
	}{
		{0, 3, 0},
		{1, 3, 1},
		{3, 3, 0},
		{-1, 3, 2},
		{-3, 3, 0},
		{5, 3, 2},
		{-4, 3, 2},
	}
	for _, tt := range tests {
		got := wrapIndex(tt.index, tt.count)
		if got != tt.want {
			t.Errorf("wrapIndex(%d, %d) = %d, want %d", tt.index, tt.count, got, tt.want)
		}
	}
}
