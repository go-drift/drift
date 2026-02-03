package navigation

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/overlay"
	"github.com/go-drift/drift/pkg/widgets"
)

// DefaultBarrierColor is the default semi-transparent black used for modal barriers.
var DefaultBarrierColor = graphics.RGBA(0, 0, 0, 128) // 50% opacity black

// ModalRoute is a route that displays as a modal overlay with a barrier.
// The modal content appears above a semi-transparent barrier that can
// optionally dismiss the modal when tapped.
type ModalRoute struct {
	BaseRoute
	builder func(ctx core.BuildContext) core.Widget

	// BarrierDismissible controls whether tapping the barrier dismisses the modal.
	BarrierDismissible bool

	// BarrierColor is the color of the semi-transparent barrier.
	// If nil, defaults to DefaultBarrierColor.
	// Set to a pointer to graphics.Color(0) for a fully transparent barrier.
	BarrierColor *graphics.Color

	// BarrierLabel is the accessibility label for the barrier.
	BarrierLabel string

	// internal
	overlayState   OverlayState
	barrierEntry   *overlay.OverlayEntry
	contentEntry   *overlay.OverlayEntry
	didPushPending bool // true if DidPush called before SetOverlay
}

// NewModalRoute creates a new ModalRoute with the given builder and settings.
// By default, the barrier is dismissible and uses DefaultBarrierColor.
func NewModalRoute(builder func(ctx core.BuildContext) core.Widget, settings RouteSettings) *ModalRoute {
	defaultColor := DefaultBarrierColor
	return &ModalRoute{
		BaseRoute:          NewBaseRoute(settings),
		builder:            builder,
		BarrierDismissible: true,
		BarrierColor:       &defaultColor,
		BarrierLabel:       "Dismiss",
	}
}

// SetOverlay is called by Navigator when OverlayState becomes available.
func (r *ModalRoute) SetOverlay(o OverlayState) {
	wasNil := r.overlayState == nil
	r.overlayState = o
	// If DidPush was called before overlay was ready, insert entries now
	if wasNil && r.didPushPending {
		r.didPushPending = false
		r.insertEntries()
	}
}

// DidPush is called when the route is pushed onto the navigator.
func (r *ModalRoute) DidPush() {
	if r.overlayState == nil {
		// Overlay not ready yet - defer entry insertion
		r.didPushPending = true
		return
	}
	r.insertEntries()
}

func (r *ModalRoute) insertEntries() {
	// Create barrier entry
	barrierColor := DefaultBarrierColor
	if r.BarrierColor != nil {
		barrierColor = *r.BarrierColor
	}

	r.barrierEntry = overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		return overlay.ModalBarrier{
			Color:         barrierColor,
			Dismissible:   r.BarrierDismissible,
			OnDismiss:     func() { NavigatorOf(ctx).Pop(nil) },
			SemanticLabel: r.BarrierLabel,
		}
	})
	r.barrierEntry.Opaque = false // Don't block hit testing everywhere

	// Create content entry
	r.contentEntry = overlay.NewOverlayEntry(r.builder)
	r.contentEntry.Opaque = true // Block hit testing everywhere below
	r.contentEntry.MaintainState = false

	// Insert barrier then content (content on top)
	r.overlayState.Insert(r.barrierEntry, nil, nil)
	r.overlayState.Insert(r.contentEntry, nil, nil)
}

// DidPop is called when the route is popped from the navigator.
func (r *ModalRoute) DidPop(result any) {
	r.didPushPending = false
	if r.barrierEntry != nil {
		r.barrierEntry.Remove()
		r.barrierEntry = nil
	}
	if r.contentEntry != nil {
		r.contentEntry.Remove()
		r.contentEntry = nil
	}
}

// Build returns the widget for this route.
// When overlay is available and entries inserted, returns a placeholder.
// Falls back to direct rendering if no overlay is available.
func (r *ModalRoute) Build(ctx core.BuildContext) core.Widget {
	// When overlay is available and entries inserted, render placeholder
	// (Navigator triggers rebuild via SetState after OnOverlayReady)
	if r.overlayState != nil && r.barrierEntry != nil {
		return widgets.SizedBox{} // Placeholder - content is in overlay
	}
	// Fallback: render directly (no barrier, but at least visible)
	if r.builder == nil {
		return nil
	}
	return r.builder(ctx)
}
