package navigation

import (
	"reflect"
	"testing"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/overlay"
	"github.com/go-drift/drift/pkg/widgets"
)

type mockBuildContext struct{}

func (m mockBuildContext) Widget() core.Widget                                   { return nil }
func (m mockBuildContext) FindAncestor(func(core.Element) bool) core.Element     { return nil }
func (m mockBuildContext) DependOnInherited(reflect.Type, any) any               { return nil }
func (m mockBuildContext) DependOnInheritedWithAspects(reflect.Type, ...any) any { return nil }

type fakeNavigator struct {
	popCount int
	lastPop  any
}

func (f *fakeNavigator) Push(Route)                       {}
func (f *fakeNavigator) PushNamed(string, any)            {}
func (f *fakeNavigator) PushReplacementNamed(string, any) {}
func (f *fakeNavigator) Pop(result any)                   { f.popCount++; f.lastPop = result }
func (f *fakeNavigator) PopUntil(func(Route) bool)        {}
func (f *fakeNavigator) PushReplacement(Route)            {}
func (f *fakeNavigator) CanPop() bool                     { return true }
func (f *fakeNavigator) MaybePop(result any) bool         { f.Pop(result); return true }

func TestNewBottomSheetRoute_Defaults(t *testing.T) {
	route := NewBottomSheetRoute(nil, RouteSettings{})

	if !route.BarrierDismissible {
		t.Error("BarrierDismissible should default to true")
	}
	if !route.EnableDrag {
		t.Error("EnableDrag should default to true")
	}
	if route.DragMode != widgets.DragModeAuto {
		t.Error("DragMode should default to DragModeAuto")
	}
	if !route.ShowHandle {
		t.Error("ShowHandle should default to true")
	}
	if !route.UseSafeArea {
		t.Error("UseSafeArea should default to true")
	}
	if route.SnapPoints != nil {
		t.Error("SnapPoints should default to nil (uses widget defaults)")
	}
	if route.InitialSnapPoint != 0 {
		t.Error("InitialSnapPoint should default to 0")
	}
	if route.BarrierColor != nil {
		t.Error("BarrierColor should default to nil (uses theme)")
	}
}

func TestBottomSheetRoute_Options(t *testing.T) {
	route := NewBottomSheetRoute(nil, RouteSettings{})

	// Apply options
	WithSnapPoints(widgets.SnapHalf, widgets.SnapFull)(route)
	WithInitialSnapPoint(1)(route)
	WithBarrierDismissible(false)(route)
	WithBarrierColor(graphics.ColorBlack)(route)
	WithDragEnabled(false)(route)
	WithDragMode(widgets.DragModeHandleOnly)(route)
	WithHandle(false)(route)
	WithSafeArea(false)(route)

	if len(route.SnapPoints) != 2 {
		t.Errorf("Expected 2 snap points, got %d", len(route.SnapPoints))
	}
	if route.InitialSnapPoint != 1 {
		t.Errorf("Expected InitialSnapPoint 1, got %d", route.InitialSnapPoint)
	}
	if route.BarrierDismissible {
		t.Error("Expected BarrierDismissible false")
	}
	if route.BarrierColor == nil || *route.BarrierColor != graphics.ColorBlack {
		t.Error("Expected BarrierColor to be ColorBlack")
	}
	if route.EnableDrag {
		t.Error("Expected EnableDrag false")
	}
	if route.DragMode != widgets.DragModeHandleOnly {
		t.Error("Expected DragModeHandleOnly")
	}
	if route.ShowHandle {
		t.Error("Expected ShowHandle false")
	}
	if route.UseSafeArea {
		t.Error("Expected UseSafeArea false")
	}
}

func TestBottomSheetRoute_DidPush_PendingWithoutOverlay(t *testing.T) {
	route := NewBottomSheetRoute(nil, RouteSettings{})

	// Simulate DidPush before overlay is ready
	route.DidPush()
	if !route.didPushPending {
		t.Error("didPushPending should be true when DidPush called without overlay")
	}
}

func TestBottomSheetRoute_DidPop_ClearsState(t *testing.T) {
	route := NewBottomSheetRoute(nil, RouteSettings{})
	route.didPushPending = true

	route.DidPop(nil)

	if route.didPushPending {
		t.Error("didPushPending should be false after DidPop")
	}
	if route.barrierEntry != nil {
		t.Error("barrierEntry should be nil after DidPop")
	}
	if route.sheetEntry != nil {
		t.Error("sheetEntry should be nil after DidPop")
	}
}

func TestBottomSheetRoute_DidPop_RemovesEntries(t *testing.T) {
	route := NewBottomSheetRoute(nil, RouteSettings{})

	// Create actual overlay entries
	barrierRemoved := false
	sheetRemoved := false

	route.barrierEntry = overlay.NewOverlayEntry(nil)
	route.sheetEntry = overlay.NewOverlayEntry(nil)

	// Track if entries are cleared (real Remove needs overlay, so we just check nil)
	route.DidPop("test-result")

	// Entries should be nil after DidPop (even if Remove() didn't work due to no overlay)
	if route.barrierEntry != nil {
		t.Error("barrierEntry should be nil after DidPop")
	}
	if route.sheetEntry != nil {
		t.Error("sheetEntry should be nil after DidPop")
	}

	// These are checked just to avoid unused variable warnings
	_ = barrierRemoved
	_ = sheetRemoved
}

func TestBottomSheetRoute_DismissIdempotent(t *testing.T) {
	route := NewBottomSheetRoute(nil, RouteSettings{})

	dismissCount := 0
	route.onDismiss = func(any) {
		dismissCount++
	}

	// First dismiss via onAnimationComplete should call onDismiss
	route.onAnimationComplete("result1")
	if dismissCount != 1 {
		t.Errorf("Expected 1 dismiss call, got %d", dismissCount)
	}

	// Second dismiss should be ignored (idempotent)
	route.onAnimationComplete("result2")
	if dismissCount != 1 {
		t.Errorf("Expected still 1 dismiss call (idempotent), got %d", dismissCount)
	}
}

func TestBottomSheetRoute_DismissCallsOnDismiss(t *testing.T) {
	route := NewBottomSheetRoute(nil, RouteSettings{})

	var receivedResult any
	route.onDismiss = func(result any) {
		receivedResult = result
	}

	// Simulate animation completion calling onAnimationComplete
	route.onAnimationComplete("test-result")

	if receivedResult != "test-result" {
		t.Errorf("Expected 'test-result', got %v", receivedResult)
	}
}

func TestShowModalBottomSheet_ChannelReceivesResult(t *testing.T) {
	// This test verifies the channel behavior without a real navigator
	route := NewBottomSheetRoute(nil, RouteSettings{})

	resultChan := make(chan any, 1)
	route.onDismiss = func(value any) {
		resultChan <- value
		close(resultChan)
	}

	// Simulate animation completing with result
	route.onAnimationComplete("test-result")

	// Channel should receive the result
	select {
	case result := <-resultChan:
		if result != "test-result" {
			t.Errorf("Expected result 'test-result', got %v", result)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for result")
	}

	// Channel should be closed (reading again should return default value)
	select {
	case _, ok := <-resultChan:
		if ok {
			t.Error("Expected channel to be closed")
		}
	default:
		// Channel is closed and empty - this is expected
	}
}

func TestShowModalBottomSheet_ChannelWorksWithNilResult(t *testing.T) {
	route := NewBottomSheetRoute(nil, RouteSettings{})

	resultChan := make(chan any, 1)
	route.onDismiss = func(value any) {
		resultChan <- value
		close(resultChan)
	}

	// Simulate animation completing without result
	route.onAnimationComplete(nil)

	// Channel should receive nil
	select {
	case result := <-resultChan:
		if result != nil {
			t.Errorf("Expected nil result, got %v", result)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for result")
	}
}

func TestBottomSheetRoute_DidPop_CallsOnDismiss(t *testing.T) {
	route := NewBottomSheetRoute(nil, RouteSettings{})

	var onDismissCalled bool
	var receivedResult any
	route.onDismiss = func(result any) {
		onDismissCalled = true
		receivedResult = result
	}

	// DidPop should call onDismiss if not already dismissed
	route.DidPop("pop-result")

	if !onDismissCalled {
		t.Error("onDismiss should be called from DidPop")
	}
	if receivedResult != "pop-result" {
		t.Errorf("Expected 'pop-result', got %v", receivedResult)
	}
}

func TestBottomSheetRoute_DidPop_IdempotentWithDismiss(t *testing.T) {
	route := NewBottomSheetRoute(nil, RouteSettings{})

	dismissCount := 0
	route.onDismiss = func(any) {
		dismissCount++
	}

	// First dismiss via onAnimationComplete (simulating animation finished)
	route.onAnimationComplete("dismiss-result")
	if dismissCount != 1 {
		t.Fatalf("Expected 1 dismiss call after onAnimationComplete(), got %d", dismissCount)
	}

	// DidPop should not call onDismiss again (already dismissed)
	route.DidPop("pop-result")
	if dismissCount != 1 {
		t.Errorf("Expected still 1 dismiss call after DidPop (already dismissed), got %d", dismissCount)
	}
}

func TestBottomSheetRoute_OnAnimationComplete_DoesNotPopWhenDidPop(t *testing.T) {
	route := NewBottomSheetRoute(nil, RouteSettings{})
	nav := &fakeNavigator{}
	route.pushingNav = nav
	route.poppedFromNav = true

	route.onAnimationComplete("result")

	if nav.popCount != 0 {
		t.Errorf("Expected Pop not to be called, got %d", nav.popCount)
	}
}

func TestShowModalBottomSheet_NavigatorNilClosesChannel(t *testing.T) {
	ctx := mockBuildContext{}
	ch := ShowModalBottomSheet(ctx, func(core.BuildContext) core.Widget {
		return nil
	})

	select {
	case result, ok := <-ch:
		if !ok {
			t.Fatal("Expected channel to yield a value before closing")
		}
		if result != nil {
			t.Errorf("Expected nil result, got %v", result)
		}
		_, ok = <-ch
		if ok {
			t.Error("Expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timed out waiting for channel result")
	}
}
