// Package gestures provides gesture recognition for pointer input, including
// an arena for resolving competing recognizers and common gesture types like
// tap and pan.
package gestures

import "github.com/go-drift/drift/pkg/rendering"

// PointerPhase represents the phase of a pointer event.
type PointerPhase int

const (
	// PointerPhaseDown indicates the pointer made contact with the surface.
	PointerPhaseDown PointerPhase = iota
	// PointerPhaseMove indicates the pointer moved while in contact.
	PointerPhaseMove
	// PointerPhaseUp indicates the pointer lifted from the surface.
	PointerPhaseUp
	// PointerPhaseCancel indicates the pointer interaction was cancelled.
	PointerPhaseCancel
)

// String returns the string representation of the pointer phase.
func (p PointerPhase) String() string {
	switch p {
	case PointerPhaseDown:
		return "down"
	case PointerPhaseMove:
		return "move"
	case PointerPhaseUp:
		return "up"
	case PointerPhaseCancel:
		return "cancel"
	default:
		return "unknown"
	}
}

// PointerEvent represents a pointer event routed to gesture recognizers.
type PointerEvent struct {
	// PointerID uniquely identifies this pointer (finger/mouse).
	PointerID int64
	// Position is the pointer location in logical pixels.
	Position rendering.Offset
	// Delta is the change in position since the last event.
	Delta rendering.Offset
	// Phase indicates the current phase of the pointer interaction.
	Phase PointerPhase
}

// DefaultTouchSlop is the movement threshold before a drag wins a gesture.
var DefaultTouchSlop = 12.0
