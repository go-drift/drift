//go:build android || darwin || ios
// +build android darwin ios

package semantics

import (
	"testing"
)

func TestSemanticsActions_SetHandler(t *testing.T) {
	actions := NewSemanticsActions()

	called := false
	actions.SetHandler(SemanticsActionTap, func(args any) {
		called = true
	})

	if !actions.HasAction(SemanticsActionTap) {
		t.Error("Should have tap action")
	}

	actions.PerformAction(SemanticsActionTap, nil)

	if !called {
		t.Error("Handler should have been called")
	}
}

func TestSemanticsActions_GetHandler(t *testing.T) {
	actions := NewSemanticsActions()

	// No handler set
	handler := actions.GetHandler(SemanticsActionTap)
	if handler != nil {
		t.Error("Should return nil for unset handler")
	}

	// Set handler
	actions.SetHandler(SemanticsActionTap, func(args any) {})

	handler = actions.GetHandler(SemanticsActionTap)
	if handler == nil {
		t.Error("Should return handler after setting")
	}
}

func TestSemanticsActions_HasAction(t *testing.T) {
	actions := NewSemanticsActions()

	if actions.HasAction(SemanticsActionTap) {
		t.Error("Should not have tap action initially")
	}

	actions.SetHandler(SemanticsActionTap, func(args any) {})

	if !actions.HasAction(SemanticsActionTap) {
		t.Error("Should have tap action after setting")
	}
}

func TestSemanticsActions_PerformAction(t *testing.T) {
	actions := NewSemanticsActions()

	// Should return false for missing handler
	handled := actions.PerformAction(SemanticsActionTap, nil)
	if handled {
		t.Error("Should return false for missing handler")
	}

	// Should return true and call handler
	var receivedArgs any
	actions.SetHandler(SemanticsActionTap, func(args any) {
		receivedArgs = args
	})

	testArgs := map[string]int{"test": 42}
	handled = actions.PerformAction(SemanticsActionTap, testArgs)

	if !handled {
		t.Error("Should return true when handler exists")
	}

	if receivedArgs != testArgs {
		t.Error("Args should be passed to handler")
	}
}

func TestSemanticsActions_SupportedActions(t *testing.T) {
	actions := NewSemanticsActions()

	if actions.SupportedActions() != 0 {
		t.Error("Should have no supported actions initially")
	}

	actions.SetHandler(SemanticsActionTap, func(args any) {})
	actions.SetHandler(SemanticsActionLongPress, func(args any) {})

	supported := actions.SupportedActions()

	if supported&SemanticsActionTap == 0 {
		t.Error("Should include tap action")
	}

	if supported&SemanticsActionLongPress == 0 {
		t.Error("Should include long press action")
	}

	if supported&SemanticsActionScrollUp != 0 {
		t.Error("Should not include scroll up action")
	}
}

func TestSemanticsActions_Merge(t *testing.T) {
	actions1 := NewSemanticsActions()
	actions2 := NewSemanticsActions()

	var called1, called2 bool

	actions1.SetHandler(SemanticsActionTap, func(args any) { called1 = true })
	actions2.SetHandler(SemanticsActionTap, func(args any) { called2 = true })
	actions2.SetHandler(SemanticsActionLongPress, func(args any) {})

	actions1.Merge(actions2)

	// actions2's handler should override actions1's for tap
	actions1.PerformAction(SemanticsActionTap, nil)

	if called1 {
		t.Error("Merged handler should override original")
	}

	if !called2 {
		t.Error("Merged handler should be called")
	}

	// Long press should be added
	if !actions1.HasAction(SemanticsActionLongPress) {
		t.Error("Long press should be merged")
	}
}

func TestSemanticsActions_MergeNil(t *testing.T) {
	actions := NewSemanticsActions()
	actions.SetHandler(SemanticsActionTap, func(args any) {})

	// Should not panic
	actions.Merge(nil)

	if !actions.HasAction(SemanticsActionTap) {
		t.Error("Should still have tap action after merging nil")
	}
}

func TestSemanticsActions_Clear(t *testing.T) {
	actions := NewSemanticsActions()
	actions.SetHandler(SemanticsActionTap, func(args any) {})
	actions.SetHandler(SemanticsActionLongPress, func(args any) {})

	actions.Clear()

	if !actions.IsEmpty() {
		t.Error("Should be empty after clear")
	}

	if actions.SupportedActions() != 0 {
		t.Error("Should have no supported actions after clear")
	}
}

func TestSemanticsActions_IsEmpty(t *testing.T) {
	actions := NewSemanticsActions()

	if !actions.IsEmpty() {
		t.Error("New actions should be empty")
	}

	actions.SetHandler(SemanticsActionTap, func(args any) {})

	if actions.IsEmpty() {
		t.Error("Actions with handler should not be empty")
	}
}

func TestSemanticsActions_NilReceiver(t *testing.T) {
	var actions *SemanticsActions

	// Should not panic
	handler := actions.GetHandler(SemanticsActionTap)
	if handler != nil {
		t.Error("Should return nil for nil receiver")
	}

	if actions.HasAction(SemanticsActionTap) {
		t.Error("Should return false for nil receiver")
	}

	handled := actions.PerformAction(SemanticsActionTap, nil)
	if handled {
		t.Error("Should return false for nil receiver")
	}

	if actions.SupportedActions() != 0 {
		t.Error("Should return 0 for nil receiver")
	}

	if !actions.IsEmpty() {
		t.Error("Nil should be considered empty")
	}
}

func TestSemanticsAction_String(t *testing.T) {
	tests := []struct {
		action   SemanticsAction
		expected string
	}{
		{SemanticsActionTap, "tap"},
		{SemanticsActionLongPress, "longPress"},
		{SemanticsActionScrollUp, "scrollUp"},
		{SemanticsActionScrollDown, "scrollDown"},
		{SemanticsActionIncrease, "increase"},
		{SemanticsActionDecrease, "decrease"},
		{SemanticsActionFocus, "focus"},
		{SemanticsActionDismiss, "dismiss"},
	}

	for _, test := range tests {
		result := test.action.String()
		if result != test.expected {
			t.Errorf("Action %d: expected %q, got %q", test.action, test.expected, result)
		}
	}
}
