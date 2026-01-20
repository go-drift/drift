package gestures

import "sync"

// ArenaMember participates in the gesture arena.
type ArenaMember interface {
	AcceptGesture(pointerID int64)
	RejectGesture(pointerID int64)
}

// GestureArena resolves competing gesture recognizers per pointer.
type GestureArena struct {
	mu      sync.Mutex
	entries map[int64]*arenaEntry
}

type arenaEntry struct {
	members  []ArenaMember
	resolved ArenaMember
	closed   bool
}

// NewGestureArena creates a new arena instance.
func NewGestureArena() *GestureArena {
	return &GestureArena{entries: make(map[int64]*arenaEntry)}
}

// DefaultArena is the global gesture arena for the app.
var DefaultArena = NewGestureArena()

// Add registers a member for a pointer.
func (a *GestureArena) Add(pointerID int64, member ArenaMember) {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry := a.entries[pointerID]
	if entry == nil {
		entry = &arenaEntry{}
		a.entries[pointerID] = entry
	}
	if entry.resolved != nil {
		member.RejectGesture(pointerID)
		return
	}
	entry.members = append(entry.members, member)
}

// Close signals that no more members will be added for this pointer.
func (a *GestureArena) Close(pointerID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry := a.entries[pointerID]
	if entry == nil {
		return
	}
	entry.closed = true
	if entry.resolved == nil && len(entry.members) == 1 {
		a.resolveLocked(pointerID, entry, entry.members[0])
	}
}

// Resolve declares the winner for this pointer.
func (a *GestureArena) Resolve(pointerID int64, member ArenaMember) {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry := a.entries[pointerID]
	if entry == nil || entry.resolved != nil {
		return
	}
	a.resolveLocked(pointerID, entry, member)
}

// Reject removes a member from the arena.
func (a *GestureArena) Reject(pointerID int64, member ArenaMember) {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry := a.entries[pointerID]
	if entry == nil || entry.resolved != nil {
		return
	}
	for i, existing := range entry.members {
		if existing == member {
			entry.members = append(entry.members[:i], entry.members[i+1:]...)
			break
		}
	}
	if len(entry.members) == 0 {
		delete(a.entries, pointerID)
		return
	}
	if entry.closed && entry.resolved == nil && len(entry.members) == 1 {
		a.resolveLocked(pointerID, entry, entry.members[0])
	}
}

// Sweep clears the arena entry for a pointer.
func (a *GestureArena) Sweep(pointerID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.entries, pointerID)
}

func (a *GestureArena) resolveLocked(pointerID int64, entry *arenaEntry, winner ArenaMember) {
	entry.resolved = winner
	winner.AcceptGesture(pointerID)
	for _, member := range entry.members {
		if member != winner {
			member.RejectGesture(pointerID)
		}
	}
}
