package overlay

import (
	"reflect"
	"sync/atomic"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/platform"
)

// Overlay manages a stack of overlay entries above its child.
// Use OverlayOf(ctx) to access the nearest overlay's state.
type Overlay struct {
	Child          core.Widget
	InitialEntries []*OverlayEntry

	// OnOverlayReady is called once after Overlay's first build when
	// OverlayState becomes available. The callback is deferred to a
	// post-frame callback to avoid re-entrancy during build.
	// Use this to store OverlayState for later use (e.g., in Navigator).
	OnOverlayReady func(state OverlayState)
}

func (o Overlay) CreateElement() core.Element {
	return core.NewStatefulElement(o, nil)
}

func (o Overlay) Key() any {
	return nil
}

func (o Overlay) CreateState() core.State {
	return &overlayState{}
}

// OverlayState provides methods to manipulate overlay entries.
// This is a sealed interface - only overlayState implements it.
type OverlayState interface {
	// Insert adds entry to the overlay.
	// Positioning: exactly one of below/above may be non-nil.
	//   - below non-nil: inserts just below that entry
	//   - above non-nil: inserts just above that entry
	//   - both nil: inserts at top
	// Panics if both below AND above are non-nil (ambiguous).
	// Panics if entry is already inserted to any overlay.
	// If called during build, insertion is queued until after build completes.
	Insert(entry *OverlayEntry, below *OverlayEntry, above *OverlayEntry)

	// InsertAll adds multiple entries. Same positioning logic as Insert.
	InsertAll(entries []*OverlayEntry, below *OverlayEntry, above *OverlayEntry)

	// Rearrange reorders entries. Entries not in newEntries are removed.
	Rearrange(newEntries []*OverlayEntry)

	// sealed prevents external implementations
	sealed()
}

// overlayState is the concrete implementation of OverlayState
type overlayState struct {
	core.StateBase
	overlay       Overlay
	entries       []*OverlayEntry
	onReadyCalled bool
	isBuilding    bool
	pendingOps    []func() // queued insertions during build
}

func (s *overlayState) sealed() {} // prevents external implementation

func (s *overlayState) InitState() {
	s.overlay = s.Element().Widget().(Overlay)
	// Initialize entries from InitialEntries
	for _, entry := range s.overlay.InitialEntries {
		entry.overlay = s
		// Assign ID if missing (fallback for literal construction)
		if entry.id == 0 {
			entry.id = atomic.AddUint64(&nextEntryID, 1)
		}
	}
	s.entries = append(s.entries, s.overlay.InitialEntries...)
}

func (s *overlayState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	s.overlay = s.Element().Widget().(Overlay)
}

func (s *overlayState) Build(ctx core.BuildContext) core.Widget {
	s.isBuilding = true

	// Schedule OnOverlayReady to fire in the next frame via platform.Dispatch.
	// This ensures the callback runs after the current build completes,
	// avoiding re-entrancy issues when the callback triggers SetState.
	if !s.onReadyCalled && s.overlay.OnOverlayReady != nil {
		s.onReadyCalled = true
		callback := s.overlay.OnOverlayReady
		platform.Dispatch(func() {
			callback(s)
		})
	}

	// Build all entries - they're all rendered and can receive hit tests.
	// The Opaque flag only affects whether hits pass through to the child
	// (page content), not whether entries below are built or receive hits.
	// This is essential for modal barriers to work correctly.
	entryWidgets := make([]core.Widget, 0, len(s.entries))
	opaqueIndex := -1

	for i, entry := range s.entries {
		entryWidgets = append(entryWidgets, overlayEntryWidget{entry: entry})

		// Track first opaque entry for hit testing
		if entry.Opaque && opaqueIndex < 0 {
			opaqueIndex = i
		}
	}

	s.isBuilding = false

	// Process queued operations after build
	if len(s.pendingOps) > 0 {
		ops := s.pendingOps
		s.pendingOps = nil
		for _, op := range ops {
			op()
		}
		if len(s.pendingOps) == 0 { // Only rebuild if ops were processed and no new ones added
			s.Element().MarkNeedsBuild()
		}
	}

	// Build custom overlay render that handles Opaque hit testing
	return overlayInherited{
		state: s,
		child: overlayRender{
			child:   s.overlay.Child,
			entries: entryWidgets,
			opaque:  opaqueIndex,
		},
	}
}

// Insert adds entry to the overlay.
func (s *overlayState) Insert(entry *OverlayEntry, below, above *OverlayEntry) {
	// Validation upfront (before queuing)
	if below != nil && above != nil {
		panic("overlay: both below and above specified")
	}
	if entry.overlay != nil {
		panic("overlay: entry already inserted")
	}

	// Mark entry as belonging to this overlay immediately
	// (allows Remove during build to work correctly)
	entry.overlay = s

	// Assign ID if missing
	if entry.id == 0 {
		entry.id = atomic.AddUint64(&nextEntryID, 1)
	}

	if s.isBuilding {
		// Queue actual insertion for after build
		s.pendingOps = append(s.pendingOps, func() {
			// Check entry wasn't removed while queued
			if entry.overlay != s {
				return // Entry was removed before insert completed
			}
			s.insertIntoEntries(entry, below, above)
			s.Element().MarkNeedsBuild()
		})
		return
	}
	s.insertIntoEntries(entry, below, above)
	s.Element().MarkNeedsBuild()
}

// InsertAll adds multiple entries.
func (s *overlayState) InsertAll(entries []*OverlayEntry, below, above *OverlayEntry) {
	for _, entry := range entries {
		s.Insert(entry, below, above)
		// Each subsequent entry goes above the previous one when below/above is nil
		if below == nil && above == nil && len(entries) > 1 {
			above = entry
		}
	}
}

// Rearrange reorders entries. Entries not in newEntries are removed.
func (s *overlayState) Rearrange(newEntries []*OverlayEntry) {
	// Build set of new entries for quick lookup
	newSet := make(map[*OverlayEntry]bool, len(newEntries))
	for _, entry := range newEntries {
		newSet[entry] = true
	}

	// Remove entries not in newEntries
	for _, entry := range s.entries {
		if !newSet[entry] {
			entry.overlay = nil
			entry.mounted = false
			entry.entryState = nil
		}
	}

	// Set overlay on new entries
	for _, entry := range newEntries {
		entry.overlay = s
		if entry.id == 0 {
			entry.id = atomic.AddUint64(&nextEntryID, 1)
		}
	}

	s.entries = newEntries
	s.Element().MarkNeedsBuild()
}

func (s *overlayState) insertIntoEntries(entry *OverlayEntry, below, above *OverlayEntry) {
	if below != nil {
		// Insert just below the specified entry
		for i, e := range s.entries {
			if e == below {
				// Insert at position i (below moves to i+1)
				s.entries = append(s.entries[:i], append([]*OverlayEntry{entry}, s.entries[i:]...)...)
				return
			}
		}
		// below not found, insert at bottom
		s.entries = append([]*OverlayEntry{entry}, s.entries...)
	} else if above != nil {
		// Insert just above the specified entry
		for i, e := range s.entries {
			if e == above {
				// Insert at position i+1
				s.entries = append(s.entries[:i+1], append([]*OverlayEntry{entry}, s.entries[i+1:]...)...)
				return
			}
		}
		// above not found, insert at top
		s.entries = append(s.entries, entry)
	} else {
		// Insert at top
		s.entries = append(s.entries, entry)
	}
}

func (s *overlayState) removeEntry(entry *OverlayEntry) {
	if s.isBuilding {
		// Queue for after build
		s.pendingOps = append(s.pendingOps, func() {
			s.doRemoveEntry(entry)
		})
		return
	}
	s.doRemoveEntry(entry)
}

func (s *overlayState) doRemoveEntry(entry *OverlayEntry) {
	// Skip if entry was already removed or re-inserted elsewhere
	if entry.overlay != s {
		return
	}

	// Clear entry references FIRST (before removing from entries)
	// This ensures queued insertIntoEntries will skip if it runs after us
	entry.overlay = nil
	entry.mounted = false
	entry.entryState = nil

	// Remove from entries slice (may not be present if Insert was queued)
	for i, e := range s.entries {
		if e == entry {
			s.entries = append(s.entries[:i], s.entries[i+1:]...)
			s.Element().MarkNeedsBuild()
			break
		}
	}
}

// overlayInherited provides OverlayState to descendants.
type overlayInherited struct {
	state *overlayState
	child core.Widget
}

func (o overlayInherited) CreateElement() core.Element {
	return core.NewInheritedElement(o, nil)
}

func (o overlayInherited) Key() any {
	return nil
}

func (o overlayInherited) ChildWidget() core.Widget {
	return o.child
}

func (o overlayInherited) UpdateShouldNotify(oldWidget core.InheritedWidget) bool {
	if old, ok := oldWidget.(overlayInherited); ok {
		return o.state != old.state
	}
	return true
}

func (o overlayInherited) UpdateShouldNotifyDependent(oldWidget core.InheritedWidget, aspects map[any]struct{}) bool {
	return o.UpdateShouldNotify(oldWidget)
}

var overlayInheritedType = reflect.TypeFor[overlayInherited]()

// OverlayOf returns the nearest Overlay ancestor's state.
// Returns nil if no Overlay ancestor exists.
func OverlayOf(ctx core.BuildContext) OverlayState {
	inherited := ctx.DependOnInherited(overlayInheritedType, nil)
	if inherited == nil {
		return nil
	}
	if overlay, ok := inherited.(overlayInherited); ok {
		return overlay.state
	}
	return nil
}
