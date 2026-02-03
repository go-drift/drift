package overlay

import (
	"testing"
	"time"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	dtesting "github.com/go-drift/drift/pkg/testing"
	"github.com/go-drift/drift/pkg/widgets"
)

// TestNewOverlayEntry_UniqueIDs verifies that NewOverlayEntry assigns unique IDs.
func TestNewOverlayEntry_UniqueIDs(t *testing.T) {
	entry1 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.SizedBox{}
	})
	entry2 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.SizedBox{}
	})

	if entry1.id == 0 {
		t.Error("entry1 should have non-zero ID")
	}
	if entry2.id == 0 {
		t.Error("entry2 should have non-zero ID")
	}
	if entry1.id == entry2.id {
		t.Error("entries should have different IDs")
	}
}

// TestOverlayEntry_Key verifies that the entry widget has a stable key.
func TestOverlayEntry_Key(t *testing.T) {
	entry := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.SizedBox{}
	})

	widget := overlayEntryWidget{entry: entry}
	key := widget.Key()

	if key != entry.id {
		t.Errorf("expected key to be entry ID %d, got %v", entry.id, key)
	}
}

// TestOverlayEntry_MarkNeedsBuild_BeforeMounted verifies that MarkNeedsBuild is a no-op before mount.
func TestOverlayEntry_MarkNeedsBuild_BeforeMounted(t *testing.T) {
	entry := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.SizedBox{}
	})

	// Should not panic
	entry.MarkNeedsBuild()

	if entry.mounted {
		t.Error("entry should not be mounted")
	}
}

// TestOverlayEntry_Remove_BeforeInsert verifies that Remove is a no-op before insert.
func TestOverlayEntry_Remove_BeforeInsert(t *testing.T) {
	entry := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.SizedBox{}
	})

	// Should not panic
	entry.Remove()

	if entry.overlay != nil {
		t.Error("overlay should be nil")
	}
}

// testOverlayWidget is a test helper that captures the overlay state.
type testOverlayWidget struct {
	onReady func(OverlayState)
}

func (t testOverlayWidget) CreateElement() core.Element {
	return core.NewStatelessElement(t, nil)
}

func (t testOverlayWidget) Key() any {
	return nil
}

func (t testOverlayWidget) Build(ctx core.BuildContext) core.Widget {
	return Overlay{
		Child:          widgets.SizedBox{Width: 100, Height: 100},
		OnOverlayReady: t.onReady,
	}
}

// TestOverlay_OverlayOf_ReturnsState verifies that OverlayOf returns the overlay state.
func TestOverlay_OverlayOf_ReturnsState(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	var capturedState OverlayState
	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			capturedState = state
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pump again to process the Dispatch callback
	tester.Pump()

	if capturedState == nil {
		t.Error("expected to capture overlay state")
	}
}

// TestOverlay_Insert_AddsEntry verifies that Insert adds an entry to the overlay.
func TestOverlay_Insert_AddsEntry(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	var overlayState OverlayState
	entry := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.Text{Content: "overlay content"}
	})

	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			overlayState = state
			state.Insert(entry, nil, nil)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pump to process the Dispatch callback and rebuild
	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if overlayState == nil {
		t.Fatal("overlay state should be set")
	}

	// Entry should now be mounted
	if !entry.mounted {
		t.Error("entry should be mounted after insert and rebuild")
	}
}

// TestOverlay_Remove_RemovesEntry verifies that Remove removes an entry from the overlay.
func TestOverlay_Remove_RemovesEntry(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	entry := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.Text{Content: "overlay content"}
	})

	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			state.Insert(entry, nil, nil)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pump to process the Dispatch callback and rebuild
	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// Remove the entry
	entry.Remove()
	tester.Pump()

	if entry.mounted {
		t.Error("entry should not be mounted after remove")
	}
	if entry.overlay != nil {
		t.Error("entry.overlay should be nil after remove")
	}
}

// TestOverlay_InsertAll_AddsMultipleEntries verifies that InsertAll adds multiple entries.
func TestOverlay_InsertAll_AddsMultipleEntries(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	entry1 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.Text{Content: "entry 1"}
	})
	entry2 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.Text{Content: "entry 2"}
	})

	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			state.InsertAll([]*OverlayEntry{entry1, entry2}, nil, nil)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pump to process the Dispatch callback and rebuild
	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if !entry1.mounted {
		t.Error("entry1 should be mounted")
	}
	if !entry2.mounted {
		t.Error("entry2 should be mounted")
	}
}

// TestOverlay_Rearrange_ReordersEntries verifies that Rearrange reorders entries.
func TestOverlay_Rearrange_ReordersEntries(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	var overlayState OverlayState
	entry1 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.Text{Content: "entry 1"}
	})
	entry2 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.Text{Content: "entry 2"}
	})

	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			overlayState = state
			state.InsertAll([]*OverlayEntry{entry1, entry2}, nil, nil)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pump to process the Dispatch callback and rebuild
	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// Rearrange to only include entry2
	overlayState.Rearrange([]*OverlayEntry{entry2})
	tester.Pump()

	if entry1.overlay != nil {
		t.Error("entry1 should be removed after rearrange")
	}
	if entry2.overlay == nil {
		t.Error("entry2 should still be in overlay after rearrange")
	}
}

// TestOverlay_Insert_PanicsOnBothBelowAndAbove verifies that Insert panics when both below and above are non-nil.
func TestOverlay_Insert_PanicsOnBothBelowAndAbove(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	var overlayState OverlayState
	entry1 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.SizedBox{}
	})
	entry2 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.SizedBox{}
	})
	entry3 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.SizedBox{}
	})

	panicCaught := false

	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			overlayState = state
			state.Insert(entry1, nil, nil)
			state.Insert(entry2, nil, nil)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pump to process the Dispatch callback
	tester.Pump()

	// This should panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicCaught = true
			}
		}()
		overlayState.Insert(entry3, entry1, entry2)
	}()

	if !panicCaught {
		t.Error("expected panic when both below and above are non-nil")
	}
}

// TestOverlay_Insert_PanicsOnAlreadyInserted verifies that Insert panics when entry is already inserted.
func TestOverlay_Insert_PanicsOnAlreadyInserted(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	var overlayState OverlayState
	entry := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.SizedBox{}
	})

	panicCaught := false

	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			overlayState = state
			state.Insert(entry, nil, nil)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pump to process the Dispatch callback
	tester.Pump()

	// This should panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicCaught = true
			}
		}()
		overlayState.Insert(entry, nil, nil)
	}()

	if !panicCaught {
		t.Error("expected panic when entry is already inserted")
	}
}

// TestOverlay_InitialEntries verifies that InitialEntries are added on mount.
func TestOverlay_InitialEntries(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	entry := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.Text{Content: "initial entry"}
	})

	err := tester.PumpWidget(Overlay{
		Child:          widgets.SizedBox{Width: 100, Height: 100},
		InitialEntries: []*OverlayEntry{entry},
	})
	if err != nil {
		t.Fatal(err)
	}

	if entry.overlay == nil {
		t.Error("initial entry should have overlay set")
	}
	if !entry.mounted {
		t.Error("initial entry should be mounted after build")
	}
}

// TestOverlay_OnOverlayReady_FiresOnce verifies that OnOverlayReady fires exactly once.
func TestOverlay_OnOverlayReady_FiresOnce(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	callCount := 0
	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			callCount++
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pump multiple times
	tester.Pump()
	tester.Pump()
	tester.Pump()

	if callCount != 1 {
		t.Errorf("expected OnOverlayReady to fire once, got %d calls", callCount)
	}
}

// TestModalBarrier_Build verifies that ModalBarrier builds correctly.
func TestModalBarrier_Build(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	err := tester.PumpWidget(ModalBarrier{
		Color:         0x80000000,
		Dismissible:   true,
		SemanticLabel: "Dismiss",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Just verify it doesn't crash
	ro := tester.RootRenderObject()
	if ro == nil {
		t.Fatal("expected render object")
	}
}

// TestOverlayEntry_Lifecycle verifies the complete entry lifecycle.
func TestOverlayEntry_Lifecycle(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	entry := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.SizedBox{}
	})

	// State: Created
	if entry.overlay != nil {
		t.Error("created: overlay should be nil")
	}
	if entry.mounted {
		t.Error("created: should not be mounted")
	}
	if entry.entryState != nil {
		t.Error("created: entryState should be nil")
	}

	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			state.Insert(entry, nil, nil)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// State: After Insert (before build)
	tester.Pump() // Process dispatch

	// State: After build (mounted)
	if entry.overlay == nil {
		t.Error("mounted: overlay should be set")
	}
	if !entry.mounted {
		t.Error("mounted: should be mounted")
	}
	if entry.entryState == nil {
		t.Error("mounted: entryState should be set")
	}

	// Remove
	entry.Remove()
	tester.Pump()

	// State: After Remove
	if entry.overlay != nil {
		t.Error("removed: overlay should be nil")
	}
	if entry.mounted {
		t.Error("removed: should not be mounted")
	}
	if entry.entryState != nil {
		t.Error("removed: entryState should be nil")
	}
}

// TestOverlay_OpaqueBlocksChild verifies that opaque entries block hit testing
// to the child (page content) but NOT to other overlay entries below them.
// This is essential for modal barriers to work - the barrier sits below the
// opaque dialog content but must still receive dismiss taps.
func TestOverlay_OpaqueBlocksChild(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	// Track which entries receive builds
	entry1Built := false
	entry2Built := false

	entry1 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		entry1Built = true
		return widgets.SizedBox{Width: 100, Height: 100}
	})
	// entry1 is the barrier - no special flags needed

	entry2 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		entry2Built = true
		return widgets.SizedBox{Width: 100, Height: 100}
	})
	entry2.Opaque = true // Blocks child but NOT entry1

	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			state.Insert(entry1, nil, nil) // Bottom (e.g., barrier)
			state.Insert(entry2, nil, nil) // Top (e.g., dialog content, opaque)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pump to process dispatch and rebuild
	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// Both entries should be built - all entries are always built
	if !entry1Built {
		t.Error("entry1 should be built")
	}
	if !entry2Built {
		t.Error("entry2 should be built")
	}

	// Both entries should be mounted
	if !entry1.mounted {
		t.Error("entry1 should be mounted (barrier must receive hits)")
	}
	if !entry2.mounted {
		t.Error("entry2 should be mounted")
	}
}

// TestOverlay_AllEntriesBuilt verifies that all entries are always built,
// regardless of opaque flags. This is essential for modal barriers to work.
func TestOverlay_AllEntriesBuilt(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	entry1BuildCount := 0
	entry1 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		entry1BuildCount++
		return widgets.SizedBox{Width: 50, Height: 50}
	})

	entry2 := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.SizedBox{Width: 100, Height: 100}
	})
	entry2.Opaque = true

	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			state.Insert(entry1, nil, nil) // Bottom
			state.Insert(entry2, nil, nil) // Top (opaque)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	// Entry1 should be built even though there's an opaque entry above
	if entry1BuildCount == 0 {
		t.Error("entry1 should be built (all entries are always built)")
	}
	if !entry1.mounted {
		t.Error("entry1 should be mounted")
	}
}

// TestOverlay_RemoveDuringBuild verifies that removing an already-inserted entry
// during build works correctly (no orphaned entries).
func TestOverlay_RemoveDuringBuild(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	var overlayState OverlayState
	entry := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.SizedBox{}
	})

	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			overlayState = state
			state.Insert(entry, nil, nil)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pump to mount the entry
	err = tester.PumpAndSettle(time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if !entry.mounted {
		t.Fatal("entry should be mounted before test")
	}

	// Now create a scenario where Remove is called during build
	// by inserting a new entry whose builder calls Remove on the first entry
	removerEntry := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		entry.Remove() // Remove during build
		return widgets.SizedBox{}
	})

	overlayState.Insert(removerEntry, nil, nil)
	tester.Pump() // This triggers build which calls Remove

	// After the build and queued operations complete, entry should be removed
	tester.Pump()

	if entry.overlay != nil {
		t.Error("entry.overlay should be nil after remove during build")
	}
	if entry.mounted {
		t.Error("entry should not be mounted after remove during build")
	}
}

// TestOverlay_OnOverlayReady_PostFrame verifies that OnOverlayReady fires
// after the build phase completes (via dispatch), not during build itself.
func TestOverlay_OnOverlayReady_PostFrame(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	buildPhaseActive := false
	readyFiredDuringBuild := false
	var readyState OverlayState

	onReady := func(state OverlayState) {
		// This should NOT fire while buildPhaseActive is true
		if buildPhaseActive {
			readyFiredDuringBuild = true
		}
		readyState = state
	}

	// Wrap the overlay to track build phase
	wrapper := testBuildTrackerWidget{
		onReady: onReady,
		onBuildStart: func() {
			buildPhaseActive = true
		},
		onBuildEnd: func() {
			buildPhaseActive = false
		},
	}

	err := tester.PumpWidget(wrapper)
	if err != nil {
		t.Fatal(err)
	}

	// Pump again to process the dispatched OnOverlayReady callback
	tester.Pump()

	// OnOverlayReady should have fired (via dispatch after build)
	if readyState == nil {
		t.Error("OnOverlayReady should have fired")
	}

	// But it should NOT have fired during the build phase
	if readyFiredDuringBuild {
		t.Error("OnOverlayReady should not fire during build phase (should be post-build via dispatch)")
	}
}

// testBuildTrackerWidget wraps an overlay and tracks build phase.
type testBuildTrackerWidget struct {
	onReady      func(state OverlayState)
	onBuildStart func()
	onBuildEnd   func()
}

func (w testBuildTrackerWidget) CreateElement() core.Element {
	return core.NewStatelessElement(w, nil)
}

func (w testBuildTrackerWidget) Key() any {
	return nil
}

func (w testBuildTrackerWidget) Build(ctx core.BuildContext) core.Widget {
	if w.onBuildStart != nil {
		w.onBuildStart()
	}
	defer func() {
		if w.onBuildEnd != nil {
			w.onBuildEnd()
		}
	}()

	return Overlay{
		Child:          widgets.SizedBox{Width: 100, Height: 100},
		OnOverlayReady: w.onReady,
	}
}

// TestOverlay_OnOverlayReady_InsertDuringCallback verifies that inserting
// entries during OnOverlayReady callback works correctly.
func TestOverlay_OnOverlayReady_InsertDuringCallback(t *testing.T) {
	tester := dtesting.NewWidgetTesterWithT(t)

	entry := NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return widgets.Text{Content: "inserted during ready"}
	})

	err := tester.PumpWidget(testOverlayWidget{
		onReady: func(state OverlayState) {
			// Insert during callback - should not cause re-entrancy issues
			state.Insert(entry, nil, nil)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Pump to fire OnOverlayReady and insert the entry
	tester.Pump()

	// Entry should have overlay set (insert was accepted)
	if entry.overlay == nil {
		t.Error("entry.overlay should be set after insert during OnOverlayReady")
	}

	// Pump again to rebuild with the new entry
	tester.Pump()

	// Entry should now be mounted
	if !entry.mounted {
		t.Error("entry should be mounted after rebuild")
	}
}

// =============================================================================
// Hit Testing Tests
// =============================================================================

// testHitRenderBox is a render box that tracks hit test calls.
type testHitRenderBox struct {
	layout.RenderBoxBase
	wasHitTested bool
	hitAccepts   bool // if true, returns true from HitTest
	size         graphics.Size
}

func (r *testHitRenderBox) PerformLayout() {
	r.SetSize(r.size)
}

func (r *testHitRenderBox) Paint(ctx *layout.PaintContext) {}

func (r *testHitRenderBox) HitTest(position graphics.Offset, result *layout.HitTestResult) bool {
	r.wasHitTested = true
	if r.hitAccepts && position.X >= 0 && position.Y >= 0 && position.X <= r.size.Width && position.Y <= r.size.Height {
		result.Add(r)
		return true
	}
	return false
}

func newTestHitRenderBox(width, height float64, accepts bool) *testHitRenderBox {
	r := &testHitRenderBox{
		size:       graphics.Size{Width: width, Height: height},
		hitAccepts: accepts,
	}
	r.SetSelf(r)
	return r
}

// TestRenderOverlay_HitTest_OpaqueBlocksChild verifies that when an opaque entry
// exists, hits don't reach the child (page content).
func TestRenderOverlay_HitTest_OpaqueBlocksChild(t *testing.T) {
	child := newTestHitRenderBox(400, 400, true)
	entry := newTestHitRenderBox(100, 100, false) // Entry at 100x100, doesn't accept hits

	r := &renderOverlay{
		opaqueIndex: 0, // First entry is opaque
		hasChild:    true,
	}
	r.SetSelf(r)
	r.SetChildren([]layout.RenderObject{child, entry})
	r.Layout(layout.Constraints{MaxWidth: 400, MaxHeight: 400}, true)

	// Hit at (200, 200) - outside entry bounds, should go to child but blocked by opaque
	result := &layout.HitTestResult{}
	hit := r.HitTest(graphics.Offset{X: 200, Y: 200}, result)

	if hit {
		t.Error("hit should be false - opaque entry blocks child")
	}
	if !entry.wasHitTested {
		t.Error("entry should have been hit tested")
	}
	if child.wasHitTested {
		t.Error("child should NOT have been hit tested - opaque blocks it")
	}
}

// TestRenderOverlay_HitTest_NoOpaquePassesToChild verifies that without opaque entries,
// hits pass through to the child.
func TestRenderOverlay_HitTest_NoOpaquePassesToChild(t *testing.T) {
	child := newTestHitRenderBox(400, 400, true)
	entry := newTestHitRenderBox(100, 100, false) // Entry at 100x100, doesn't accept hits

	r := &renderOverlay{
		opaqueIndex: -1, // No opaque entry
		hasChild:    true,
	}
	r.SetSelf(r)
	r.SetChildren([]layout.RenderObject{child, entry})
	r.Layout(layout.Constraints{MaxWidth: 400, MaxHeight: 400}, true)

	// Hit at (200, 200) - outside entry bounds, should reach child
	result := &layout.HitTestResult{}
	hit := r.HitTest(graphics.Offset{X: 200, Y: 200}, result)

	if !hit {
		t.Error("hit should be true - child accepts")
	}
	if !entry.wasHitTested {
		t.Error("entry should have been hit tested first")
	}
	if !child.wasHitTested {
		t.Error("child should have been hit tested - no opaque to block it")
	}
}

// TestRenderOverlay_HitTest_AllEntriesTested verifies that ALL overlay entries
// are hit tested even when an opaque entry exists (allows barriers to receive hits).
func TestRenderOverlay_HitTest_AllEntriesTested(t *testing.T) {
	child := newTestHitRenderBox(400, 400, true)
	barrier := newTestHitRenderBox(400, 400, true)  // Full-screen barrier below dialog
	dialog := newTestHitRenderBox(200, 200, false)  // Dialog doesn't accept hit at test position

	r := &renderOverlay{
		opaqueIndex: 1, // Dialog is opaque (second entry)
		hasChild:    true,
	}
	r.SetSelf(r)
	r.SetChildren([]layout.RenderObject{child, barrier, dialog})
	r.Layout(layout.Constraints{MaxWidth: 400, MaxHeight: 400}, true)

	// Hit at (300, 300) - outside dialog but inside barrier
	// Should test dialog first (top), then barrier (accepts)
	result := &layout.HitTestResult{}
	hit := r.HitTest(graphics.Offset{X: 300, Y: 300}, result)

	if !hit {
		t.Error("hit should be true - barrier accepts")
	}
	if !dialog.wasHitTested {
		t.Error("dialog should have been hit tested (top entry)")
	}
	if !barrier.wasHitTested {
		t.Error("barrier should have been hit tested (below opaque but still tested)")
	}
	if child.wasHitTested {
		t.Error("child should NOT have been hit tested - opaque blocks it")
	}
	if len(result.Entries) != 1 || result.Entries[0] != barrier {
		t.Error("only barrier should be in hit result")
	}
}

// TestRenderOverlay_HitTest_EntryAcceptsHit verifies that when an entry accepts a hit,
// hit testing stops (doesn't continue to entries below).
func TestRenderOverlay_HitTest_EntryAcceptsHit(t *testing.T) {
	child := newTestHitRenderBox(400, 400, true)
	bottomEntry := newTestHitRenderBox(400, 400, true) // Would accept if tested
	topEntry := newTestHitRenderBox(400, 400, true)    // Accepts hit first

	r := &renderOverlay{
		opaqueIndex: -1,
		hasChild:    true,
	}
	r.SetSelf(r)
	r.SetChildren([]layout.RenderObject{child, bottomEntry, topEntry})
	r.Layout(layout.Constraints{MaxWidth: 400, MaxHeight: 400}, true)

	result := &layout.HitTestResult{}
	hit := r.HitTest(graphics.Offset{X: 50, Y: 50}, result)

	if !hit {
		t.Error("hit should be true - topEntry accepts")
	}
	if !topEntry.wasHitTested {
		t.Error("topEntry should have been hit tested")
	}
	if bottomEntry.wasHitTested {
		t.Error("bottomEntry should NOT have been hit tested - topEntry already accepted")
	}
	if child.wasHitTested {
		t.Error("child should NOT have been hit tested")
	}
}

// TestRenderOverlay_HitTest_OutOfBounds verifies hits outside overlay bounds return false.
func TestRenderOverlay_HitTest_OutOfBounds(t *testing.T) {
	child := newTestHitRenderBox(400, 400, true)

	r := &renderOverlay{
		opaqueIndex: -1,
		hasChild:    true,
	}
	r.SetSelf(r)
	r.SetChildren([]layout.RenderObject{child})
	r.Layout(layout.Constraints{MaxWidth: 400, MaxHeight: 400}, true)

	result := &layout.HitTestResult{}
	hit := r.HitTest(graphics.Offset{X: 500, Y: 500}, result)

	if hit {
		t.Error("hit should be false - position is outside bounds")
	}
	if child.wasHitTested {
		t.Error("child should NOT have been hit tested - position out of bounds")
	}
}
