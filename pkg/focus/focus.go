// Package focus provides focus management structures.
package focus

import "math"

// FocusRect represents a rectangle for focus geometry calculations.
type FocusRect struct {
	Left, Top, Right, Bottom float64
}

// Center returns the center point of the rectangle.
func (r FocusRect) Center() (x, y float64) {
	return (r.Left + r.Right) / 2, (r.Top + r.Bottom) / 2
}

// IsValid returns true if the rect has positive dimensions.
func (r FocusRect) IsValid() bool {
	return r.Right > r.Left && r.Bottom > r.Top
}

// RectProvider is implemented by elements that can provide their geometry.
type RectProvider interface {
	FocusRect() FocusRect
}

// TraversalDirection indicates the focus traversal direction.
type TraversalDirection int

const (
	// TraversalDirectionUp moves focus upward.
	TraversalDirectionUp TraversalDirection = iota

	// TraversalDirectionDown moves focus downward.
	TraversalDirectionDown

	// TraversalDirectionLeft moves focus leftward.
	TraversalDirectionLeft

	// TraversalDirectionRight moves focus rightward.
	TraversalDirectionRight
)

// KeyEvent represents a keyboard event placeholder.
type KeyEvent struct{}

// KeyEventResult indicates how a key event was handled.
type KeyEventResult int

const (
	// KeyEventIgnored indicates the event was not handled.
	KeyEventIgnored KeyEventResult = iota

	// KeyEventHandled indicates the event was consumed.
	KeyEventHandled
)

// FocusNode represents a focusable element in the tree.
type FocusNode struct {
	CanRequestFocus bool
	SkipTraversal   bool
	DebugLabel      string

	OnFocusChange func(hasFocus bool)
	OnKeyEvent    func(event KeyEvent) KeyEventResult

	// Rect provides the geometry for directional focus navigation.
	Rect RectProvider

	// SemanticsLabel provides the accessibility label for this focus node.
	SemanticsLabel string

	// SemanticsHint provides the accessibility hint for this focus node.
	SemanticsHint string

	// SemanticsNodeID links this focus node to a semantics node ID.
	SemanticsNodeID int64

	hasFocus        bool
	hasPrimaryFocus bool
}

// canReceiveFocus reports whether the node can receive focus.
func (n *FocusNode) canReceiveFocus() bool {
	return n != nil && n.CanRequestFocus && !n.SkipTraversal
}

// HasFocus reports whether this node or a descendant has focus.
func (n *FocusNode) HasFocus() bool {
	return n.hasFocus
}

// HasPrimaryFocus reports whether this node is the primary focus.
func (n *FocusNode) HasPrimaryFocus() bool {
	return n.hasPrimaryFocus
}

// RequestFocus requests that this node receive primary focus.
func (n *FocusNode) RequestFocus() {
	if !n.canReceiveFocus() {
		return
	}
	GetFocusManager().setPrimaryFocus(n)
}

// Unfocus removes focus from this node if it has primary focus.
func (n *FocusNode) Unfocus() {
	manager := GetFocusManager()
	if manager.PrimaryFocus == n {
		manager.setPrimaryFocus(nil)
	}
}

// NextFocus moves focus to the next focusable node.
func (n *FocusNode) NextFocus() bool {
	return GetFocusManager().MoveFocus(1)
}

// PreviousFocus moves focus to the previous focusable node.
func (n *FocusNode) PreviousFocus() bool {
	return GetFocusManager().MoveFocus(-1)
}

// FocusScopeNode groups focus nodes.
type FocusScopeNode struct {
	FocusNode
	FocusedChild *FocusNode
	Children     []*FocusNode
}

// SetFirstFocus sets focus to the first focusable child.
func (s *FocusScopeNode) SetFirstFocus() {
	if s == nil || len(s.Children) == 0 {
		return
	}
	for _, child := range s.Children {
		if child.canReceiveFocus() {
			GetFocusManager().setPrimaryFocus(child)
			s.FocusedChild = child
			return
		}
	}
}

// FocusInDirection moves focus in the given direction.
func (s *FocusScopeNode) FocusInDirection(direction TraversalDirection) {
	manager := GetFocusManager()
	current := manager.PrimaryFocus
	if current == nil {
		s.SetFirstFocus()
		return
	}

	// Collect all focusable nodes including from nested scopes
	candidates := s.collectFocusableNodes()

	// Get current node's rect - fallback to linear if invalid
	var currentRect FocusRect
	hasValidCurrentRect := current.Rect != nil
	if hasValidCurrentRect {
		currentRect = current.Rect.FocusRect()
		hasValidCurrentRect = currentRect.IsValid()
	}

	// If current rect is invalid, fallback to linear traversal
	if !hasValidCurrentRect {
		delta := 1
		if direction == TraversalDirectionUp || direction == TraversalDirectionLeft {
			delta = -1
		}
		manager.MoveFocus(delta)
		return
	}

	// Find best candidate in the given direction
	var best *FocusNode
	bestScore := math.MaxFloat64

	for _, child := range candidates {
		if child == current || !child.canReceiveFocus() {
			continue
		}

		// Skip nodes without valid geometry
		if child.Rect == nil {
			continue
		}
		childRect := child.Rect.FocusRect()
		if !childRect.IsValid() {
			continue
		}

		// Check if child is in the requested direction
		if !isInDirection(currentRect, childRect, direction) {
			continue
		}

		// Score by distance and alignment
		score := directionalScore(currentRect, childRect, direction)
		if score < bestScore {
			bestScore = score
			best = child
		}
	}

	if best != nil {
		manager.setPrimaryFocus(best)
		s.FocusedChild = best
	} else {
		// No candidate in direction, fall back to linear traversal
		delta := 1
		if direction == TraversalDirectionUp || direction == TraversalDirectionLeft {
			delta = -1
		}
		manager.MoveFocus(delta)
	}
}

// collectFocusableNodes returns all focusable nodes from this scope and nested scopes.
func (s *FocusScopeNode) collectFocusableNodes() []*FocusNode {
	var nodes []*FocusNode
	s.collectFocusableNodesRecursive(&nodes)
	return nodes
}

// collectFocusableNodesRecursive collects nodes into the provided slice.
func (s *FocusScopeNode) collectFocusableNodesRecursive(nodes *[]*FocusNode) {
	for _, child := range s.Children {
		*nodes = append(*nodes, child)
	}
	// Note: nested FocusScopeNodes would need to be tracked separately
	// and traversed here. Currently FocusScopeNode.Children contains FocusNodes,
	// not nested scopes. When nested scope support is added, traverse them here.
}

// isInDirection checks if target rect is in the specified direction from source.
func isInDirection(source, target FocusRect, direction TraversalDirection) bool {
	sourceCX, sourceCY := source.Center()
	targetCX, targetCY := target.Center()

	switch direction {
	case TraversalDirectionUp:
		return targetCY < sourceCY
	case TraversalDirectionDown:
		return targetCY > sourceCY
	case TraversalDirectionLeft:
		return targetCX < sourceCX
	case TraversalDirectionRight:
		return targetCX > sourceCX
	}
	return false
}

// directionalScore calculates a score for how good a target is for directional focus.
// Lower scores are better. Combines distance with alignment penalty.
func directionalScore(source, target FocusRect, direction TraversalDirection) float64 {
	sourceCX, sourceCY := source.Center()
	targetCX, targetCY := target.Center()

	var primaryDist, crossDist float64

	switch direction {
	case TraversalDirectionUp, TraversalDirectionDown:
		primaryDist = math.Abs(targetCY - sourceCY)
		crossDist = math.Abs(targetCX - sourceCX)
	case TraversalDirectionLeft, TraversalDirectionRight:
		primaryDist = math.Abs(targetCX - sourceCX)
		crossDist = math.Abs(targetCY - sourceCY)
	}

	// Weight cross-axis distance more heavily to prefer aligned elements
	return primaryDist + crossDist*2
}

// FocusManager manages the global focus state.
type FocusManager struct {
	RootScope    *FocusScopeNode
	PrimaryFocus *FocusNode
}

var focusManager = &FocusManager{RootScope: &FocusScopeNode{}}

// GetFocusManager returns the singleton focus manager.
func GetFocusManager() *FocusManager {
	return focusManager
}

// MoveFocus moves focus by delta positions within the root scope.
func (m *FocusManager) MoveFocus(delta int) bool {
	scope := m.RootScope
	if scope == nil || len(scope.Children) == 0 {
		return false
	}

	currentIndex := m.findCurrentFocusIndex(scope)
	count := len(scope.Children)

	for step := 1; step <= count; step++ {
		nextIndex := wrapIndex(currentIndex+delta*step, count)
		candidate := scope.Children[nextIndex]
		if candidate.canReceiveFocus() {
			m.setPrimaryFocus(candidate)
			scope.FocusedChild = candidate
			return true
		}
	}
	return false
}

// findCurrentFocusIndex returns the index of the currently focused node, or -1 if none.
func (m *FocusManager) findCurrentFocusIndex(scope *FocusScopeNode) int {
	for i, child := range scope.Children {
		if child == m.PrimaryFocus {
			return i
		}
	}
	return -1
}

// wrapIndex wraps an index to stay within [0, count).
func wrapIndex(index, count int) int {
	index = index % count
	if index < 0 {
		index += count
	}
	return index
}

// setPrimaryFocus updates the primary focus to the given node.
func (m *FocusManager) setPrimaryFocus(node *FocusNode) {
	if m.PrimaryFocus == node {
		return
	}
	if m.PrimaryFocus != nil {
		m.PrimaryFocus.setFocusState(false)
	}
	m.PrimaryFocus = node
	if node != nil {
		node.setFocusState(true)
	}
}

// setFocusState updates the focus flags and notifies the callback.
func (n *FocusNode) setFocusState(hasFocus bool) {
	n.hasPrimaryFocus = hasFocus
	n.hasFocus = hasFocus
	if n.OnFocusChange != nil {
		n.OnFocusChange(hasFocus)
	}
}
