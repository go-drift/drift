package gestures

import "testing"

// mockMember implements ArenaMember for testing.
type mockMember struct {
	accepted bool
	rejected bool
}

func (m *mockMember) AcceptGesture(pointerID int64) {
	m.accepted = true
}

func (m *mockMember) RejectGesture(pointerID int64) {
	m.rejected = true
}

func TestArena_HoldPreventsAutoResolve(t *testing.T) {
	arena := NewGestureArena()
	m1 := &mockMember{}

	arena.Add(1, m1)
	arena.Hold(1, m1)
	arena.Close(1)

	// With a hold, auto-resolution should not happen even with one member
	if m1.accepted {
		t.Error("Member should not be accepted while holding")
	}

	// Release the hold - now it should auto-resolve
	arena.ReleaseHold(1, m1)
	if !m1.accepted {
		t.Error("Member should be accepted after releasing hold")
	}
}

func TestArena_MultipleHolders(t *testing.T) {
	arena := NewGestureArena()
	m1 := &mockMember{}
	m2 := &mockMember{}

	arena.Add(1, m1)
	arena.Add(1, m2)
	arena.Hold(1, m1)
	arena.Hold(1, m2)
	arena.Close(1)

	// Neither should be resolved yet
	if m1.accepted || m2.accepted {
		t.Error("No member should be accepted while both are holding")
	}

	// First to resolve wins immediately
	arena.Resolve(1, m1)

	if !m1.accepted {
		t.Error("m1 should be accepted after Resolve")
	}
	if !m2.rejected {
		t.Error("m2 should be rejected after m1 wins")
	}
}

func TestArena_HoldThenReject(t *testing.T) {
	arena := NewGestureArena()
	m1 := &mockMember{}
	m2 := &mockMember{}

	arena.Add(1, m1)
	arena.Add(1, m2)
	arena.Hold(1, m1)
	arena.Hold(1, m2)
	arena.Close(1)

	// Reject m1 - this should also release the hold
	arena.Reject(1, m1)

	// m2 should now auto-resolve since it's the only member and its hold was released
	// Wait, actually m2 still has a hold. Let me reconsider.
	// After m1 rejects, m2 is still holding. So m2 should NOT be auto-resolved.
	if m2.accepted {
		t.Error("m2 should not be accepted while still holding")
	}

	// Release m2's hold
	arena.ReleaseHold(1, m2)
	if !m2.accepted {
		t.Error("m2 should be accepted after releasing hold with one member remaining")
	}
}

func TestArena_RejectReleasesHold(t *testing.T) {
	arena := NewGestureArena()
	m1 := &mockMember{}
	m2 := &mockMember{}

	arena.Add(1, m1)
	arena.Add(1, m2)
	arena.Hold(1, m1)
	arena.Close(1)

	// m2 is not holding, m1 is holding
	// Since there are holders, no auto-resolve
	if m1.accepted || m2.accepted {
		t.Error("No member should be accepted while m1 is holding")
	}

	// Reject m1 - this removes m1's hold
	arena.Reject(1, m1)

	// Now m2 is the only member and no holders remain, so m2 should win
	if !m2.accepted {
		t.Error("m2 should be accepted after m1 rejects and releases hold")
	}
}

func TestArena_HoldRequiresMembership(t *testing.T) {
	arena := NewGestureArena()
	m1 := &mockMember{}
	m2 := &mockMember{}

	arena.Add(1, m1)

	// m2 is not in the arena, so Hold should fail
	success := arena.Hold(1, m2)
	if success {
		t.Error("Hold should fail for non-member")
	}

	// m1 is in the arena, so Hold should succeed
	success = arena.Hold(1, m1)
	if !success {
		t.Error("Hold should succeed for member")
	}
}

func TestArena_SweepClearsAllState(t *testing.T) {
	arena := NewGestureArena()
	m1 := &mockMember{}
	m2 := &mockMember{}

	arena.Add(1, m1)
	arena.Add(1, m2)
	arena.Hold(1, m1)
	arena.Close(1)

	// Sweep should clear everything
	arena.Sweep(1)

	// Adding a new member should work cleanly
	m3 := &mockMember{}
	arena.Add(1, m3)
	arena.Close(1)

	// m3 should auto-resolve as the only member
	if !m3.accepted {
		t.Error("m3 should be accepted after sweep and re-add")
	}
}

func TestArena_ResolveImmediateWin(t *testing.T) {
	arena := NewGestureArena()
	m1 := &mockMember{}
	m2 := &mockMember{}

	arena.Add(1, m1)
	arena.Add(1, m2)
	arena.Hold(1, m1)
	arena.Hold(1, m2)

	// Resolve m1 immediately wins, regardless of holds
	arena.Resolve(1, m1)

	if !m1.accepted {
		t.Error("m1 should be accepted after Resolve")
	}
	if !m2.rejected {
		t.Error("m2 should be rejected after m1 resolves")
	}
}

func TestArena_CloseWithoutHold(t *testing.T) {
	arena := NewGestureArena()
	m1 := &mockMember{}

	arena.Add(1, m1)
	arena.Close(1)

	// With one member and no hold, should auto-resolve
	if !m1.accepted {
		t.Error("m1 should be auto-accepted when Close is called with single member and no holds")
	}
}

func TestArena_CloseWithMultipleMembersNoHolds(t *testing.T) {
	arena := NewGestureArena()
	m1 := &mockMember{}
	m2 := &mockMember{}

	arena.Add(1, m1)
	arena.Add(1, m2)
	arena.Close(1)

	// With multiple members and no holds, no auto-resolve happens
	if m1.accepted || m2.accepted {
		t.Error("No member should be auto-accepted with multiple members")
	}
}
