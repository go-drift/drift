//go:build !(android || darwin || ios)
// +build !android,!darwin,!ios

package engine

import (
	"github.com/go-drift/drift/pkg/accessibility"
	"github.com/go-drift/drift/pkg/layout"
)

// initializeAccessibility is a no-op on non-mobile platforms.
func initializeAccessibility() {}

// flushSemantics is a no-op on non-mobile platforms.
func flushSemantics(rootRender layout.RenderObject) {}

// flushSemanticsWithScale is a no-op on non-mobile platforms.
func flushSemanticsWithScale(rootRender layout.RenderObject, deviceScale float64) {}

// GetAccessibilityService returns nil on non-mobile platforms.
func GetAccessibilityService() *accessibility.Service {
	return nil
}

// SetAccessibilityService is a no-op on non-mobile platforms.
func SetAccessibilityService(service *accessibility.Service) {}
