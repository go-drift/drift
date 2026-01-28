package widgets

import (
	"testing"

	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/graphics"
)

func TestGestureDetector_HorizontalDrag(t *testing.T) {
	var started, updated, ended bool
	var lastDelta float64

	gd := GestureDetector{
		OnHorizontalDragStart: func(d DragStartDetails) {
			started = true
		},
		OnHorizontalDragUpdate: func(d DragUpdateDetails) {
			updated = true
			lastDelta = d.PrimaryDelta
		},
		OnHorizontalDragEnd: func(d DragEndDetails) {
			ended = true
		},
	}

	// Create render object
	detector := &renderGestureDetector{}
	detector.SetSelf(detector)
	detector.configure(gd)

	if detector.horizontalDrag == nil {
		t.Fatal("horizontalDrag recognizer should be created")
	}

	// Simulate gesture
	down := gestures.PointerEvent{
		PointerID: 1,
		Position:  graphics.Offset{X: 100, Y: 100},
		Phase:     gestures.PointerPhaseDown,
	}
	detector.HandlePointer(down)
	gestures.DefaultArena.Close(1)

	move := gestures.PointerEvent{
		PointerID: 1,
		Position:  graphics.Offset{X: 100 + gestures.DefaultTouchSlop + 20, Y: 100},
		Phase:     gestures.PointerPhaseMove,
	}
	detector.HandlePointer(move)

	move2 := gestures.PointerEvent{
		PointerID: 1,
		Position:  graphics.Offset{X: 100 + gestures.DefaultTouchSlop + 40, Y: 100},
		Phase:     gestures.PointerPhaseMove,
	}
	detector.HandlePointer(move2)

	up := gestures.PointerEvent{
		PointerID: 1,
		Position:  graphics.Offset{X: 100 + gestures.DefaultTouchSlop + 40, Y: 100},
		Phase:     gestures.PointerPhaseUp,
	}
	detector.HandlePointer(up)

	// Clean up arena for next test
	gestures.DefaultArena.Sweep(1)

	if !started {
		t.Error("OnHorizontalDragStart should be called")
	}
	if !updated {
		t.Error("OnHorizontalDragUpdate should be called")
	}
	if lastDelta != 20 {
		t.Errorf("PrimaryDelta should be 20, got %f", lastDelta)
	}
	if !ended {
		t.Error("OnHorizontalDragEnd should be called")
	}
}

func TestGestureDetector_VerticalDrag(t *testing.T) {
	var started, updated, ended bool
	var lastDelta float64

	gd := GestureDetector{
		OnVerticalDragStart: func(d DragStartDetails) {
			started = true
		},
		OnVerticalDragUpdate: func(d DragUpdateDetails) {
			updated = true
			lastDelta = d.PrimaryDelta
		},
		OnVerticalDragEnd: func(d DragEndDetails) {
			ended = true
		},
	}

	// Create render object
	detector := &renderGestureDetector{}
	detector.SetSelf(detector)
	detector.configure(gd)

	if detector.verticalDrag == nil {
		t.Fatal("verticalDrag recognizer should be created")
	}

	// Simulate gesture
	down := gestures.PointerEvent{
		PointerID: 2,
		Position:  graphics.Offset{X: 100, Y: 100},
		Phase:     gestures.PointerPhaseDown,
	}
	detector.HandlePointer(down)
	gestures.DefaultArena.Close(2)

	move := gestures.PointerEvent{
		PointerID: 2,
		Position:  graphics.Offset{X: 100, Y: 100 + gestures.DefaultTouchSlop + 30},
		Phase:     gestures.PointerPhaseMove,
	}
	detector.HandlePointer(move)

	move2 := gestures.PointerEvent{
		PointerID: 2,
		Position:  graphics.Offset{X: 100, Y: 100 + gestures.DefaultTouchSlop + 60},
		Phase:     gestures.PointerPhaseMove,
	}
	detector.HandlePointer(move2)

	up := gestures.PointerEvent{
		PointerID: 2,
		Position:  graphics.Offset{X: 100, Y: 100 + gestures.DefaultTouchSlop + 60},
		Phase:     gestures.PointerPhaseUp,
	}
	detector.HandlePointer(up)

	// Clean up arena for next test
	gestures.DefaultArena.Sweep(2)

	if !started {
		t.Error("OnVerticalDragStart should be called")
	}
	if !updated {
		t.Error("OnVerticalDragUpdate should be called")
	}
	if lastDelta != 30 {
		t.Errorf("PrimaryDelta should be 30, got %f", lastDelta)
	}
	if !ended {
		t.Error("OnVerticalDragEnd should be called")
	}
}

func TestGestureDetector_RecognizerDisposal(t *testing.T) {
	// Start with horizontal drag
	gd1 := GestureDetector{
		OnHorizontalDragStart: func(d DragStartDetails) {},
	}

	detector := &renderGestureDetector{}
	detector.SetSelf(detector)
	detector.configure(gd1)

	if detector.horizontalDrag == nil {
		t.Error("horizontalDrag should be created")
	}
	if detector.verticalDrag != nil {
		t.Error("verticalDrag should be nil")
	}

	// Reconfigure without horizontal drag
	gd2 := GestureDetector{}
	detector.configure(gd2)

	if detector.horizontalDrag != nil {
		t.Error("horizontalDrag should be disposed")
	}
}

func TestGestureDetector_MultipleRecognizers(t *testing.T) {
	var tapFired, horizontalStarted bool

	gd := GestureDetector{
		OnTap: func() {
			tapFired = true
		},
		OnHorizontalDragStart: func(d DragStartDetails) {
			horizontalStarted = true
		},
	}

	detector := &renderGestureDetector{}
	detector.SetSelf(detector)
	detector.configure(gd)

	if detector.tap == nil {
		t.Error("tap recognizer should be created")
	}
	if detector.horizontalDrag == nil {
		t.Error("horizontalDrag recognizer should be created")
	}

	// Simulate drag - tap should lose
	down := gestures.PointerEvent{
		PointerID: 3,
		Position:  graphics.Offset{X: 100, Y: 100},
		Phase:     gestures.PointerPhaseDown,
	}
	detector.HandlePointer(down)
	gestures.DefaultArena.Close(3)

	move := gestures.PointerEvent{
		PointerID: 3,
		Position:  graphics.Offset{X: 100 + gestures.DefaultTouchSlop + 20, Y: 100},
		Phase:     gestures.PointerPhaseMove,
	}
	detector.HandlePointer(move)

	up := gestures.PointerEvent{
		PointerID: 3,
		Position:  graphics.Offset{X: 100 + gestures.DefaultTouchSlop + 20, Y: 100},
		Phase:     gestures.PointerPhaseUp,
	}
	detector.HandlePointer(up)

	gestures.DefaultArena.Sweep(3)

	if !horizontalStarted {
		t.Error("Horizontal drag should have won")
	}
	if tapFired {
		t.Error("Tap should NOT have fired when drag won")
	}
}
