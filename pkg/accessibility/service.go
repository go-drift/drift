//go:build android || darwin || ios

package accessibility

import (
	"sync"

	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/rendering"
	"github.com/go-drift/drift/pkg/semantics"
)

// Service manages the accessibility system, coordinating between the semantics
// tree, platform channels, and action handling.
type Service struct {
	owner       *semantics.SemanticsOwner
	binding     *semantics.SemanticsBinding
	lastRoot    *semantics.SemanticsNode
	initialized bool
	deviceScale float64
	mu          sync.RWMutex
}

// NewService creates a new accessibility service.
func NewService() *Service {
	return &Service{
		deviceScale: 1.0,
	}
}

// Initialize sets up the accessibility system.
// This should be called once when the engine starts.
func (s *Service) Initialize() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return
	}
	s.initialized = true

	// Create semantics owner
	s.owner = semantics.NewSemanticsOwner()

	// Initialize platform accessibility channel
	platform.InitializeAccessibility()

	// Connect binding to owner
	s.binding = semantics.GetSemanticsBinding()
	s.binding.SetOwner(s.owner)

	// Query actual accessibility state from platform instead of forcing enabled.
	// This avoids triggering semantics traffic when accessibility is disabled.
	// The initial state event from the platform will update this if needed.
	enabled, err := platform.Accessibility.IsAccessibilityEnabled()
	if err == nil {
		s.binding.SetEnabled(enabled)
	}
	// If query fails, start disabled and rely on platform state event
}

// SetDeviceScale sets the device pixel scale for coordinate conversion.
func (s *Service) SetDeviceScale(scale float64) {
	s.mu.Lock()
	s.deviceScale = scale
	s.mu.Unlock()
}

// IsEnabled returns whether accessibility is enabled.
func (s *Service) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initialized && s.binding != nil && s.binding.IsEnabled()
}

// SetEnabled enables or disables accessibility.
func (s *Service) SetEnabled(enabled bool) {
	s.mu.Lock()
	if s.binding != nil {
		s.binding.SetEnabled(enabled)
	}
	s.mu.Unlock()
}

// FlushSemantics rebuilds and sends the semantics tree.
// If dirtyBoundaries is nil or empty on first frame, does a full rebuild.
// Otherwise, only rebuilds the dirty portions.
func (s *Service) FlushSemantics(rootRender layout.RenderObject, dirtyBoundaries []layout.RenderObject) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized || s.owner == nil || rootRender == nil {
		return
	}

	if s.binding == nil || !s.binding.IsEnabled() {
		return
	}

	// First frame: full rebuild
	if s.lastRoot == nil {
		s.fullRebuild(rootRender)
		return
	}

	// No dirty boundaries: skip entirely (nothing changed)
	if len(dirtyBoundaries) == 0 {
		return
	}

	// Incremental update: rebuild and clear dirty flags
	s.incrementalUpdate(rootRender, dirtyBoundaries)
}

// fullRebuild rebuilds the entire semantics tree from scratch.
func (s *Service) fullRebuild(rootRender layout.RenderObject) {
	// Create synthetic root node (ID 0)
	size := rootRender.Size()
	syntheticRoot := semantics.NewSemanticsNodeWithID(0)
	syntheticRoot.Rect = rendering.RectFromLTWH(
		0, 0,
		size.Width*s.deviceScale,
		size.Height*s.deviceScale,
	)

	// Build semantics tree from render tree
	s.buildFromRender(rootRender, syntheticRoot, rendering.Offset{}, s.deviceScale)

	// Compute diff from last tree
	diff := semantics.ComputeDiff(s.lastRoot, syntheticRoot)

	// Update the owner's root
	s.owner.SetRoot(syntheticRoot)

	// Send updates to platform
	if !diff.IsEmpty() {
		platform.Accessibility.SendSemanticsUpdate(diff)
	}

	s.lastRoot = syntheticRoot
}

// incrementalUpdate rebuilds only the dirty portions of the semantics tree.
func (s *Service) incrementalUpdate(rootRender layout.RenderObject, dirtyBoundaries []layout.RenderObject) {
	// For now, fall back to full rebuild when there are dirty boundaries.
	// This still provides the optimization of skipping entirely when nothing changed.
	// Full incremental updates can be optimized further in the future by only
	// rebuilding subtrees rooted at each dirty boundary.
	s.fullRebuild(rootRender)

	// Clear dirty flags on processed boundaries
	for _, boundary := range dirtyBoundaries {
		if clearer, ok := boundary.(interface{ ClearNeedsSemanticsUpdate() }); ok {
			clearer.ClearNeedsSemanticsUpdate()
		}
	}
}

// buildFromRender recursively builds semantics nodes from render objects.
func (s *Service) buildFromRender(renderObj layout.RenderObject, parent *semantics.SemanticsNode, globalOffset rendering.Offset, deviceScale float64) {
	if renderObj == nil {
		return
	}

	// Get this object's offset from its parent data
	localOffset := rendering.Offset{}
	if parentData := renderObj.ParentData(); parentData != nil {
		if boxData, ok := parentData.(*layout.BoxParentData); ok {
			localOffset = boxData.Offset
		}
	}

	// Compute absolute position
	absolutePos := rendering.Offset{
		X: globalOffset.X + localOffset.X,
		Y: globalOffset.Y + localOffset.Y,
	}

	var currentNode *semantics.SemanticsNode

	// Check if this render object contributes semantics
	if describer, ok := renderObj.(layout.SemanticsDescriber); ok {
		var config semantics.SemanticsConfiguration
		contributes := describer.DescribeSemanticsConfiguration(&config)
		config.EnsureFocusable()

		// Skip hidden nodes and their descendants
		if config.Properties.Flags.Has(semantics.SemanticsIsHidden) {
			return
		}

		if contributes || config.IsSemanticBoundary || !config.IsEmpty() {
			size := renderObj.Size()
			nodeID := s.owner.GetStableID(renderObj)
			currentNode = semantics.NewSemanticsNodeWithID(nodeID)
			currentNode.Config = config
			currentNode.Rect = rendering.RectFromLTWH(
				absolutePos.X*deviceScale,
				absolutePos.Y*deviceScale,
				size.Width*deviceScale,
				size.Height*deviceScale,
			)

			// Merge descendant labels if configured
			if config.IsMergingSemanticsOfDescendants {
				s.mergeDescendantLabels(renderObj, currentNode)
			}

			parent.AddChild(currentNode)

			// Don't recurse if merging descendants
			if config.IsMergingSemanticsOfDescendants {
				return
			}
		}
	}

	// Use current node or parent for children
	nodeForChildren := currentNode
	if nodeForChildren == nil {
		nodeForChildren = parent
	}

	// Compute offset for children, accounting for scroll offset if this is a scrollable
	childOffset := absolutePos
	if scroller, ok := renderObj.(layout.ScrollOffsetProvider); ok {
		scrollOffset := scroller.SemanticScrollOffset()
		childOffset.X -= scrollOffset.X
		childOffset.Y -= scrollOffset.Y
	}

	// Visit children
	if visitor, ok := renderObj.(layout.ChildVisitor); ok {
		visitor.VisitChildren(func(child layout.RenderObject) {
			s.buildFromRender(child, nodeForChildren, childOffset, deviceScale)
		})
	}
}

// mergeDescendantLabels collects labels from descendants and merges them into the node.
func (s *Service) mergeDescendantLabels(renderObj layout.RenderObject, node *semantics.SemanticsNode) {
	labels := s.collectLabels(renderObj)
	if len(labels) == 0 {
		return
	}

	merged := ""
	for i, label := range labels {
		if i > 0 {
			merged += " "
		}
		merged += label
	}

	if node.Config.Properties.Label == "" {
		node.Config.Properties.Label = merged
	} else {
		node.Config.Properties.Label += " " + merged
	}
}

// collectLabels recursively collects all labels from descendant render objects.
func (s *Service) collectLabels(renderObj layout.RenderObject) []string {
	var labels []string

	visitor, ok := renderObj.(layout.ChildVisitor)
	if !ok {
		return labels
	}

	visitor.VisitChildren(func(child layout.RenderObject) {
		if describer, ok := child.(layout.SemanticsDescriber); ok {
			var config semantics.SemanticsConfiguration
			describer.DescribeSemanticsConfiguration(&config)
			if config.Properties.Label != "" {
				labels = append(labels, config.Properties.Label)
			}
		}
		labels = append(labels, s.collectLabels(child)...)
	})

	return labels
}

// Owner returns the semantics owner.
func (s *Service) Owner() *semantics.SemanticsOwner {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.owner
}

// Binding returns the semantics binding.
func (s *Service) Binding() *semantics.SemanticsBinding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.binding
}
