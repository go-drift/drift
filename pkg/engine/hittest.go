package engine

import (
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
)

// HitTestPlatformView checks whether a native platform view is the topmost
// hit target at the given pixel coordinates. Returns true if the first
// interactive entry in the hit test result is a PlatformViewOwner with a
// matching viewID, meaning the platform view should receive the touch.
//
// Called synchronously from the native UI thread (via CGo) before each touch
// is dispatched to a platform view. Both this function and HandlePointer run
// on the same native thread, so they never execute concurrently despite both
// acquiring frameLock.
func HitTestPlatformView(viewID int64, x, y float64) bool {
	frameLock.Lock()
	defer frameLock.Unlock()

	rootRender := app.rootRender
	if rootRender == nil {
		return false
	}

	scale := app.deviceScale
	position := graphics.Offset{X: x / scale, Y: y / scale}

	result := &layout.HitTestResult{}
	if !rootRender.HitTest(position, result) || len(result.Entries) == 0 {
		return false
	}

	// Walk entries looking for the first interactive target. A PlatformViewOwner
	// is interactive even without PointerHandler (it handles touches natively).
	// Non-interactive entries (neither PlatformViewOwner nor PointerHandler) are
	// purely decorative and skipped.
	for _, entry := range result.Entries {
		// Check PlatformViewOwner first: views like video player and webview
		// handle touches natively and don't implement PointerHandler.
		if owner, ok := entry.(layout.PlatformViewOwner); ok {
			return owner.PlatformViewID() == viewID
		}
		// Skip non-interactive (decorative) entries.
		if _, ok := entry.(layout.PointerHandler); !ok {
			continue
		}
		// First interactive handler is not a platform view owner: view is obscured.
		return false
	}

	return false
}
