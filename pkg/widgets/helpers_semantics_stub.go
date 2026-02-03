//go:build !android && !darwin && !ios

package widgets

import "github.com/go-drift/drift/pkg/core"

// On non-mobile platforms, these helpers pass through the child widget
// since accessibility services aren't available.

// Tappable creates a tappable widget.
func Tappable(label string, onTap func(), child core.Widget) GestureDetector {
	return GestureDetector{OnTap: onTap, Child: child}
}

// TappableWithHint creates a tappable widget with hint.
func TappableWithHint(label, hint string, onTap func(), child core.Widget) GestureDetector {
	return GestureDetector{OnTap: onTap, Child: child}
}

// SemanticLabel wraps a child with an accessibility label (no-op on desktop).
func SemanticLabel(label string, child core.Widget) core.Widget {
	return child
}

// SemanticImage marks a widget as an image (no-op on desktop).
func SemanticImage(description string, child core.Widget) core.Widget {
	return child
}

// SemanticHeading marks a widget as a heading (no-op on desktop).
func SemanticHeading(level int, child core.Widget) core.Widget {
	return child
}

// SemanticLink marks a widget as a link.
func SemanticLink(label string, onTap func(), child core.Widget) GestureDetector {
	return GestureDetector{OnTap: onTap, Child: child}
}

// SemanticGroup groups widgets into a single accessibility unit (no-op on desktop).
func SemanticGroup(child core.Widget) core.Widget {
	return child
}

// SemanticLiveRegion marks a widget as a live region (no-op on desktop).
func SemanticLiveRegion(child core.Widget) core.Widget {
	return child
}

// Decorative hides a widget from screen readers (no-op on desktop).
func Decorative(child core.Widget) core.Widget {
	return child
}
