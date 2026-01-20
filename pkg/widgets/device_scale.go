package widgets

import (
	"reflect"

	"github.com/go-drift/drift/pkg/core"
)

// DeviceScale provides the current device pixel scale factor to descendants.
type DeviceScale struct {
	Scale       float64
	ChildWidget core.Widget
}

func (d DeviceScale) CreateElement() core.Element {
	return core.NewInheritedElement(d, nil)
}

func (d DeviceScale) Key() any {
	return nil
}

func (d DeviceScale) Child() core.Widget {
	return d.ChildWidget
}

func (d DeviceScale) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(DeviceScale); ok {
		return d.Scale != old.Scale
	}
	return true
}

var deviceScaleType = reflect.TypeOf(DeviceScale{})

// DeviceScaleOf returns the current device scale, defaulting to 1 if not found.
func DeviceScaleOf(ctx core.BuildContext) float64 {
	inherited := ctx.DependOnInherited(deviceScaleType)
	if ds, ok := inherited.(DeviceScale); ok && ds.Scale > 0 {
		return ds.Scale
	}
	return 1
}
