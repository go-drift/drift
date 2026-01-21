package theme

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
)

// AppThemeData holds both Material and Cupertino theme data.
// Brightness is derived from Material.Brightness to avoid divergence.
type AppThemeData struct {
	Platform  TargetPlatform
	Material  *ThemeData
	Cupertino *CupertinoThemeData
}

// Brightness returns the theme brightness (derived from Material theme).
func (a *AppThemeData) Brightness() Brightness {
	if a.Material != nil {
		return a.Material.Brightness
	}
	return BrightnessLight
}

// NewAppThemeData creates theme data for the given platform and brightness.
func NewAppThemeData(platform TargetPlatform, brightness Brightness) *AppThemeData {
	var material *ThemeData
	var cupertino *CupertinoThemeData
	if brightness == BrightnessDark {
		material = DefaultDarkTheme()
		cupertino = DefaultCupertinoDarkTheme()
	} else {
		material = DefaultLightTheme()
		cupertino = DefaultCupertinoLightTheme()
	}
	return &AppThemeData{
		Platform:  platform,
		Material:  material,
		Cupertino: cupertino,
	}
}

// AppTheme provides unified theme data via InheritedWidget.
type AppTheme struct {
	Data        *AppThemeData
	ChildWidget core.Widget
}

// CreateElement returns an InheritedElement for this AppTheme.
func (a AppTheme) CreateElement() core.Element {
	return core.NewInheritedElement(a, nil)
}

// Key returns nil (no key).
func (a AppTheme) Key() any {
	return nil
}

// Child returns the child widget.
func (a AppTheme) Child() core.Widget {
	return a.ChildWidget
}

// UpdateShouldNotify returns true if the theme data has changed.
func (a AppTheme) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	old, ok := oldWidget.(AppTheme)
	if !ok {
		return true
	}
	// Handle nil Data safely
	if a.Data == nil || old.Data == nil {
		return a.Data != old.Data
	}
	return a.Data.Platform != old.Data.Platform ||
		a.Data.Material != old.Data.Material ||
		a.Data.Cupertino != old.Data.Cupertino
}

var appThemeType = reflect.TypeOf(AppTheme{})

// Cached default to avoid repeated allocations when no AppTheme is found.
var defaultAppThemeData = NewAppThemeData(TargetPlatformMaterial, BrightnessLight)

// AppThemeOf returns the nearest AppThemeData.
// Returns a cached default if no AppTheme is found or if Data is nil.
func AppThemeOf(ctx core.BuildContext) *AppThemeData {
	inherited := ctx.DependOnInherited(appThemeType)
	if inherited == nil {
		return defaultAppThemeData
	}
	if a, ok := inherited.(AppTheme); ok && a.Data != nil {
		return a.Data
	}
	return defaultAppThemeData
}

// AppThemeMaybeOf returns the nearest AppThemeData, or nil if not found.
// Returns nil if no AppTheme is found or if Data is nil.
func AppThemeMaybeOf(ctx core.BuildContext) *AppThemeData {
	inherited := ctx.DependOnInherited(appThemeType)
	if inherited == nil {
		return nil
	}
	if a, ok := inherited.(AppTheme); ok && a.Data != nil {
		return a.Data
	}
	return nil
}
