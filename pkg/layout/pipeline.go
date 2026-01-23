package layout

import "slices"

// PipelineOwner tracks render objects that need layout or paint.
//
// Layout scheduling works with relayout boundaries: when a node needs layout,
// MarkNeedsLayout walks up to the nearest boundary, marking each node along
// the way. The boundary gets scheduled here. During FlushLayoutForRoot, layout
// propagates from the root (or scheduled boundaries) down through all marked
// nodes.
type PipelineOwner struct {
	dirtyLayout    []RenderObject          // boundaries needing layout, processed depth-first
	dirtyLayoutSet map[RenderObject]bool   // O(1) dedup check
	dirtyPaint     map[RenderObject]struct{}
	needsLayout    bool
	needsPaint     bool
}

// ScheduleLayout marks a relayout boundary as needing layout.
// Only relayout boundaries should be scheduled here - intermediate nodes
// are marked via MarkNeedsLayout but not scheduled directly.
func (p *PipelineOwner) ScheduleLayout(object RenderObject) {
	if p.dirtyLayoutSet == nil {
		p.dirtyLayoutSet = make(map[RenderObject]bool)
	}
	if p.dirtyLayoutSet[object] {
		return
	}
	p.dirtyLayoutSet[object] = true
	p.dirtyLayout = append(p.dirtyLayout, object)
	p.needsLayout = true
	p.needsPaint = true
}

// SchedulePaint marks a render object as needing paint.
func (p *PipelineOwner) SchedulePaint(object RenderObject) {
	if p.dirtyPaint == nil {
		p.dirtyPaint = make(map[RenderObject]struct{})
	}
	if _, exists := p.dirtyPaint[object]; exists {
		return
	}
	p.dirtyPaint[object] = struct{}{}
	p.needsPaint = true
}

// NeedsLayout reports if any render objects need layout.
func (p *PipelineOwner) NeedsLayout() bool {
	return p.needsLayout
}

// NeedsPaint reports if any render objects need paint.
func (p *PipelineOwner) NeedsPaint() bool {
	return p.needsPaint
}

// FlushLayoutForRoot runs layout starting from the root.
//
// The typical frame sequence is:
//  1. FlushBuild - rebuilds dirty elements, updates render object properties
//  2. FlushLayoutForRoot - lays out from root, propagating to dirty subtrees
//  3. Paint - renders the tree
//
// Layout starts at the root with tight constraints (root is always a boundary).
// From there, layout propagates down. Nodes with needsLayout=true will run
// PerformLayout; clean nodes with unchanged constraints skip layout entirely.
func (p *PipelineOwner) FlushLayoutForRoot(root RenderObject, constraints Constraints) {
	if !p.needsLayout || root == nil {
		return
	}

	// Layout root with parentUsesSize=false (root is always a boundary).
	// This propagates layout down through all nodes marked via MarkNeedsLayout.
	root.Layout(constraints, false)

	// Process any boundaries that were scheduled during the layout pass.
	// This handles cases where MarkNeedsLayout is called during PerformLayout.
	p.flushDirtyBoundaries()

	// Clear state for next frame
	p.dirtyLayout = nil
	p.dirtyLayoutSet = nil
	p.needsLayout = false
}

// FlushLayoutFromBoundaries processes dirty relayout boundaries without a root.
// This is useful for incremental updates outside the normal frame cycle.
func (p *PipelineOwner) FlushLayoutFromBoundaries() {
	if !p.needsLayout {
		return
	}

	p.flushDirtyBoundaries()

	p.dirtyLayout = nil
	p.dirtyLayoutSet = nil
	p.needsLayout = false
}

// flushDirtyBoundaries processes scheduled boundaries in depth order (parents first).
//
// Boundaries are processed parent-first so that if a parent and child are both
// scheduled, the parent lays out first and may clear the child's dirty flag
// as a side effect (since the child gets laid out as part of the parent's
// PerformLayout). This avoids redundant layout work.
func (p *PipelineOwner) flushDirtyBoundaries() {
	for len(p.dirtyLayout) > 0 {
		// Sort by depth - parents first (lower depth = processed first)
		slices.SortFunc(p.dirtyLayout, func(a, b RenderObject) int {
			return getDepth(a) - getDepth(b)
		})

		// Take current batch and clear for next iteration
		dirty := p.dirtyLayout
		p.dirtyLayout = nil
		p.dirtyLayoutSet = nil

		for _, node := range dirty {
			// Only layout if still dirty - a parent's layout may have already
			// laid out this node as part of its subtree
			if layouter, ok := node.(interface {
				NeedsLayout() bool
				Constraints() Constraints
				Layout(Constraints, bool)
			}); ok {
				if layouter.NeedsLayout() {
					// Re-layout boundary with its cached constraints.
					// parentUsesSize=false because boundaries don't propagate
					// size changes to their parents.
					layouter.Layout(layouter.Constraints(), false)
				}
			}
		}
	}
}

// getDepth returns the tree depth of a render object.
func getDepth(obj RenderObject) int {
	if getter, ok := obj.(interface{ Depth() int }); ok {
		return getter.Depth()
	}
	return 0
}

// FlushPaint clears the dirty paint list.
func (p *PipelineOwner) FlushPaint() {
	p.dirtyPaint = nil
	p.needsPaint = false
}

// FlushLayout clears the dirty layout list without performing layout.
func (p *PipelineOwner) FlushLayout() {
	p.dirtyLayout = nil
	p.needsLayout = false
}
