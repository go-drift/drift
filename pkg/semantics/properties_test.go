package semantics

import "testing"

// --- SemanticsRole.String ---

func TestSemanticsRole_String(t *testing.T) {
	tests := []struct {
		role SemanticsRole
		want string
	}{
		{SemanticsRoleNone, "none"},
		{SemanticsRoleButton, "button"},
		{SemanticsRoleCheckbox, "checkbox"},
		{SemanticsRoleRadio, "radio"},
		{SemanticsRoleSwitch, "switch"},
		{SemanticsRoleTextField, "textField"},
		{SemanticsRoleLink, "link"},
		{SemanticsRoleImage, "image"},
		{SemanticsRoleSlider, "slider"},
		{SemanticsRoleProgressIndicator, "progressIndicator"},
		{SemanticsRoleTab, "tab"},
		{SemanticsRoleTabBar, "tabBar"},
		{SemanticsRoleList, "list"},
		{SemanticsRoleListItem, "listItem"},
		{SemanticsRoleScrollView, "scrollView"},
		{SemanticsRoleHeader, "header"},
		{SemanticsRoleAlert, "alert"},
		{SemanticsRoleMenu, "menu"},
		{SemanticsRoleMenuItem, "menuItem"},
		{SemanticsRolePopup, "popup"},
		{SemanticsRole(999), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.role.String(); got != tt.want {
				t.Errorf("SemanticsRole(%d).String() = %q, want %q", tt.role, got, tt.want)
			}
		})
	}
}

// --- SemanticsFlag ---

func TestSemanticsFlag_Has(t *testing.T) {
	f := SemanticsIsButton | SemanticsIsEnabled
	if !f.Has(SemanticsIsButton) {
		t.Error("should have IsButton")
	}
	if !f.Has(SemanticsIsEnabled) {
		t.Error("should have IsEnabled")
	}
	if f.Has(SemanticsIsFocused) {
		t.Error("should not have IsFocused")
	}
}

func TestSemanticsFlag_Set(t *testing.T) {
	var f SemanticsFlag
	f = f.Set(SemanticsIsButton)
	if !f.Has(SemanticsIsButton) {
		t.Error("Set should add the flag")
	}

	f = f.Set(SemanticsIsEnabled)
	if !f.Has(SemanticsIsButton) || !f.Has(SemanticsIsEnabled) {
		t.Error("Set should preserve existing flags")
	}
}

func TestSemanticsFlag_Clear(t *testing.T) {
	f := SemanticsIsButton | SemanticsIsEnabled | SemanticsIsFocused
	f = f.Clear(SemanticsIsEnabled)

	if f.Has(SemanticsIsEnabled) {
		t.Error("Clear should remove the flag")
	}
	if !f.Has(SemanticsIsButton) {
		t.Error("Clear should not affect other flags (IsButton)")
	}
	if !f.Has(SemanticsIsFocused) {
		t.Error("Clear should not affect other flags (IsFocused)")
	}
}

// --- SemanticsProperties.IsEmpty ---

func TestSemanticsProperties_IsEmpty(t *testing.T) {
	if !(SemanticsProperties{}).IsEmpty() {
		t.Error("zero-value SemanticsProperties should be empty")
	}
}

func TestSemanticsProperties_IsEmpty_Label(t *testing.T) {
	p := SemanticsProperties{Label: "x"}
	if p.IsEmpty() {
		t.Error("properties with Label should not be empty")
	}
}

func TestSemanticsProperties_IsEmpty_Value(t *testing.T) {
	p := SemanticsProperties{Value: "x"}
	if p.IsEmpty() {
		t.Error("properties with Value should not be empty")
	}
}

func TestSemanticsProperties_IsEmpty_Hint(t *testing.T) {
	p := SemanticsProperties{Hint: "x"}
	if p.IsEmpty() {
		t.Error("properties with Hint should not be empty")
	}
}

func TestSemanticsProperties_IsEmpty_Tooltip(t *testing.T) {
	p := SemanticsProperties{Tooltip: "x"}
	if p.IsEmpty() {
		t.Error("properties with Tooltip should not be empty")
	}
}

func TestSemanticsProperties_IsEmpty_Role(t *testing.T) {
	p := SemanticsProperties{Role: SemanticsRoleButton}
	if p.IsEmpty() {
		t.Error("properties with Role should not be empty")
	}
}

func TestSemanticsProperties_IsEmpty_Flags(t *testing.T) {
	p := SemanticsProperties{Flags: SemanticsIsButton}
	if p.IsEmpty() {
		t.Error("properties with Flags should not be empty")
	}
}

func TestSemanticsProperties_IsEmpty_CurrentValue(t *testing.T) {
	v := 0.5
	p := SemanticsProperties{CurrentValue: &v}
	if p.IsEmpty() {
		t.Error("properties with CurrentValue should not be empty")
	}
}

func TestSemanticsProperties_IsEmpty_HeadingLevel(t *testing.T) {
	p := SemanticsProperties{HeadingLevel: 1}
	if p.IsEmpty() {
		t.Error("properties with HeadingLevel should not be empty")
	}
}

func TestSemanticsProperties_IsEmpty_CustomActions(t *testing.T) {
	p := SemanticsProperties{
		CustomActions: []CustomSemanticsAction{{ID: 1, Label: "do"}},
	}
	if p.IsEmpty() {
		t.Error("properties with CustomActions should not be empty")
	}
}

// --- SemanticsProperties.Merge ---

func TestSemanticsProperties_Merge_Override(t *testing.T) {
	base := SemanticsProperties{Label: "base", Value: "v1"}
	other := SemanticsProperties{Label: "override"}

	result := base.Merge(other)

	if result.Label != "override" {
		t.Errorf("Label = %q, want %q", result.Label, "override")
	}
	if result.Value != "v1" {
		t.Errorf("Value = %q, want %q (preserved from base)", result.Value, "v1")
	}
}

func TestSemanticsProperties_Merge_FlagsOR(t *testing.T) {
	base := SemanticsProperties{Flags: SemanticsIsButton}
	other := SemanticsProperties{Flags: SemanticsIsEnabled}

	result := base.Merge(other)

	if !result.Flags.Has(SemanticsIsButton) {
		t.Error("merged flags should contain IsButton")
	}
	if !result.Flags.Has(SemanticsIsEnabled) {
		t.Error("merged flags should contain IsEnabled")
	}
}

func TestSemanticsProperties_Merge_CustomActionsAppend(t *testing.T) {
	base := SemanticsProperties{
		CustomActions: []CustomSemanticsAction{{ID: 1, Label: "a"}},
	}
	other := SemanticsProperties{
		CustomActions: []CustomSemanticsAction{{ID: 2, Label: "b"}},
	}

	result := base.Merge(other)

	if len(result.CustomActions) != 2 {
		t.Fatalf("expected 2 custom actions, got %d", len(result.CustomActions))
	}
	if result.CustomActions[0].ID != 1 || result.CustomActions[1].ID != 2 {
		t.Error("custom actions should be appended in order")
	}
}

func TestSemanticsProperties_Merge_NilPointerPreserved(t *testing.T) {
	v := 1.0
	base := SemanticsProperties{CurrentValue: &v}
	other := SemanticsProperties{} // CurrentValue is nil

	result := base.Merge(other)

	if result.CurrentValue != &v {
		t.Error("nil pointer in other should preserve base value")
	}
}

func TestSemanticsProperties_Merge_PointerOverride(t *testing.T) {
	v1 := 1.0
	v2 := 2.0
	base := SemanticsProperties{CurrentValue: &v1, MinValue: &v1}
	other := SemanticsProperties{CurrentValue: &v2}

	result := base.Merge(other)

	if result.CurrentValue != &v2 {
		t.Error("non-nil pointer in other should override base")
	}
	if result.MinValue != &v1 {
		t.Error("unset pointer fields should be preserved")
	}
}

func TestSemanticsProperties_Merge_Role(t *testing.T) {
	base := SemanticsProperties{Role: SemanticsRoleButton}
	other := SemanticsProperties{Role: SemanticsRoleCheckbox}

	result := base.Merge(other)
	if result.Role != SemanticsRoleCheckbox {
		t.Error("non-zero Role in other should override base")
	}
}

func TestSemanticsProperties_Merge_RolePreserved(t *testing.T) {
	base := SemanticsProperties{Role: SemanticsRoleButton}
	other := SemanticsProperties{} // Role is SemanticsRoleNone (zero)

	result := base.Merge(other)
	if result.Role != SemanticsRoleButton {
		t.Error("zero Role in other should preserve base")
	}
}

func TestSemanticsProperties_Merge_HeadingLevel(t *testing.T) {
	base := SemanticsProperties{HeadingLevel: 1}
	other := SemanticsProperties{HeadingLevel: 3}

	result := base.Merge(other)
	if result.HeadingLevel != 3 {
		t.Errorf("HeadingLevel = %d, want 3", result.HeadingLevel)
	}
}

func TestSemanticsProperties_Merge_SortKey(t *testing.T) {
	v := 5.0
	base := SemanticsProperties{}
	other := SemanticsProperties{SortKey: &v}

	result := base.Merge(other)
	if result.SortKey != &v {
		t.Error("SortKey should be set from other")
	}
}
