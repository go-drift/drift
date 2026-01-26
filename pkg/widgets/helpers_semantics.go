//go:build android || darwin || ios

package widgets

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/semantics"
)

// Tappable creates an accessible tappable widget with the given label.
// This is the accessible version of Tap() - use this when the tappable
// element should be accessible to screen readers.
//
// Example:
//
//	Tappable("Submit form", func() { submit() }, myButton)
func Tappable(label string, onTap func(), child core.Widget) Semantics {
	return Semantics{
		Label:            label,
		Hint:             "Double tap to activate",
		Role:             semantics.SemanticsRoleButton,
		Flags:            semantics.SemanticsIsButton | semantics.SemanticsHasEnabledState | semantics.SemanticsIsEnabled,
		Container:        true,
		MergeDescendants: true,
		OnTap:            onTap,
		ChildWidget:      GestureDetector{OnTap: onTap, ChildWidget: child},
	}
}

// TappableWithHint creates an accessible tappable widget with custom label and hint.
func TappableWithHint(label, hint string, onTap func(), child core.Widget) Semantics {
	return Semantics{
		Label:            label,
		Hint:             hint,
		Role:             semantics.SemanticsRoleButton,
		Flags:            semantics.SemanticsIsButton | semantics.SemanticsHasEnabledState | semantics.SemanticsIsEnabled,
		Container:        true,
		MergeDescendants: true,
		OnTap:            onTap,
		ChildWidget:      GestureDetector{OnTap: onTap, ChildWidget: child},
	}
}

// SemanticLabel wraps a child with an accessibility label.
// Use this to provide a description for widgets that don't have built-in semantics.
//
// Example:
//
//	SemanticLabel("Company logo", logoImage)
func SemanticLabel(label string, child core.Widget) Semantics {
	return Semantics{
		Label:       label,
		Container:   true,
		ChildWidget: child,
	}
}

// SemanticImage marks a widget as an image with the given description.
// Use this for meaningful images that convey information.
//
// Example:
//
//	SemanticImage("Chart showing sales growth", chartWidget)
func SemanticImage(description string, child core.Widget) Semantics {
	return Semantics{
		Label:       description,
		Role:        semantics.SemanticsRoleImage,
		Flags:       semantics.SemanticsIsImage,
		Container:   true,
		ChildWidget: child,
	}
}

// SemanticHeading marks a widget as a heading at the specified level (1-6).
// Screen readers use headings for navigation.
//
// Example:
//
//	SemanticHeading(1, widgets.Text{Content: "Welcome"})
func SemanticHeading(level int, child core.Widget) Semantics {
	return Semantics{
		Role:         semantics.SemanticsRoleHeader,
		Flags:        semantics.SemanticsIsHeader,
		HeadingLevel: level,
		Container:    true,
		ChildWidget:  child,
	}
}

// SemanticLink marks a widget as a link with the given label.
//
// Example:
//
//	SemanticLink("Visit our website", func() { openURL() }, linkText)
func SemanticLink(label string, onTap func(), child core.Widget) Semantics {
	return Semantics{
		Label:            label,
		Hint:             "Double tap to open link",
		Role:             semantics.SemanticsRoleLink,
		Container:        true,
		MergeDescendants: true,
		OnTap:            onTap,
		ChildWidget:      GestureDetector{OnTap: onTap, ChildWidget: child},
	}
}

// SemanticGroup groups related widgets into a single accessibility unit.
// The screen reader will read all children as one combined announcement.
//
// Example:
//
//	SemanticGroup(widgets.Row{Children: []core.Widget{icon, priceText, currencyText}})
func SemanticGroup(child core.Widget) Semantics {
	return Semantics{
		Container:        true,
		MergeDescendants: true,
		ChildWidget:      child,
	}
}

// SemanticLiveRegion marks a widget as a live region that announces changes.
// Use this for content that updates dynamically (e.g., status messages, timers).
//
// Example:
//
//	SemanticLiveRegion(statusText)
func SemanticLiveRegion(child core.Widget) Semantics {
	return Semantics{
		Flags:       semantics.SemanticsIsLiveRegion,
		Container:   true,
		ChildWidget: child,
	}
}

// Decorative marks a widget as decorative, hiding it from screen readers.
// Use this for purely visual elements that don't convey information.
//
// Example:
//
//	Decorative(dividerLine)
func Decorative(child core.Widget) ExcludeSemantics {
	return ExcludeSemantics{
		Excluding:   true,
		ChildWidget: child,
	}
}
