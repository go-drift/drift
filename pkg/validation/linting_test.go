//go:build android || darwin || ios
// +build android darwin ios

package validation

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/semantics"
)

func TestLintSemanticsTree_Nil(t *testing.T) {
	results := LintSemanticsTree(nil)
	if results != nil {
		t.Error("LintSemanticsTree(nil) should return nil")
	}
}

func TestLintSemanticsTree_MissingLabel(t *testing.T) {
	node := semantics.NewSemanticsNode()
	node.Config.Actions = semantics.NewSemanticsActions()
	node.Config.Actions.SetHandler(semantics.SemanticsActionTap, func(args any) {})

	results := LintSemanticsTree(node)

	if len(results) == 0 {
		t.Fatal("Expected at least one lint result")
	}

	found := false
	for _, r := range results {
		if r.Rule == LintRuleMissingLabel {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected missing-label lint error")
	}
}

func TestLintSemanticsTree_EmptyButton(t *testing.T) {
	node := semantics.NewSemanticsNode()
	node.Config.Properties.Role = semantics.SemanticsRoleButton
	// No label set

	results := LintSemanticsTree(node)

	found := false
	for _, r := range results {
		if r.Rule == LintRuleEmptyButton {
			found = true
			if r.Severity != SeverityError {
				t.Error("Empty button should be an error")
			}
			break
		}
	}

	if !found {
		t.Error("Expected empty-button lint error")
	}
}

func TestLintSemanticsTree_ImageMissingAlt(t *testing.T) {
	node := semantics.NewSemanticsNode()
	node.Config.Properties.Role = semantics.SemanticsRoleImage
	// No label set, not hidden

	results := LintSemanticsTree(node)

	found := false
	for _, r := range results {
		if r.Rule == LintRuleImageMissingAlt {
			found = true
			if r.Severity != SeverityWarning {
				t.Error("Image missing alt should be a warning")
			}
			break
		}
	}

	if !found {
		t.Error("Expected image-missing-alt lint warning")
	}
}

func TestLintSemanticsTree_ImageWithAlt(t *testing.T) {
	node := semantics.NewSemanticsNode()
	node.Config.Properties.Role = semantics.SemanticsRoleImage
	node.Config.Properties.Label = "A beautiful sunset"

	results := LintSemanticsTree(node)

	for _, r := range results {
		if r.Rule == LintRuleImageMissingAlt {
			t.Error("Should not report missing alt when label is set")
		}
	}
}

func TestLintSemanticsTree_HiddenImage(t *testing.T) {
	node := semantics.NewSemanticsNode()
	node.Config.Properties.Role = semantics.SemanticsRoleImage
	node.Config.Properties.Flags = semantics.SemanticsIsHidden

	results := LintSemanticsTree(node)

	for _, r := range results {
		if r.Rule == LintRuleImageMissingAlt {
			t.Error("Should not report missing alt for hidden images")
		}
	}
}

func TestLintSemanticsTree_SmallTouchTarget(t *testing.T) {
	node := semantics.NewSemanticsNode()
	node.Rect = graphics.RectFromLTWH(0, 0, 30, 30) // Too small
	node.Config.Actions = semantics.NewSemanticsActions()
	node.Config.Actions.SetHandler(semantics.SemanticsActionTap, func(args any) {})
	node.Config.Properties.Label = "Button" // Add label to avoid missing-label error

	results := LintSemanticsTree(node)

	found := false
	for _, r := range results {
		if r.Rule == LintRuleTouchTargetSize {
			found = true
			if r.Severity != SeverityWarning {
				t.Error("Small touch target should be a warning")
			}
			break
		}
	}

	if !found {
		t.Error("Expected touch-target-size lint warning")
	}
}

func TestLintSemanticsTree_AdequateTouchTarget(t *testing.T) {
	node := semantics.NewSemanticsNode()
	node.Rect = graphics.RectFromLTWH(0, 0, 48, 48) // Minimum size
	node.Config.Actions = semantics.NewSemanticsActions()
	node.Config.Actions.SetHandler(semantics.SemanticsActionTap, func(args any) {})
	node.Config.Properties.Label = "Button"

	results := LintSemanticsTree(node)

	for _, r := range results {
		if r.Rule == LintRuleTouchTargetSize {
			t.Error("48x48 touch target should not trigger warning")
		}
	}
}

func TestLintSemanticsTree_SliderMissingValue(t *testing.T) {
	node := semantics.NewSemanticsNode()
	node.Config.Properties.Role = semantics.SemanticsRoleSlider
	// No CurrentValue set

	results := LintSemanticsTree(node)

	found := false
	for _, r := range results {
		if r.Rule == LintRuleMissingValue {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected missing-value lint warning for slider")
	}
}

func TestLintSemanticsTree_SliderWithValue(t *testing.T) {
	node := semantics.NewSemanticsNode()
	node.Config.Properties.Role = semantics.SemanticsRoleSlider
	value := 50.0
	node.Config.Properties.CurrentValue = &value

	results := LintSemanticsTree(node)

	for _, r := range results {
		if r.Rule == LintRuleMissingValue {
			t.Error("Should not report missing value when set")
		}
	}
}

func TestHasErrors(t *testing.T) {
	noErrors := []LintResult{
		{Severity: SeverityWarning},
		{Severity: SeverityInfo},
	}

	if HasErrors(noErrors) {
		t.Error("Should return false when no errors")
	}

	withError := []LintResult{
		{Severity: SeverityWarning},
		{Severity: SeverityError},
	}

	if !HasErrors(withError) {
		t.Error("Should return true when error exists")
	}
}

func TestHasWarnings(t *testing.T) {
	infoOnly := []LintResult{
		{Severity: SeverityInfo},
	}

	if HasWarnings(infoOnly) {
		t.Error("Should return false for info only")
	}

	withWarning := []LintResult{
		{Severity: SeverityWarning},
	}

	if !HasWarnings(withWarning) {
		t.Error("Should return true when warning exists")
	}
}

func TestFilterBySeverity(t *testing.T) {
	results := []LintResult{
		{Severity: SeverityInfo},
		{Severity: SeverityWarning},
		{Severity: SeverityError},
	}

	warnings := FilterBySeverity(results, SeverityWarning)
	if len(warnings) != 2 {
		t.Errorf("Expected 2 results at warning+, got %d", len(warnings))
	}

	errors := FilterBySeverity(results, SeverityError)
	if len(errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errors))
	}
}

func TestGroupByRule(t *testing.T) {
	results := []LintResult{
		{Rule: LintRuleMissingLabel},
		{Rule: LintRuleMissingLabel},
		{Rule: LintRuleEmptyButton},
	}

	groups := GroupByRule(results)

	if len(groups[LintRuleMissingLabel]) != 2 {
		t.Error("Should have 2 missing-label results")
	}

	if len(groups[LintRuleEmptyButton]) != 1 {
		t.Error("Should have 1 empty-button result")
	}
}

func TestLintWithOptions_DisabledRules(t *testing.T) {
	node := semantics.NewSemanticsNode()
	node.Config.Properties.Role = semantics.SemanticsRoleButton

	options := DefaultLintOptions()
	options.DisabledRules[LintRuleEmptyButton] = true

	results := LintWithOptions(node, options)

	for _, r := range results {
		if r.Rule == LintRuleEmptyButton {
			t.Error("Disabled rule should not appear in results")
		}
	}
}

func TestLintWithOptions_IncludeInfo(t *testing.T) {
	node := semantics.NewSemanticsNode()
	node.Config.Properties.Role = semantics.SemanticsRoleTextField
	// No hint or label

	optionsNoInfo := DefaultLintOptions()
	optionsNoInfo.IncludeInfo = false

	resultsNoInfo := LintWithOptions(node, optionsNoInfo)
	hasInfo := false
	for _, r := range resultsNoInfo {
		if r.Severity == SeverityInfo {
			hasInfo = true
			break
		}
	}

	if hasInfo {
		t.Error("Info results should be filtered out when IncludeInfo is false")
	}

	optionsWithInfo := DefaultLintOptions()
	optionsWithInfo.IncludeInfo = true

	resultsWithInfo := LintWithOptions(node, optionsWithInfo)
	hasInfo = false
	for _, r := range resultsWithInfo {
		if r.Severity == SeverityInfo {
			hasInfo = true
			break
		}
	}

	if !hasInfo {
		t.Error("Info results should be included when IncludeInfo is true")
	}
}

func TestSeverity_String(t *testing.T) {
	if SeverityInfo.String() != "info" {
		t.Error("SeverityInfo should be 'info'")
	}
	if SeverityWarning.String() != "warning" {
		t.Error("SeverityWarning should be 'warning'")
	}
	if SeverityError.String() != "error" {
		t.Error("SeverityError should be 'error'")
	}
}
