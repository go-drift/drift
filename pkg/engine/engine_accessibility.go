//go:build android || darwin || ios

package engine

import (
	"github.com/go-drift/drift/pkg/accessibility"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/semantics"
)

// accessibilityService is the engine's accessibility service instance.
// This can be replaced for testing.
var accessibilityService = accessibility.NewService()

// initializeAccessibility sets up the accessibility system.
// Called once when the first frame is rendered.
func initializeAccessibility() {
	accessibilityService.Initialize()

	// When accessibility is enabled after the initial frame, request a frame
	// so the semantics tree gets built. Without this, on-demand scheduling
	// keeps the display link paused and flushSemanticsIfNeeded never runs.
	binding := semantics.GetSemanticsBinding()
	binding.SetOnEnabledChanged(func(enabled bool) {
		if enabled {
			Dispatch(func() {
				app.semanticsForceFlush = true
			})
			RequestFrame()
		}
	})
}

// flushSemanticsWithScale rebuilds and sends the semantics tree.
// deviceScale is used to convert logical pixels to screen pixels.
// dirtyBoundaries contains semantics boundaries that need update (nil means full rebuild).
func flushSemanticsWithScale(rootRender layout.RenderObject, deviceScale float64, dirtyBoundaries []layout.RenderObject) {
	accessibilityService.SetDeviceScale(deviceScale)
	accessibilityService.FlushSemantics(rootRender, dirtyBoundaries)
}

// flushSemantics is called from engine.go - uses default scale of 1.0
func flushSemantics(rootRender layout.RenderObject) {
	flushSemanticsWithScale(rootRender, 1.0, nil)
}

// GetAccessibilityService returns the engine's accessibility service.
// This is useful for testing and advanced use cases.
func GetAccessibilityService() *accessibility.Service {
	return accessibilityService
}

// SetAccessibilityService replaces the engine's accessibility service.
// This is primarily useful for testing.
func SetAccessibilityService(service *accessibility.Service) {
	accessibilityService = service
}
