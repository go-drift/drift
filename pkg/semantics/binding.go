//go:build android || darwin || ios
// +build android darwin ios

package semantics

import "sync"

// SemanticsBinding connects the semantics system to platform accessibility services.
//
// Why this exists as a separate layer from SemanticsOwner:
//
// The binding serves as a stable "rendezvous point" that handles initialization
// order between platform channels and the accessibility service:
//
//  1. Platform init: During package initialization (init()), platform event
//     handlers are registered that need to route accessibility events (like
//     TalkBack actions) somewhere. The binding exists at this point as a global.
//
//  2. Service init: The accessibility.Service is created later during the first
//     frame render. It creates a SemanticsOwner and connects it to this binding.
//
// The binding also provides:
//   - Enable/disable toggle that can be set by the platform before the owner exists
//   - Action routing from platform to the semantics tree
//   - Send function registration for platform updates
//
// Alternative approach (not implemented):
// This layer could be eliminated by making platform init lazy:
//   - Move event handler setup from init() to a function called by Service.Initialize()
//   - Inline the binding's state (enabled, sendFn, actionFn) into SemanticsOwner
//   - Have platform/accessibility.go call a registration function instead of using init()
//
// This would remove the global singleton but requires coordinating init order
// across packages. The current approach is simpler and the global is isolated
// to this package.
type SemanticsBinding struct {
	owner    *SemanticsOwner
	enabled  bool
	sendFn   func(SemanticsUpdate) error
	actionFn func(nodeID int64, action SemanticsAction, args any) bool
	mu       sync.RWMutex
}

// binding is the global semantics binding instance.
var binding = &SemanticsBinding{}

// GetSemanticsBinding returns the global semantics binding.
func GetSemanticsBinding() *SemanticsBinding {
	return binding
}

// SetOwner sets the semantics owner for this binding.
func (b *SemanticsBinding) SetOwner(owner *SemanticsOwner) {
	b.mu.Lock()
	enabled := b.enabled
	b.owner = owner
	if owner != nil {
		owner.SetUpdateCallback(func(update SemanticsUpdate) {
			b.sendUpdate(update)
		})
	}
	b.mu.Unlock()

	// If accessibility is already enabled, trigger an update
	if enabled && owner != nil {
		owner.SendFullUpdate()
	}
}

// Owner returns the current semantics owner.
func (b *SemanticsBinding) Owner() *SemanticsOwner {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.owner
}

// SetEnabled enables or disables accessibility.
// When disabled, semantics updates are not sent to the platform.
func (b *SemanticsBinding) SetEnabled(enabled bool) {
	b.mu.Lock()
	wasEnabled := b.enabled
	b.enabled = enabled
	owner := b.owner
	b.mu.Unlock()

	// If enabling and we have an owner, send full update
	if enabled && !wasEnabled && owner != nil {
		owner.SendFullUpdate()
	}
}

// IsEnabled reports whether accessibility is enabled.
func (b *SemanticsBinding) IsEnabled() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.enabled
}

// SetSendFunction sets the function used to send updates to the platform.
func (b *SemanticsBinding) SetSendFunction(fn func(SemanticsUpdate) error) {
	b.mu.Lock()
	b.sendFn = fn
	b.mu.Unlock()
}

// SetActionCallback sets the callback for handling actions from the platform.
// This is called when the platform requests an action on a node.
func (b *SemanticsBinding) SetActionCallback(fn func(nodeID int64, action SemanticsAction, args any) bool) {
	b.mu.Lock()
	b.actionFn = fn
	b.mu.Unlock()
}

// sendUpdate sends a semantics update to the platform if enabled.
func (b *SemanticsBinding) sendUpdate(update SemanticsUpdate) {
	b.mu.RLock()
	enabled := b.enabled
	sendFn := b.sendFn
	b.mu.RUnlock()

	if !enabled || sendFn == nil || update.IsEmpty() {
		return
	}
	_ = sendFn(update)
}

// HandleAction handles an action request from the platform.
func (b *SemanticsBinding) HandleAction(nodeID int64, action SemanticsAction, args any) bool {
	b.mu.RLock()
	actionFn := b.actionFn
	owner := b.owner
	b.mu.RUnlock()

	// First try the custom action callback
	if actionFn != nil && actionFn(nodeID, action, args) {
		return true
	}

	// Fall back to the owner's action handling
	if owner != nil {
		return owner.PerformAction(nodeID, action, args)
	}

	return false
}

// RequestFullUpdate requests a full semantics tree update.
func (b *SemanticsBinding) RequestFullUpdate() {
	b.mu.RLock()
	owner := b.owner
	enabled := b.enabled
	b.mu.RUnlock()

	if !enabled || owner == nil {
		return
	}
	owner.SendFullUpdate()
}

// FlushSemantics sends any pending semantics updates.
func (b *SemanticsBinding) FlushSemantics() {
	b.mu.RLock()
	owner := b.owner
	enabled := b.enabled
	b.mu.RUnlock()

	if !enabled || owner == nil {
		return
	}
	owner.SendSemanticsUpdate()
}

// MarkNodeDirty marks a specific node as needing update.
func (b *SemanticsBinding) MarkNodeDirty(node *SemanticsNode) {
	b.mu.RLock()
	owner := b.owner
	b.mu.RUnlock()

	if owner != nil {
		owner.MarkDirty(node)
	}
}

// FindNodeByID finds a semantics node by its ID.
func (b *SemanticsBinding) FindNodeByID(id int64) *SemanticsNode {
	b.mu.RLock()
	owner := b.owner
	b.mu.RUnlock()

	if owner == nil {
		return nil
	}
	return owner.FindNodeByID(id)
}
