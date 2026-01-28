package widgets

import (
	"testing"

	"github.com/go-drift/drift/pkg/gestures"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
)

func TestScrollView_VerticalScrollRejectsHorizontal(t *testing.T) {
	// Create a vertical scroll view
	scroll := &renderScrollView{
		direction: AxisVertical,
		physics:   ClampingScrollPhysics{},
	}
	scroll.SetSelf(scroll)
	scroll.position = NewScrollPosition(nil, scroll.physics, func() {})
	scroll.configurePan()

	if scroll.verticalDrag == nil {
		t.Fatal("verticalDrag recognizer should be created for vertical scroll")
	}
	if scroll.horizontalDrag != nil {
		t.Error("horizontalDrag should be nil for vertical scroll")
	}

	// Track scroll updates
	var scrolled bool
	originalOffset := scroll.position.Offset()
	scroll.position.onUpdate = func() {
		if scroll.position.Offset() != originalOffset {
			scrolled = true
		}
	}

	// Simulate horizontal drag (should be rejected)
	down := gestures.PointerEvent{
		PointerID: 10,
		Position:  graphics.Offset{X: 100, Y: 100},
		Phase:     gestures.PointerPhaseDown,
	}
	scroll.HandlePointer(down)
	gestures.DefaultArena.Close(10)

	// Move horizontally
	move := gestures.PointerEvent{
		PointerID: 10,
		Position:  graphics.Offset{X: 100 + gestures.DefaultTouchSlop + 30, Y: 100},
		Phase:     gestures.PointerPhaseMove,
	}
	scroll.HandlePointer(move)

	up := gestures.PointerEvent{
		PointerID: 10,
		Position:  graphics.Offset{X: 100 + gestures.DefaultTouchSlop + 30, Y: 100},
		Phase:     gestures.PointerPhaseUp,
	}
	scroll.HandlePointer(up)

	gestures.DefaultArena.Sweep(10)

	if scrolled {
		t.Error("Vertical ScrollView should NOT scroll from horizontal drag")
	}
}

func TestScrollView_VerticalScrollAcceptsVertical(t *testing.T) {
	// Create a vertical scroll view with content
	scroll := &renderScrollView{
		direction: AxisVertical,
		physics:   ClampingScrollPhysics{},
	}
	scroll.SetSelf(scroll)
	scroll.SetSize(graphics.Size{Width: 400, Height: 600})
	scroll.position = NewScrollPosition(nil, scroll.physics, func() {})
	scroll.position.SetExtents(0, 1000) // Content is taller than viewport
	scroll.configurePan()

	// Track scroll updates
	var scrolled bool
	scroll.position.onUpdate = func() {
		scrolled = true
	}

	// Simulate vertical drag (should be accepted)
	down := gestures.PointerEvent{
		PointerID: 11,
		Position:  graphics.Offset{X: 100, Y: 100},
		Phase:     gestures.PointerPhaseDown,
	}
	scroll.HandlePointer(down)
	gestures.DefaultArena.Close(11)

	// Move vertically
	move := gestures.PointerEvent{
		PointerID: 11,
		Position:  graphics.Offset{X: 100, Y: 100 + gestures.DefaultTouchSlop + 50},
		Phase:     gestures.PointerPhaseMove,
	}
	scroll.HandlePointer(move)

	// Continue moving
	move2 := gestures.PointerEvent{
		PointerID: 11,
		Position:  graphics.Offset{X: 100, Y: 100 + gestures.DefaultTouchSlop + 100},
		Phase:     gestures.PointerPhaseMove,
	}
	scroll.HandlePointer(move2)

	up := gestures.PointerEvent{
		PointerID: 11,
		Position:  graphics.Offset{X: 100, Y: 100 + gestures.DefaultTouchSlop + 100},
		Phase:     gestures.PointerPhaseUp,
	}
	scroll.HandlePointer(up)

	gestures.DefaultArena.Sweep(11)

	if !scrolled {
		t.Error("Vertical ScrollView SHOULD scroll from vertical drag")
	}
}

func TestScrollView_HorizontalScrollRejectsVertical(t *testing.T) {
	// Create a horizontal scroll view
	scroll := &renderScrollView{
		direction: AxisHorizontal,
		physics:   ClampingScrollPhysics{},
	}
	scroll.SetSelf(scroll)
	scroll.position = NewScrollPosition(nil, scroll.physics, func() {})
	scroll.configurePan()

	if scroll.horizontalDrag == nil {
		t.Fatal("horizontalDrag recognizer should be created for horizontal scroll")
	}
	if scroll.verticalDrag != nil {
		t.Error("verticalDrag should be nil for horizontal scroll")
	}

	// Track scroll updates
	var scrolled bool
	originalOffset := scroll.position.Offset()
	scroll.position.onUpdate = func() {
		if scroll.position.Offset() != originalOffset {
			scrolled = true
		}
	}

	// Simulate vertical drag (should be rejected)
	down := gestures.PointerEvent{
		PointerID: 12,
		Position:  graphics.Offset{X: 100, Y: 100},
		Phase:     gestures.PointerPhaseDown,
	}
	scroll.HandlePointer(down)
	gestures.DefaultArena.Close(12)

	// Move vertically
	move := gestures.PointerEvent{
		PointerID: 12,
		Position:  graphics.Offset{X: 100, Y: 100 + gestures.DefaultTouchSlop + 30},
		Phase:     gestures.PointerPhaseMove,
	}
	scroll.HandlePointer(move)

	up := gestures.PointerEvent{
		PointerID: 12,
		Position:  graphics.Offset{X: 100, Y: 100 + gestures.DefaultTouchSlop + 30},
		Phase:     gestures.PointerPhaseUp,
	}
	scroll.HandlePointer(up)

	gestures.DefaultArena.Sweep(12)

	if scrolled {
		t.Error("Horizontal ScrollView should NOT scroll from vertical drag")
	}
}

func TestScrollView_HorizontalScrollAcceptsHorizontal(t *testing.T) {
	// Create a horizontal scroll view with content
	scroll := &renderScrollView{
		direction: AxisHorizontal,
		physics:   ClampingScrollPhysics{},
	}
	scroll.SetSelf(scroll)
	scroll.SetSize(graphics.Size{Width: 400, Height: 600})
	scroll.position = NewScrollPosition(nil, scroll.physics, func() {})
	scroll.position.SetExtents(0, 1000) // Content is wider than viewport
	scroll.configurePan()

	// Track scroll updates
	var scrolled bool
	scroll.position.onUpdate = func() {
		scrolled = true
	}

	// Simulate horizontal drag (should be accepted)
	down := gestures.PointerEvent{
		PointerID: 13,
		Position:  graphics.Offset{X: 100, Y: 100},
		Phase:     gestures.PointerPhaseDown,
	}
	scroll.HandlePointer(down)
	gestures.DefaultArena.Close(13)

	// Move horizontally
	move := gestures.PointerEvent{
		PointerID: 13,
		Position:  graphics.Offset{X: 100 + gestures.DefaultTouchSlop + 50, Y: 100},
		Phase:     gestures.PointerPhaseMove,
	}
	scroll.HandlePointer(move)

	// Continue moving
	move2 := gestures.PointerEvent{
		PointerID: 13,
		Position:  graphics.Offset{X: 100 + gestures.DefaultTouchSlop + 100, Y: 100},
		Phase:     gestures.PointerPhaseMove,
	}
	scroll.HandlePointer(move2)

	up := gestures.PointerEvent{
		PointerID: 13,
		Position:  graphics.Offset{X: 100 + gestures.DefaultTouchSlop + 100, Y: 100},
		Phase:     gestures.PointerPhaseUp,
	}
	scroll.HandlePointer(up)

	gestures.DefaultArena.Sweep(13)

	if !scrolled {
		t.Error("Horizontal ScrollView SHOULD scroll from horizontal drag")
	}
}

func TestScrollView_DirectionChange(t *testing.T) {
	scroll := &renderScrollView{
		direction: AxisVertical,
		physics:   ClampingScrollPhysics{},
	}
	scroll.SetSelf(scroll)
	scroll.position = NewScrollPosition(nil, scroll.physics, func() {})
	scroll.configurePan()

	if scroll.verticalDrag == nil {
		t.Error("Should have verticalDrag for AxisVertical")
	}
	if scroll.horizontalDrag != nil {
		t.Error("Should NOT have horizontalDrag for AxisVertical")
	}

	// Change direction
	scroll.direction = AxisHorizontal
	scroll.configurePan()

	if scroll.horizontalDrag == nil {
		t.Error("Should have horizontalDrag after direction change")
	}
	if scroll.verticalDrag != nil {
		t.Error("verticalDrag should be disposed after direction change")
	}
}

func TestScrollView_PrimaryVelocityUsed(t *testing.T) {
	scroll := &renderScrollView{
		direction: AxisVertical,
		physics:   ClampingScrollPhysics{},
	}
	scroll.SetSelf(scroll)
	scroll.SetSize(graphics.Size{Width: 400, Height: 600})
	scroll.position = NewScrollPosition(nil, scroll.physics, func() {})
	scroll.position.SetExtents(0, 1000)
	scroll.configurePan()

	// The configureDrag should set up handlers that use PrimaryDelta and PrimaryVelocity
	// This test verifies the recognizer is wired up correctly by checking the handler exists
	if scroll.verticalDrag.OnUpdate == nil {
		t.Error("OnUpdate handler should be set")
	}
	if scroll.verticalDrag.OnEnd == nil {
		t.Error("OnEnd handler should be set")
	}
}

// mockRenderBox is a minimal RenderBox implementation for testing.
type mockRenderBox struct {
	layout.RenderBoxBase
}

func (m *mockRenderBox) PerformLayout() {
	constraints := m.Constraints()
	m.SetSize(graphics.Size{Width: constraints.MaxWidth, Height: 2000})
}

func (m *mockRenderBox) Paint(ctx *layout.PaintContext) {}
