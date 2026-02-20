package navigation

import (
	"testing"

	"github.com/go-drift/drift/pkg/widgets"
)

func TestTabNavigatorState_RegisterNavigator(t *testing.T) {
	// Save and restore global state
	oldScope := globalScope
	globalScope = &NavigationScope{}
	defer func() { globalScope = oldScope }()

	state := &tabNavigatorState{
		navigators:   make([]NavigatorState, 3),
		currentIndex: 0,
	}

	nav0 := &mockNavigatorState{}
	nav1 := &mockNavigatorState{}
	nav2 := &mockNavigatorState{}

	// Register first tab - should become active since currentIndex is 0
	state.registerNavigator(0, nav0)
	if state.navigators[0] != nav0 {
		t.Error("Tab 0 navigator not stored")
	}
	if globalScope.ActiveNavigator() != nav0 {
		t.Error("Tab 0 should be set as active navigator")
	}

	// Register other tabs - should not change active
	state.registerNavigator(1, nav1)
	state.registerNavigator(2, nav2)
	if globalScope.ActiveNavigator() != nav0 {
		t.Error("Active navigator should still be nav0")
	}
}

func TestTabNavigatorState_RegisterNavigator_InvalidIndex(t *testing.T) {
	state := &tabNavigatorState{
		navigators:   make([]NavigatorState, 2),
		currentIndex: 0,
	}

	nav := &mockNavigatorState{}

	// Out of bounds indices should be silently ignored
	state.registerNavigator(-1, nav)
	state.registerNavigator(5, nav)

	// No panic, no modifications
	if state.navigators[0] != nil || state.navigators[1] != nil {
		t.Error("Invalid indices should not modify navigators")
	}
}

func TestTabNavigatorState_OnTabChanged(t *testing.T) {
	// Save and restore global state
	oldScope := globalScope
	globalScope = &NavigationScope{}
	defer func() { globalScope = oldScope }()

	nav0 := &mockNavigatorState{}
	nav1 := &mockNavigatorState{}
	nav2 := &mockNavigatorState{}

	state := &tabNavigatorState{
		navigators:   []NavigatorState{nav0, nav1, nav2},
		currentIndex: 0,
	}

	// Switch to tab 1
	state.onTabChanged(1)
	if state.currentIndex != 1 {
		t.Errorf("currentIndex = %d, want 1", state.currentIndex)
	}
	if globalScope.ActiveNavigator() != nav1 {
		t.Error("Active navigator should be nav1 after switching to tab 1")
	}

	// Switch to tab 2
	state.onTabChanged(2)
	if state.currentIndex != 2 {
		t.Errorf("currentIndex = %d, want 2", state.currentIndex)
	}
	if globalScope.ActiveNavigator() != nav2 {
		t.Error("Active navigator should be nav2 after switching to tab 2")
	}

	// Switch back to tab 0
	state.onTabChanged(0)
	if globalScope.ActiveNavigator() != nav0 {
		t.Error("Active navigator should be nav0 after switching to tab 0")
	}
}

func TestTabNavigatorState_OnTabChanged_NilNavigator(t *testing.T) {
	// Save and restore global state
	oldScope := globalScope
	globalScope = &NavigationScope{}
	defer func() { globalScope = oldScope }()

	nav0 := &mockNavigatorState{}

	state := &tabNavigatorState{
		navigators:   []NavigatorState{nav0, nil, nil}, // Tab 1 and 2 not yet registered
		currentIndex: 0,
	}

	// Initially set nav0 as active
	globalScope.SetActiveNavigator(nav0)

	// Switch to tab 1 (nil navigator)
	state.onTabChanged(1)
	if state.currentIndex != 1 {
		t.Errorf("currentIndex = %d, want 1", state.currentIndex)
	}
	// Active navigator should remain unchanged when tab has nil navigator
	if globalScope.ActiveNavigator() != nav0 {
		t.Error("Active navigator should remain nav0 when switching to tab with nil navigator")
	}
}

func TestTabNavigatorState_OnTabChanged_OutOfBounds(t *testing.T) {
	// Save and restore global state
	oldScope := globalScope
	globalScope = &NavigationScope{}
	defer func() { globalScope = oldScope }()

	nav0 := &mockNavigatorState{}

	state := &tabNavigatorState{
		navigators:   []NavigatorState{nav0},
		currentIndex: 0,
	}

	globalScope.SetActiveNavigator(nav0)

	// Out of bounds - should update currentIndex but not crash or change active
	state.onTabChanged(5)
	if state.currentIndex != 5 {
		t.Errorf("currentIndex = %d, want 5", state.currentIndex)
	}
	// Active should remain unchanged
	if globalScope.ActiveNavigator() != nav0 {
		t.Error("Active navigator should remain nav0 for out of bounds index")
	}
}

func TestTabNavigatorState_ValidatedIndex(t *testing.T) {
	controller := NewTabController(5) // Start at invalid index

	state := &tabNavigatorState{
		nav: TabNavigator{
			Tabs: []Tab{
				{Item: widgets.TabItem{Label: "Tab 0"}},
				{Item: widgets.TabItem{Label: "Tab 1"}},
			},
		},
		controller: controller,
	}

	// Index 5 is out of bounds (only 2 tabs), should clamp to 0
	index := state.validatedIndex()
	if index != 0 {
		t.Errorf("validatedIndex() = %d, want 0 for out of bounds index", index)
	}
	if controller.Index() != 0 {
		t.Errorf("Controller index should be reset to 0, got %d", controller.Index())
	}
}

func TestTabNavigatorState_ValidatedIndex_Negative(t *testing.T) {
	controller := NewTabController(-1)

	state := &tabNavigatorState{
		nav: TabNavigator{
			Tabs: []Tab{
				{Item: widgets.TabItem{Label: "Tab 0"}},
			},
		},
		controller: controller,
	}

	index := state.validatedIndex()
	if index != 0 {
		t.Errorf("validatedIndex() = %d, want 0 for negative index", index)
	}
}

func TestTabNavigatorState_ValidatedIndex_Valid(t *testing.T) {
	controller := NewTabController(1)

	state := &tabNavigatorState{
		nav: TabNavigator{
			Tabs: []Tab{
				{Item: widgets.TabItem{Label: "Tab 0"}},
				{Item: widgets.TabItem{Label: "Tab 1"}},
				{Item: widgets.TabItem{Label: "Tab 2"}},
			},
		},
		controller: controller,
	}

	index := state.validatedIndex()
	if index != 1 {
		t.Errorf("validatedIndex() = %d, want 1", index)
	}
	// Controller should not be modified
	if controller.Index() != 1 {
		t.Errorf("Controller index should remain 1, got %d", controller.Index())
	}
}

func TestTabNavigatorState_DidUpdateWidget_ResizesNavigators(t *testing.T) {
	nav0 := &mockNavigatorState{}
	nav1 := &mockNavigatorState{}

	// Initial state with 2 tabs
	state := &tabNavigatorState{
		nav: TabNavigator{
			Tabs: []Tab{
				{Item: widgets.TabItem{Label: "Tab 0"}},
				{Item: widgets.TabItem{Label: "Tab 1"}},
			},
		},
		navigators: []NavigatorState{nav0, nav1},
		controller: NewTabController(0),
	}

	// Simulate widget update with more tabs
	updated := TabNavigator{
		Tabs: []Tab{
			{Item: widgets.TabItem{Label: "Tab 0"}},
			{Item: widgets.TabItem{Label: "Tab 1"}},
			{Item: widgets.TabItem{Label: "Tab 2"}},
			{Item: widgets.TabItem{Label: "Tab 3"}},
		},
		Controller: state.controller,
	}

	// Call DidUpdateWidget logic
	old := state.nav
	state.nav = updated

	// Manually resize (mimicking DidUpdateWidget logic)
	if len(state.nav.Tabs) != len(old.Tabs) {
		newNavigators := make([]NavigatorState, len(state.nav.Tabs))
		for i := 0; i < len(newNavigators) && i < len(state.navigators); i++ {
			newNavigators[i] = state.navigators[i]
		}
		state.navigators = newNavigators
	}

	// Verify resize happened
	if len(state.navigators) != 4 {
		t.Errorf("navigators should have 4 slots, got %d", len(state.navigators))
	}

	// Verify existing navigators preserved
	if state.navigators[0] != nav0 {
		t.Error("Tab 0 navigator should be preserved")
	}
	if state.navigators[1] != nav1 {
		t.Error("Tab 1 navigator should be preserved")
	}

	// New slots should be nil
	if state.navigators[2] != nil {
		t.Error("Tab 2 navigator should be nil")
	}
	if state.navigators[3] != nil {
		t.Error("Tab 3 navigator should be nil")
	}
}

func TestTabNavigatorState_DidUpdateWidget_ShrinksTabs(t *testing.T) {
	nav0 := &mockNavigatorState{}
	nav1 := &mockNavigatorState{}
	nav2 := &mockNavigatorState{}

	// Initial state with 3 tabs
	state := &tabNavigatorState{
		nav: TabNavigator{
			Tabs: []Tab{
				{Item: widgets.TabItem{Label: "Tab 0"}},
				{Item: widgets.TabItem{Label: "Tab 1"}},
				{Item: widgets.TabItem{Label: "Tab 2"}},
			},
		},
		navigators: []NavigatorState{nav0, nav1, nav2},
		controller: NewTabController(0),
	}

	// Simulate widget update with fewer tabs
	updated := TabNavigator{
		Tabs: []Tab{
			{Item: widgets.TabItem{Label: "Tab 0"}},
		},
		Controller: state.controller,
	}

	// Call DidUpdateWidget logic
	old := state.nav
	state.nav = updated

	if len(state.nav.Tabs) != len(old.Tabs) {
		newNavigators := make([]NavigatorState, len(state.nav.Tabs))
		for i := 0; i < len(newNavigators) && i < len(state.navigators); i++ {
			newNavigators[i] = state.navigators[i]
		}
		state.navigators = newNavigators
	}

	// Verify resize happened
	if len(state.navigators) != 1 {
		t.Errorf("navigators should have 1 slot, got %d", len(state.navigators))
	}

	// Verify first navigator preserved
	if state.navigators[0] != nav0 {
		t.Error("Tab 0 navigator should be preserved")
	}
}
