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
	holders  map[ArenaMember]struct{} // members requesting delayed resolution
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
	a.tryAutoResolveLocked(pointerID, entry)
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
	// Also remove from holders if present
	if entry.holders != nil {
		delete(entry.holders, member)
	}
	if len(entry.members) == 0 {
		delete(a.entries, pointerID)
		return
	}
	a.tryAutoResolveLocked(pointerID, entry)
}

// Sweep clears the arena entry for a pointer.
func (a *GestureArena) Sweep(pointerID int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.entries, pointerID)
}

// Hold defers auto-resolution for this member. Returns true if the hold was
// added successfully. The member must already be in the arena.
func (a *GestureArena) Hold(pointerID int64, member ArenaMember) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry := a.entries[pointerID]
	if entry == nil || entry.resolved != nil {
		return false
	}
	// Verify member is in the arena
	found := false
	for _, m := range entry.members {
		if m == member {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	if entry.holders == nil {
		entry.holders = make(map[ArenaMember]struct{})
	}
	entry.holders[member] = struct{}{}
	return true
}

// ReleaseHold removes a hold without resolving or rejecting.
func (a *GestureArena) ReleaseHold(pointerID int64, member ArenaMember) {
	a.mu.Lock()
	defer a.mu.Unlock()
	entry := a.entries[pointerID]
	if entry == nil || entry.holders == nil {
		return
	}
	delete(entry.holders, member)
	a.tryAutoResolveLocked(pointerID, entry)
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

// tryAutoResolveLocked resolves to the sole remaining member if the arena is
// closed and no holders remain.
func (a *GestureArena) tryAutoResolveLocked(pointerID int64, entry *arenaEntry) {
	if !entry.closed || entry.resolved != nil {
		return
	}
	if len(entry.holders) > 0 {
		return
	}
	if len(entry.members) == 1 {
		a.resolveLocked(pointerID, entry, entry.members[0])
	}
}
