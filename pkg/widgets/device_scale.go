package widgets

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
)

// DeviceScale provides the current device pixel scale factor to descendants.
type DeviceScale struct {
	core.InheritedBase
	Scale float64
	Child core.Widget
}

func (d DeviceScale) ChildWidget() core.Widget { return d.Child }

func (d DeviceScale) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(DeviceScale); ok {
		return d.Scale != old.Scale
	}
	return true
}

var deviceScaleType = reflect.TypeFor[DeviceScale]()

// DeviceScaleOf returns the current device scale, defaulting to 1 if not found.
func DeviceScaleOf(ctx core.BuildContext) float64 {
	inherited := ctx.DependOnInherited(deviceScaleType, nil)
	if ds, ok := inherited.(DeviceScale); ok && ds.Scale > 0 {
		return ds.Scale
	}
	return 1
}
