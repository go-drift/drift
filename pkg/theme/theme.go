package theme

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
)

// Theme provides ThemeData to descendant widgets via InheritedWidget.
type Theme struct {
	// Data is the theme configuration.
	Data *ThemeData
	// Child is the child widget tree.
	Child core.Widget
}

// CreateElement returns an InheritedElement for this Theme.
func (t Theme) CreateElement() core.Element {
	return core.NewInheritedElement(t, nil)
}

// Key returns nil (no key).
func (t Theme) Key() any {
	return nil
}

// ChildWidget returns the child widget.
func (t Theme) ChildWidget() core.Widget {
	return t.Child
}

// UpdateShouldNotify returns true if the theme data has changed.
func (t Theme) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(Theme); ok {
		return t.Data != old.Data
	}
	return true
}

// UpdateShouldNotifyDependent returns true for any aspects since Theme
// doesn't support granular aspect tracking yet.
func (t Theme) UpdateShouldNotifyDependent(oldWidget core.InheritedWidget, aspects map[any]struct{}) bool {
	return t.UpdateShouldNotify(oldWidget)
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
	inherited := ctx.DependOnInherited(themeType, nil)
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
	inherited := ctx.DependOnInherited(themeType, nil)
	if inherited == nil {
		return nil
	}
	if theme, ok := inherited.(Theme); ok {
		return theme.Data
	}
	return nil
}
