//go:build !(android || darwin || ios)

package engine

import (
	"github.com/go-drift/drift/pkg/accessibility"
	"github.com/go-drift/drift/pkg/layout"
)

// initializeAccessibility is a no-op on non-mobile platforms.
func initializeAccessibility() {}

// flushSemanticsWithScale is a no-op on non-mobile platforms.
func flushSemanticsWithScale(rootRender layout.RenderObject, deviceScale float64, dirtyBoundaries []layout.RenderObject) {
}

// GetAccessibilityService returns nil on non-mobile platforms.
func GetAccessibilityService() *accessibility.Service {
	return nil
}

// SetAccessibilityService is a no-op on non-mobile platforms.
func SetAccessibilityService(service *accessibility.Service) {}
