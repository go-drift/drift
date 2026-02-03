package widgets

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
)

// DeviceScale provides the current device pixel scale factor to descendants.
type DeviceScale struct {
	Scale float64
	Child core.Widget
}

func (d DeviceScale) CreateElement() core.Element {
	return core.NewInheritedElement(d, nil)
}

func (d DeviceScale) Key() any {
	return nil
}

func (d DeviceScale) ChildWidget() core.Widget {
	return d.Child
}

func (d DeviceScale) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(DeviceScale); ok {
		return d.Scale != old.Scale
	}
	return true
}

// UpdateShouldNotifyDependent returns true for any aspects since DeviceScale
// doesn't support granular aspect tracking yet.
func (d DeviceScale) UpdateShouldNotifyDependent(oldWidget core.InheritedWidget, aspects map[any]struct{}) bool {
	return d.UpdateShouldNotify(oldWidget)
}

var deviceScaleType = reflect.TypeOf(DeviceScale{})

// DeviceScaleOf returns the current device scale, defaulting to 1 if not found.
func DeviceScaleOf(ctx core.BuildContext) float64 {
	inherited := ctx.DependOnInherited(deviceScaleType, nil)
	if ds, ok := inherited.(DeviceScale); ok && ds.Scale > 0 {
		return ds.Scale
	}
	return 1
}
