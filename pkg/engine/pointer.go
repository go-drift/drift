// Package engine provides the core rendering logic for the Drift framework.
// It renders a widget tree into backend-specific surfaces consumed by native embedders.
package engine

// PointerPhase represents the phase of a pointer/touch event.
type PointerPhase int

const (
	PointerPhaseDown PointerPhase = iota
	PointerPhaseMove
	PointerPhaseUp
	PointerPhaseCancel
)

// PointerEvent represents a raw pointer/touch event from the native embedder.
// This is a simplified event type with screen coordinates; the engine converts
// it to gestures.PointerEvent for internal routing.
type PointerEvent struct {
	X     float64
	Y     float64
	Phase PointerPhase
}

// HandlePointerEvent receives a pointer event from the native layer and
// forwards it to the app runner for hit testing and gesture recognition.
func HandlePointerEvent(event PointerEvent) {
	app.HandlePointer(event)
}
