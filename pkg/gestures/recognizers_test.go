package gestures

import (
	"testing"

	"github.com/go-drift/drift/pkg/rendering"
)

func TestHorizontalDrag_WinsOnHorizontalSlop(t *testing.T) {
	arena := NewGestureArena()
	recognizer := NewHorizontalDragGestureRecognizer(arena)

	var started, ended bool
	var updateCount int
	var lastDelta float64

	recognizer.OnStart = func(d DragStartDetails) { started = true }
	recognizer.OnUpdate = func(d DragUpdateDetails) {
		updateCount++
		lastDelta = d.PrimaryDelta
	}
	recognizer.OnEnd = func(d DragEndDetails) { ended = true }

	// Pointer down
	recognizer.AddPointer(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100},
		Phase:     PointerPhaseDown,
	})
	arena.Close(1)

	// Move horizontally beyond slop
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100 + DefaultTouchSlop + 5, Y: 100},
		Phase:     PointerPhaseMove,
	})

	if !started {
		t.Error("OnStart should be called after horizontal slop exceeded")
	}

	// Continue moving
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100 + DefaultTouchSlop + 15, Y: 100},
		Phase:     PointerPhaseMove,
	})

	if updateCount < 1 {
		t.Error("OnUpdate should be called for movement after acceptance")
	}
	if lastDelta != 10 {
		t.Errorf("PrimaryDelta should be 10, got %f", lastDelta)
	}

	// Pointer up
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100 + DefaultTouchSlop + 15, Y: 100},
		Phase:     PointerPhaseUp,
	})

	if !ended {
		t.Error("OnEnd should be called on pointer up")
	}
}

func TestHorizontalDrag_RejectsOnVerticalSlop(t *testing.T) {
	arena := NewGestureArena()
	recognizer := NewHorizontalDragGestureRecognizer(arena)

	var started bool
	recognizer.OnStart = func(d DragStartDetails) { started = true }

	// Pointer down
	recognizer.AddPointer(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100},
		Phase:     PointerPhaseDown,
	})
	arena.Close(1)

	// Move vertically beyond slop
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100 + DefaultTouchSlop + 5},
		Phase:     PointerPhaseMove,
	})

	if started {
		t.Error("OnStart should NOT be called when vertical movement exceeds slop first")
	}
}

func TestVerticalDrag_WinsOnVerticalSlop(t *testing.T) {
	arena := NewGestureArena()
	recognizer := NewVerticalDragGestureRecognizer(arena)

	var started, ended bool
	var updateCount int
	var lastDelta float64

	recognizer.OnStart = func(d DragStartDetails) { started = true }
	recognizer.OnUpdate = func(d DragUpdateDetails) {
		updateCount++
		lastDelta = d.PrimaryDelta
	}
	recognizer.OnEnd = func(d DragEndDetails) { ended = true }

	// Pointer down
	recognizer.AddPointer(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100},
		Phase:     PointerPhaseDown,
	})
	arena.Close(1)

	// Move vertically beyond slop
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100 + DefaultTouchSlop + 5},
		Phase:     PointerPhaseMove,
	})

	if !started {
		t.Error("OnStart should be called after vertical slop exceeded")
	}

	// Continue moving
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100 + DefaultTouchSlop + 25},
		Phase:     PointerPhaseMove,
	})

	if updateCount < 1 {
		t.Error("OnUpdate should be called for movement after acceptance")
	}
	if lastDelta != 20 {
		t.Errorf("PrimaryDelta should be 20, got %f", lastDelta)
	}

	// Pointer up
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100 + DefaultTouchSlop + 25},
		Phase:     PointerPhaseUp,
	})

	if !ended {
		t.Error("OnEnd should be called on pointer up")
	}
}

func TestVerticalDrag_RejectsOnHorizontalSlop(t *testing.T) {
	arena := NewGestureArena()
	recognizer := NewVerticalDragGestureRecognizer(arena)

	var started bool
	recognizer.OnStart = func(d DragStartDetails) { started = true }

	// Pointer down
	recognizer.AddPointer(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100},
		Phase:     PointerPhaseDown,
	})
	arena.Close(1)

	// Move horizontally beyond slop
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100 + DefaultTouchSlop + 5, Y: 100},
		Phase:     PointerPhaseMove,
	})

	if started {
		t.Error("OnStart should NOT be called when horizontal movement exceeds slop first")
	}
}

func TestDrag_VelocityCalculation(t *testing.T) {
	arena := NewGestureArena()
	recognizer := NewHorizontalDragGestureRecognizer(arena)

	var endVelocity float64
	recognizer.OnEnd = func(d DragEndDetails) {
		endVelocity = d.PrimaryVelocity
	}

	// Pointer down
	recognizer.AddPointer(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100},
		Phase:     PointerPhaseDown,
	})
	arena.Close(1)

	// Move to trigger acceptance
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100 + DefaultTouchSlop + 10, Y: 100},
		Phase:     PointerPhaseMove,
	})

	// Multiple rapid moves to build velocity
	for i := 0; i < 5; i++ {
		recognizer.HandleEvent(PointerEvent{
			PointerID: 1,
			Position:  rendering.Offset{X: 100 + DefaultTouchSlop + 10 + float64(i+1)*50, Y: 100},
			Phase:     PointerPhaseMove,
		})
	}

	// Pointer up
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100 + DefaultTouchSlop + 260, Y: 100},
		Phase:     PointerPhaseUp,
	})

	// Velocity should be non-zero after rapid movement
	if endVelocity == 0 {
		t.Error("PrimaryVelocity should be non-zero after rapid movement")
	}
}

func TestDrag_CompetesWithTap(t *testing.T) {
	arena := NewGestureArena()
	horizontal := NewHorizontalDragGestureRecognizer(arena)
	tap := NewTapGestureRecognizer(arena)

	var dragStarted, tapFired bool
	horizontal.OnStart = func(d DragStartDetails) { dragStarted = true }
	tap.OnTap = func() { tapFired = true }

	// Pointer down - both recognizers enter arena
	down := PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100},
		Phase:     PointerPhaseDown,
	}
	horizontal.AddPointer(down)
	tap.AddPointer(down)
	arena.Close(1)

	// Move horizontally - drag should win
	move := PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100 + DefaultTouchSlop + 10, Y: 100},
		Phase:     PointerPhaseMove,
	}
	horizontal.HandleEvent(move)
	tap.HandleEvent(move)

	// Pointer up
	up := PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100 + DefaultTouchSlop + 10, Y: 100},
		Phase:     PointerPhaseUp,
	}
	horizontal.HandleEvent(up)
	tap.HandleEvent(up)

	if !dragStarted {
		t.Error("Drag should have won and started")
	}
	if tapFired {
		t.Error("Tap should NOT have fired when drag won")
	}
}

func TestDrag_HorizontalVsVertical(t *testing.T) {
	arena := NewGestureArena()
	horizontal := NewHorizontalDragGestureRecognizer(arena)
	vertical := NewVerticalDragGestureRecognizer(arena)

	var horizontalStarted, verticalStarted bool
	horizontal.OnStart = func(d DragStartDetails) { horizontalStarted = true }
	vertical.OnStart = func(d DragStartDetails) { verticalStarted = true }

	// Pointer down - both enter arena
	down := PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100},
		Phase:     PointerPhaseDown,
	}
	horizontal.AddPointer(down)
	vertical.AddPointer(down)
	arena.Close(1)

	// Move diagonally but more horizontal
	move := PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100 + DefaultTouchSlop + 10, Y: 100 + 5},
		Phase:     PointerPhaseMove,
	}
	horizontal.HandleEvent(move)
	vertical.HandleEvent(move)

	if !horizontalStarted {
		t.Error("Horizontal drag should win when X delta > Y delta")
	}
	if verticalStarted {
		t.Error("Vertical drag should be rejected when X delta > Y delta")
	}
}

func TestDrag_Cancel(t *testing.T) {
	arena := NewGestureArena()
	recognizer := NewHorizontalDragGestureRecognizer(arena)

	var cancelled bool
	recognizer.OnCancel = func() { cancelled = true }

	// Pointer down
	recognizer.AddPointer(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100},
		Phase:     PointerPhaseDown,
	})
	arena.Close(1)

	// Move to trigger acceptance
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100 + DefaultTouchSlop + 10, Y: 100},
		Phase:     PointerPhaseMove,
	})

	// Cancel
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100},
		Phase:     PointerPhaseCancel,
	})

	if !cancelled {
		t.Error("OnCancel should be called on pointer cancel after acceptance")
	}
}

func TestDrag_NegativeDirection(t *testing.T) {
	arena := NewGestureArena()
	recognizer := NewHorizontalDragGestureRecognizer(arena)

	var lastDelta float64
	recognizer.OnUpdate = func(d DragUpdateDetails) {
		lastDelta = d.PrimaryDelta
	}

	// Pointer down
	recognizer.AddPointer(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100},
		Phase:     PointerPhaseDown,
	})
	arena.Close(1)

	// Move left (negative X)
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100 - DefaultTouchSlop - 10, Y: 100},
		Phase:     PointerPhaseMove,
	})

	// Move further left
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100 - DefaultTouchSlop - 30, Y: 100},
		Phase:     PointerPhaseMove,
	})

	if lastDelta >= 0 {
		t.Errorf("PrimaryDelta should be negative for leftward movement, got %f", lastDelta)
	}
}

func TestDrag_PointerUpWithoutAcceptance(t *testing.T) {
	arena := NewGestureArena()
	recognizer := NewHorizontalDragGestureRecognizer(arena)

	var started, ended bool
	recognizer.OnStart = func(d DragStartDetails) { started = true }
	recognizer.OnEnd = func(d DragEndDetails) { ended = true }

	// Pointer down
	recognizer.AddPointer(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100},
		Phase:     PointerPhaseDown,
	})
	arena.Close(1)

	// Immediate pointer up (no movement)
	recognizer.HandleEvent(PointerEvent{
		PointerID: 1,
		Position:  rendering.Offset{X: 100, Y: 100},
		Phase:     PointerPhaseUp,
	})

	if started {
		t.Error("OnStart should NOT be called without movement")
	}
	if ended {
		t.Error("OnEnd should NOT be called without acceptance")
	}
}
