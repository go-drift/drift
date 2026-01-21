package theme

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
)

// Theme provides ThemeData to descendant widgets via InheritedWidget.
type Theme struct {
	// Data is the theme configuration.
	Data *ThemeData
	// ChildWidget is the child widget tree.
	ChildWidget core.Widget
}

// CreateElement returns an InheritedElement for this Theme.
func (t Theme) CreateElement() core.Element {
	return core.NewInheritedElement(t, nil)
}

// Key returns nil (no key).
func (t Theme) Key() any {
	return nil
}

// Child returns the child widget.
func (t Theme) Child() core.Widget {
	return t.ChildWidget
}

// UpdateShouldNotify returns true if the theme data has changed.
func (t Theme) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(Theme); ok {
		return t.Data != old.Data
	}
	return true
}

var themeType = reflect.TypeOf(Theme{})

// ThemeOf returns the nearest ThemeData in the tree.
// If no Theme ancestor is found, returns the default light theme.
func ThemeOf(ctx core.BuildContext) *ThemeData {
	// Check AppTheme first (unified provider)
	if appTheme := AppThemeMaybeOf(ctx); appTheme != nil {
		return appTheme.Material
	}
	// Fall back to legacy Theme widget
	inherited := ctx.DependOnInherited(themeType)
	if inherited == nil {
		return DefaultLightTheme()
	}
	if theme, ok := inherited.(Theme); ok {
		return theme.Data
	}
	return DefaultLightTheme()
}

// MaybeOf returns the nearest ThemeData, or nil if not found.
func MaybeOf(ctx core.BuildContext) *ThemeData {
	inherited := ctx.DependOnInherited(themeType)
	if inherited == nil {
		return nil
	}
	if theme, ok := inherited.(Theme); ok {
		return theme.Data
	}
	return nil
}
