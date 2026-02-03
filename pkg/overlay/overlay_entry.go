// Package overlay provides core overlay infrastructure for modals, dialogs,
// bottom sheets, and other floating UI elements.
package overlay

import (
	"sync/atomic"

	"github.com/go-drift/drift/pkg/core"
)

// nextEntryID is an atomic counter for unique entry IDs.
var nextEntryID uint64

// NewOverlayEntry creates an OverlayEntry with a unique ID.
// Always use this constructor rather than literal struct creation
// to ensure proper keying.
func NewOverlayEntry(builder func(ctx core.BuildContext) core.Widget) *OverlayEntry {
	return &OverlayEntry{
		Builder: builder,
		id:      atomic.AddUint64(&nextEntryID, 1),
	}
}

// OverlayEntry represents a single item in the overlay stack.
// It's a mutable handle - modifying fields and calling MarkNeedsBuild
// triggers a rebuild of just this entry.
type OverlayEntry struct {
	// Builder creates the overlay content. Called on each rebuild.
	Builder func(ctx core.BuildContext) core.Widget

	// Opaque indicates this entry blocks hit testing from reaching the
	// child (page content), but other overlay entries can still receive
	// hits. This allows modal barriers below opaque content to still
	// handle dismiss taps. Entries below are still rendered (for partial
	// transparency) and still receive hit tests.
	Opaque bool

	// MaintainState is reserved for future use.
	// Currently all entries are always built regardless of this flag.
	MaintainState bool

	// internal - set by overlayState on Insert, cleared on Remove
	overlay    *overlayState      // concrete type (not interface) for remove()
	mounted    bool               // true when entry widget is in the tree
	entryState *overlayEntryState // for MarkNeedsBuild
	id         uint64             // unique ID for stable keying
}

// Remove removes this entry from its overlay.
// Safe to call if not inserted or already removed (no-op).
// Can be called before first build to cancel a pending entry.
func (e *OverlayEntry) Remove() {
	if e.overlay == nil {
		return // Not inserted or already removed
	}
	// Don't clear entry fields here - let doRemoveEntry handle it.
	// This ensures queued removals still execute properly.
	// The overlay field guards against double-Remove (doRemoveEntry clears it).
	e.overlay.removeEntry(e)
}

// MarkNeedsBuild triggers a rebuild of this entry's widget.
// No-op if entry is not currently mounted.
func (e *OverlayEntry) MarkNeedsBuild() {
	if !e.mounted || e.entryState == nil {
		return
	}
	e.entryState.markNeedsBuild()
}

// overlayEntryWidget is an internal widget that wraps each entry.
type overlayEntryWidget struct {
	entry *OverlayEntry
}

func (w overlayEntryWidget) CreateElement() core.Element {
	return core.NewStatefulElement(w, nil)
}

func (w overlayEntryWidget) Key() any {
	// Stable key prevents state swapping during Rearrange
	return w.entry.id
}

func (w overlayEntryWidget) CreateState() core.State {
	return &overlayEntryState{}
}

// overlayEntryState manages the state of an overlay entry widget.
type overlayEntryState struct {
	core.StateBase
	entry *OverlayEntry
}

func (s *overlayEntryState) InitState() {
	widget := s.Element().Widget().(overlayEntryWidget)
	s.entry = widget.entry
	// Link entry to state for MarkNeedsBuild
	s.entry.entryState = s
	s.entry.mounted = true
}

func (s *overlayEntryState) Build(ctx core.BuildContext) core.Widget {
	if s.entry.Builder == nil {
		return nil
	}
	return s.entry.Builder(ctx)
}

func (s *overlayEntryState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	// Handle entry pointer change (shouldn't happen with stable keys, but defensive)
	old := oldWidget.(overlayEntryWidget)
	if old.entry != s.entry {
		old.entry.entryState = nil
		old.entry.mounted = false
		widget := s.Element().Widget().(overlayEntryWidget)
		s.entry = widget.entry
		s.entry.entryState = s
		s.entry.mounted = true
	}
}

func (s *overlayEntryState) Dispose() {
	// Unlink on dispose
	if s.entry != nil {
		s.entry.entryState = nil
		s.entry.mounted = false
	}
	s.StateBase.Dispose()
}

func (s *overlayEntryState) markNeedsBuild() {
	s.SetState(func() {}) // Triggers rebuild
}
