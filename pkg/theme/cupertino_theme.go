package theme

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
)

// CupertinoTheme provides CupertinoThemeData to descendant widgets via InheritedWidget.
type CupertinoTheme struct {
	// Data is the Cupertino theme configuration.
	Data *CupertinoThemeData
	// ChildWidget is the child widget tree.
	ChildWidget core.Widget
}

// CreateElement returns an InheritedElement for this CupertinoTheme.
func (t CupertinoTheme) CreateElement() core.Element {
	return core.NewInheritedElement(t, nil)
}

// Key returns nil (no key).
func (t CupertinoTheme) Key() any {
	return nil
}

// Child returns the child widget.
func (t CupertinoTheme) Child() core.Widget {
	return t.ChildWidget
}

// UpdateShouldNotify returns true if the theme data has changed.
func (t CupertinoTheme) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(CupertinoTheme); ok {
		return t.Data != old.Data
	}
	return true
}

// UpdateShouldNotifyDependent returns true for any aspects since CupertinoTheme
// doesn't support granular aspect tracking yet.
func (t CupertinoTheme) UpdateShouldNotifyDependent(oldWidget core.InheritedWidget, aspects map[any]struct{}) bool {
	return t.UpdateShouldNotify(oldWidget)
}

var cupertinoThemeType = reflect.TypeOf(CupertinoTheme{})

// CupertinoThemeOf returns the nearest CupertinoThemeData in the tree.
// If no CupertinoTheme ancestor is found, returns the default light theme.
func CupertinoThemeOf(ctx core.BuildContext) *CupertinoThemeData {
	// Check AppTheme first (unified provider)
	if appTheme := AppThemeMaybeOf(ctx); appTheme != nil {
		return appTheme.Cupertino
	}
	// Fall back to legacy CupertinoTheme widget
	inherited := ctx.DependOnInherited(cupertinoThemeType, nil)
	if inherited == nil {
		return DefaultCupertinoLightTheme()
	}
	if theme, ok := inherited.(CupertinoTheme); ok {
		return theme.Data
	}
	return DefaultCupertinoLightTheme()
}

// CupertinoMaybeOf returns the nearest CupertinoThemeData, or nil if not found.
// When using AppTheme, returns data only if Cupertino mode is active.
func CupertinoMaybeOf(ctx core.BuildContext) *CupertinoThemeData {
	// Check AppTheme first - only return if Cupertino mode is active
	if appTheme := AppThemeMaybeOf(ctx); appTheme != nil {
		if appTheme.Platform == TargetPlatformCupertino {
			return appTheme.Cupertino
		}
		return nil
	}
	// Fall back to legacy CupertinoTheme widget
	inherited := ctx.DependOnInherited(cupertinoThemeType, nil)
	if inherited == nil {
		return nil
	}
	if theme, ok := inherited.(CupertinoTheme); ok {
		return theme.Data
	}
	return nil
}

// CupertinoColorsOf returns the CupertinoColors from the nearest CupertinoTheme ancestor.
// If no CupertinoTheme is found, returns the default light colors.
func CupertinoColorsOf(ctx core.BuildContext) CupertinoColors {
	return CupertinoThemeOf(ctx).Colors
}

// CupertinoTextThemeOf returns the CupertinoTextThemeData from the nearest CupertinoTheme ancestor.
// If no CupertinoTheme is found, returns the default text theme.
func CupertinoTextThemeOf(ctx core.BuildContext) CupertinoTextThemeData {
	return CupertinoThemeOf(ctx).TextTheme
}

// UseCupertinoTheme returns all Cupertino theme components in a single call.
func UseCupertinoTheme(ctx core.BuildContext) (*CupertinoThemeData, CupertinoColors, CupertinoTextThemeData) {
	data := CupertinoThemeOf(ctx)
	return data, data.Colors, data.TextTheme
}
