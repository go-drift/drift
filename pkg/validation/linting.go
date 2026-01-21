//go:build android || darwin || ios
// +build android darwin ios

package validation

import (
	"fmt"

	"github.com/go-drift/drift/pkg/semantics"
)

// Severity indicates the severity of a lint result.
type Severity int

const (
	// SeverityInfo is for informational messages.
	SeverityInfo Severity = iota

	// SeverityWarning is for potential issues that should be addressed.
	SeverityWarning

	// SeverityError is for accessibility violations that must be fixed.
	SeverityError
)

// String returns the severity name.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return "unknown"
	}
}

// LintRule represents an accessibility lint rule.
type LintRule string

const (
	// LintRuleMissingLabel indicates a node is missing an accessibility label.
	LintRuleMissingLabel LintRule = "missing-label"

	// LintRuleImageMissingAlt indicates an image is missing alt text.
	LintRuleImageMissingAlt LintRule = "image-missing-alt"

	// LintRuleTouchTargetSize indicates a touch target is too small.
	LintRuleTouchTargetSize LintRule = "touch-target-size"

	// LintRuleEmptyButton indicates a button has no label.
	LintRuleEmptyButton LintRule = "empty-button"

	// LintRuleMissingValue indicates a control is missing a value.
	LintRuleMissingValue LintRule = "missing-value"

	// LintRuleMissingHint indicates an interactive element is missing a hint.
	LintRuleMissingHint LintRule = "missing-hint"
)

// LintResult contains the result of a lint check.
type LintResult struct {
	// NodeID is the ID of the node with the issue.
	NodeID int64

	// Severity indicates how serious the issue is.
	Severity Severity

	// Rule identifies which rule was violated.
	Rule LintRule

	// Message describes the issue.
	Message string

	// Suggestion provides guidance on how to fix the issue.
	Suggestion string
}

// LintSemanticsTree runs accessibility lint checks on a semantics tree.
func LintSemanticsTree(root *semantics.SemanticsNode) []LintResult {
	if root == nil {
		return nil
	}

	var results []LintResult

	root.Visit(func(node *semantics.SemanticsNode) bool {
		nodeResults := lintNode(node)
		results = append(results, nodeResults...)
		return true
	})

	return results
}

// lintNode runs lint checks on a single node.
func lintNode(node *semantics.SemanticsNode) []LintResult {
	var results []LintResult

	props := node.Config.Properties
	actions := node.Config.Actions

	// Check for missing labels on interactive elements
	if actions != nil && !actions.IsEmpty() {
		if props.Label == "" && props.Value == "" {
			results = append(results, LintResult{
				NodeID:     node.ID,
				Severity:   SeverityError,
				Rule:       LintRuleMissingLabel,
				Message:    "Interactive element is missing an accessibility label",
				Suggestion: "Add a Label property to describe what this element does",
			})
		}
	}

	// Check for buttons without labels
	if props.Role == semantics.SemanticsRoleButton || props.Flags.Has(semantics.SemanticsIsButton) {
		if props.Label == "" {
			results = append(results, LintResult{
				NodeID:     node.ID,
				Severity:   SeverityError,
				Rule:       LintRuleEmptyButton,
				Message:    "Button is missing an accessibility label",
				Suggestion: "Add a Label property to describe the button's action",
			})
		}
	}

	// Check for images without alt text
	if props.Role == semantics.SemanticsRoleImage || props.Flags.Has(semantics.SemanticsIsImage) {
		if props.Label == "" && !props.Flags.Has(semantics.SemanticsIsHidden) {
			results = append(results, LintResult{
				NodeID:     node.ID,
				Severity:   SeverityWarning,
				Rule:       LintRuleImageMissingAlt,
				Message:    "Image is missing alt text",
				Suggestion: "Add a SemanticLabel property, or mark as ExcludeFromSemantics if decorative",
			})
		}
	}

	// Check for sliders without values
	if props.Role == semantics.SemanticsRoleSlider || props.Flags.Has(semantics.SemanticsIsSlider) {
		if props.CurrentValue == nil {
			results = append(results, LintResult{
				NodeID:     node.ID,
				Severity:   SeverityWarning,
				Rule:       LintRuleMissingValue,
				Message:    "Slider is missing current value",
				Suggestion: "Set CurrentValue, MinValue, and MaxValue properties",
			})
		}
	}

	// Check touch target size (48x48 dp minimum recommended)
	if actions != nil && !actions.IsEmpty() {
		width := node.Rect.Right - node.Rect.Left
		height := node.Rect.Bottom - node.Rect.Top
		if width > 0 && height > 0 && (width < 48 || height < 48) {
			results = append(results, LintResult{
				NodeID:   node.ID,
				Severity: SeverityWarning,
				Rule:     LintRuleTouchTargetSize,
				Message: fmt.Sprintf(
					"Touch target is too small (%.0fx%.0f). Minimum recommended size is 48x48",
					width, height,
				),
				Suggestion: "Increase the size or add padding to meet the 48x48 minimum touch target",
			})
		}
	}

	// Check for text fields without hints
	if props.Role == semantics.SemanticsRoleTextField || props.Flags.Has(semantics.SemanticsIsTextField) {
		if props.Hint == "" && props.Label == "" {
			results = append(results, LintResult{
				NodeID:     node.ID,
				Severity:   SeverityInfo,
				Rule:       LintRuleMissingHint,
				Message:    "Text field could benefit from a hint or label",
				Suggestion: "Add a Hint property to help users understand the expected input",
			})
		}
	}

	return results
}

// LintOptions configures which lint rules to run.
type LintOptions struct {
	// IncludeInfo includes informational messages in results.
	IncludeInfo bool

	// MinTouchTargetSize is the minimum touch target size in dp.
	MinTouchTargetSize float64

	// DisabledRules contains rules that should be skipped.
	DisabledRules map[LintRule]bool
}

// DefaultLintOptions returns the default lint options.
func DefaultLintOptions() LintOptions {
	return LintOptions{
		IncludeInfo:        false,
		MinTouchTargetSize: 48,
		DisabledRules:      make(map[LintRule]bool),
	}
}

// LintWithOptions runs lint checks with custom options.
func LintWithOptions(root *semantics.SemanticsNode, options LintOptions) []LintResult {
	allResults := LintSemanticsTree(root)

	var filtered []LintResult
	for _, result := range allResults {
		// Skip disabled rules
		if options.DisabledRules[result.Rule] {
			continue
		}

		// Skip info-level results if not requested
		if result.Severity == SeverityInfo && !options.IncludeInfo {
			continue
		}

		filtered = append(filtered, result)
	}

	return filtered
}

// HasErrors returns true if any lint results are errors.
func HasErrors(results []LintResult) bool {
	for _, r := range results {
		if r.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any lint results are warnings or errors.
func HasWarnings(results []LintResult) bool {
	for _, r := range results {
		if r.Severity >= SeverityWarning {
			return true
		}
	}
	return false
}

// FilterBySeverity returns results at or above the given severity level.
func FilterBySeverity(results []LintResult, minSeverity Severity) []LintResult {
	var filtered []LintResult
	for _, r := range results {
		if r.Severity >= minSeverity {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// GroupByRule groups lint results by their rule.
func GroupByRule(results []LintResult) map[LintRule][]LintResult {
	groups := make(map[LintRule][]LintResult)
	for _, r := range results {
		groups[r.Rule] = append(groups[r.Rule], r)
	}
	return groups
}
