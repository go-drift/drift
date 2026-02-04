package navigation

import (
	"sync"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/overlay"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// BottomSheetRoute displays content as a modal bottom sheet.
// The sheet slides up from the bottom and can be dismissed by dragging down
// or tapping the barrier (if enabled).
type BottomSheetRoute struct {
	BaseRoute
	builder func(ctx core.BuildContext) core.Widget

	// SnapPoints defines heights where the sheet can rest.
	// If empty, defaults to [widgets.DefaultSnapPoints].
	SnapPoints []widgets.SnapPoint

	// InitialSnapPoint is the index into SnapPoints where the sheet starts.
	// Invalid values are clamped to valid range.
	InitialSnapPoint int

	// BarrierDismissible controls whether tapping the barrier dismisses the sheet.
	// Defaults to true.
	BarrierDismissible bool

	// BarrierColor is the color of the semi-transparent barrier behind the sheet.
	// If nil, uses theme default.
	BarrierColor *graphics.Color

	// EnableDrag controls whether the sheet can be dragged.
	// Defaults to true.
	EnableDrag bool

	// DragMode controls how drag gestures interact with sheet content.
	// Defaults to DragModeAuto.
	DragMode widgets.DragMode

	// ShowHandle controls whether a drag handle is displayed at the top of the sheet.
	// Defaults to true.
	ShowHandle bool

	// UseSafeArea controls whether the sheet respects the bottom safe area inset.
	// Defaults to true.
	UseSafeArea bool

	// SnapBehavior customizes snapping and dismiss thresholds.
	SnapBehavior widgets.SnapBehavior

	// internal
	overlayState   OverlayState
	barrierEntry   *overlay.OverlayEntry
	sheetEntry     *overlay.OverlayEntry
	controller     *widgets.BottomSheetController
	progressRemove func()
	didPushPending bool
	poppedFromNav  bool
	pushingNav     NavigatorState
	onDismiss      func(any)
	dismissed      bool
	mu             sync.Mutex
}

// NewBottomSheetRoute creates a new BottomSheetRoute with sensible defaults.
func NewBottomSheetRoute(
	builder func(ctx core.BuildContext) core.Widget,
	settings RouteSettings,
) *BottomSheetRoute {
	return &BottomSheetRoute{
		BaseRoute:          NewBaseRoute(settings),
		builder:            builder,
		SnapPoints:         nil, // Will use defaults
		InitialSnapPoint:   0,
		BarrierDismissible: true,
		BarrierColor:       nil, // Will use theme
		EnableDrag:         true,
		DragMode:           widgets.DragModeAuto,
		ShowHandle:         true,
		UseSafeArea:        true,
	}
}

// SetOverlay is called by Navigator when OverlayState becomes available.
func (r *BottomSheetRoute) SetOverlay(o OverlayState) {
	wasNil := r.overlayState == nil
	r.overlayState = o
	// If DidPush was called before overlay was ready, insert entries now
	if wasNil && r.didPushPending {
		r.didPushPending = false
		r.insertEntries()
	}
}

// DidPush is called when the route is pushed onto the navigator.
func (r *BottomSheetRoute) DidPush() {
	if r.pushingNav == nil {
		r.pushingNav = globalScope.ActiveNavigator()
	}
	if r.overlayState == nil {
		// Overlay not ready yet - defer entry insertion
		r.didPushPending = true
		return
	}
	r.insertEntries()
}

func (r *BottomSheetRoute) insertEntries() {
	// Create the controller for this sheet
	r.controller = widgets.NewBottomSheetController()

	// Create barrier entry - color resolved inside builder to access theme
	r.barrierEntry = overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		// Get barrier color from route or theme
		themeData := theme.ThemeOf(ctx).BottomSheetThemeOf()
		barrierColor := themeData.BarrierColor
		if r.BarrierColor != nil {
			barrierColor = *r.BarrierColor // Route override wins
		}
		if r.controller != nil {
			alpha := barrierColor.Alpha() * r.controller.Progress()
			barrierColor = barrierColor.WithAlpha(alpha)
		}

		return overlay.ModalBarrier{
			Color:       barrierColor,
			Dismissible: r.BarrierDismissible,
			// Barrier tap triggers animated dismiss via controller
			OnDismiss:     func() { r.controller.Close(nil) },
			SemanticLabel: "Dismiss bottom sheet",
		}
	})
	r.barrierEntry.Opaque = false // Don't block hit testing everywhere

	// Create sheet entry - positioned at bottom, animates height
	r.sheetEntry = overlay.NewOverlayEntry(func(ctx core.BuildContext) core.Widget {
		// Get theme for the sheet
		themeData := theme.ThemeOf(ctx).BottomSheetThemeOf()
		sheetTheme := widgets.BottomSheetTheme{
			BackgroundColor:     themeData.BackgroundColor,
			HandleColor:         themeData.HandleColor,
			BorderRadius:        themeData.BorderRadius,
			HandleWidth:         themeData.HandleWidth,
			HandleHeight:        themeData.HandleHeight,
			HandleTopPadding:    themeData.HandleTopPadding,
			HandleBottomPadding: themeData.HandleBottomPadding,
		}

		return widgets.BottomSheet{
			Builder:      r.builder,
			Controller:   r.controller,
			SnapPoints:   r.SnapPoints,
			InitialSnap:  r.InitialSnapPoint,
			EnableDrag:   r.EnableDrag,
			DragMode:     r.DragMode,
			ShowHandle:   r.ShowHandle,
			UseSafeArea:  r.UseSafeArea,
			Theme:        sheetTheme,
			SnapBehavior: r.SnapBehavior,
			// Called when dismiss animation completes
			OnDismiss: r.onAnimationComplete,
		}
	})
	r.sheetEntry.Opaque = true // Block hit testing below when in sheet area

	if r.controller != nil {
		r.progressRemove = r.controller.AddProgressListener(func(float64) {
			if r.barrierEntry != nil {
				r.barrierEntry.MarkNeedsBuild()
			}
		})
	}

	// Insert barrier first, then sheet (sheet on top)
	r.overlayState.Insert(r.barrierEntry, nil, nil)
	r.overlayState.Insert(r.sheetEntry, nil, r.barrierEntry) // above: barrierEntry
}

// onAnimationComplete is called when the sheet's dismiss animation finishes.
// This removes the overlay entries and notifies the result callback.
func (r *BottomSheetRoute) onAnimationComplete(result any) {
	// Remove overlay entries
	if r.barrierEntry != nil {
		r.barrierEntry.Remove()
		r.barrierEntry = nil
	}
	if r.sheetEntry != nil {
		r.sheetEntry.Remove()
		r.sheetEntry = nil
	}
	if r.progressRemove != nil {
		r.progressRemove()
		r.progressRemove = nil
	}

	// Notify dismiss callback (for ShowModalBottomSheet channel)
	r.mu.Lock()
	if !r.dismissed {
		r.dismissed = true
		onDismiss := r.onDismiss
		r.mu.Unlock()
		if onDismiss != nil {
			onDismiss(result)
		}
	} else {
		r.mu.Unlock()
	}

	// Pop from navigator if not already popped
	// (This handles the case where dismiss was triggered by drag/barrier, not by Pop)
	if !r.poppedFromNav {
		if nav := r.navigator(); nav != nil {
			nav.Pop(result)
		}
	}
}

func (r *BottomSheetRoute) navigator() NavigatorState {
	if r.pushingNav != nil {
		return r.pushingNav
	}
	// Access navigator through global scope - routes don't have direct access
	return globalScope.ActiveNavigator()
}

// DidPop is called when the route is popped from the navigator.
// This triggers the exit animation if not already animating.
func (r *BottomSheetRoute) DidPop(result any) {
	r.didPushPending = false
	r.poppedFromNav = true

	// Trigger exit animation via controller
	// The animation will call onAnimationComplete when done
	if r.controller != nil {
		r.controller.Close(result)
		return
	}

	// No controller (shouldn't happen), remove immediately
	if r.barrierEntry != nil {
		r.barrierEntry.Remove()
		r.barrierEntry = nil
	}
	if r.sheetEntry != nil {
		r.sheetEntry.Remove()
		r.sheetEntry = nil
	}
	if r.progressRemove != nil {
		r.progressRemove()
		r.progressRemove = nil
	}

	// Notify dismiss callback
	r.mu.Lock()
	if !r.dismissed {
		r.dismissed = true
		onDismiss := r.onDismiss
		r.mu.Unlock()
		if onDismiss != nil {
			onDismiss(result)
		}
	} else {
		r.mu.Unlock()
	}
}

// IsTransparent returns true - bottom sheets show content behind the barrier.
func (r *BottomSheetRoute) IsTransparent() bool {
	return true
}

// Build returns the widget for this route.
// When overlay is available and entries inserted, returns a placeholder.
func (r *BottomSheetRoute) Build(ctx core.BuildContext) core.Widget {
	// When overlay is available and entries inserted, render placeholder
	if r.overlayState != nil && r.barrierEntry != nil {
		return widgets.SizedBox{} // Placeholder - content is in overlay
	}
	// Fallback: render directly (shouldn't happen in normal use)
	if r.builder == nil {
		return nil
	}
	return r.builder(ctx)
}

// BottomSheetOption configures a bottom sheet shown via ShowModalBottomSheet.
type BottomSheetOption func(*BottomSheetRoute)

// WithSnapPoints sets the snap points for the bottom sheet.
func WithSnapPoints(points ...widgets.SnapPoint) BottomSheetOption {
	return func(r *BottomSheetRoute) {
		r.SnapPoints = points
	}
}

// WithInitialSnapPoint sets the initial snap point index.
func WithInitialSnapPoint(index int) BottomSheetOption {
	return func(r *BottomSheetRoute) {
		r.InitialSnapPoint = index
	}
}

// WithBarrierDismissible sets whether tapping the barrier dismisses the sheet.
func WithBarrierDismissible(dismissible bool) BottomSheetOption {
	return func(r *BottomSheetRoute) {
		r.BarrierDismissible = dismissible
	}
}

// WithBarrierColor sets the barrier color.
func WithBarrierColor(color graphics.Color) BottomSheetOption {
	return func(r *BottomSheetRoute) {
		r.BarrierColor = &color
	}
}

// WithDragEnabled sets whether the sheet can be dragged.
func WithDragEnabled(enabled bool) BottomSheetOption {
	return func(r *BottomSheetRoute) {
		r.EnableDrag = enabled
	}
}

// WithDragMode sets how the sheet responds to drag gestures.
func WithDragMode(mode widgets.DragMode) BottomSheetOption {
	return func(r *BottomSheetRoute) {
		r.DragMode = mode
	}
}

// WithHandle sets whether a drag handle is shown.
func WithHandle(show bool) BottomSheetOption {
	return func(r *BottomSheetRoute) {
		r.ShowHandle = show
	}
}

// WithSafeArea sets whether the sheet respects the bottom safe area.
func WithSafeArea(use bool) BottomSheetOption {
	return func(r *BottomSheetRoute) {
		r.UseSafeArea = use
	}
}

// WithSnapBehavior customizes snap thresholds.
func WithSnapBehavior(behavior widgets.SnapBehavior) BottomSheetOption {
	return func(r *BottomSheetRoute) {
		r.SnapBehavior = behavior
	}
}

// ShowModalBottomSheet displays a modal bottom sheet.
// Returns a buffered channel (size 1) that receives the result when dismissed.
// The channel is closed after sending the result (or after close without result).
// Callers can safely read once: result := <-ShowModalBottomSheet(...)
//
// To dismiss the sheet from content, use:
//
//	widgets.BottomSheetScope{}.Of(ctx).Close(result)
//
// Example:
//
//	result := <-navigation.ShowModalBottomSheet(ctx, func(ctx core.BuildContext) core.Widget {
//	    return widgets.Column{
//	        Children: []core.Widget{
//	            widgets.Text{Content: "Select an option"},
//	            widgets.Button{Label: "Option 1", OnTap: func() {
//	                widgets.BottomSheetScope{}.Of(ctx).Close("option1")
//	            }},
//	        },
//	    }
//	}, navigation.WithSnapPoints(widgets.SnapHalf, widgets.SnapFull))
func ShowModalBottomSheet(
	ctx core.BuildContext,
	builder func(ctx core.BuildContext) core.Widget,
	options ...BottomSheetOption,
) <-chan any {
	result := make(chan any, 1) // Buffered to prevent blocking
	var once sync.Once

	route := NewBottomSheetRoute(builder, RouteSettings{})

	// Apply options
	for _, opt := range options {
		opt(route)
	}

	// Set up dismiss callback
	route.onDismiss = func(value any) {
		// Idempotent: multiple dismiss paths (drag, barrier tap, back button)
		// all call onDismiss, but we only send/close once
		once.Do(func() {
			result <- value
			close(result)
		})
	}

	// Push the route
	nav := NavigatorOf(ctx)
	if nav != nil {
		route.pushingNav = nav
		nav.Push(route)
	} else {
		once.Do(func() {
			result <- nil
			close(result)
		})
	}

	return result
}
