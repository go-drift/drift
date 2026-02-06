package layout

import (
	"reflect"
	"slices"
)

const defaultTypeCountLimit = 5

// TypeCount records the count for a render object type.
type TypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// PipelineOwner tracks render objects that need layout or paint.
//
// Layout scheduling works with relayout boundaries: when a node needs layout,
// MarkNeedsLayout walks up to the nearest boundary, marking each node along
// the way. The boundary gets scheduled here. During FlushLayoutForRoot, layout
// propagates from the root (or scheduled boundaries) down through all marked
// nodes.
type PipelineOwner struct {
	dirtyLayout       []RenderObject        // boundaries needing layout, processed depth-first
	dirtyLayoutSet    map[RenderObject]bool // O(1) dedup check
	dirtyPaint        map[RenderObject]struct{}
	needsLayout       bool
	needsPaint        bool
	dirtySemantics    []RenderObject        // semantics boundaries needing update
	dirtySemanticsSet map[RenderObject]bool // O(1) dedup check for semantics
	needsSemantics    bool
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

// DirtyLayoutCount returns the number of scheduled layout boundaries.
func (p *PipelineOwner) DirtyLayoutCount() int {
	return len(p.dirtyLayout)
}

// DirtyLayoutTypes returns the most common dirty layout boundary types.
func (p *PipelineOwner) DirtyLayoutTypes(limit int) []TypeCount {
	return typeCountsForList(p.dirtyLayout, limit)
}

// NeedsPaint reports if any render objects need paint.
func (p *PipelineOwner) NeedsPaint() bool {
	return p.needsPaint
}

// DirtyPaintCount returns the number of scheduled paint boundaries.
func (p *PipelineOwner) DirtyPaintCount() int {
	return len(p.dirtyPaint)
}

// DirtyPaintTypes returns the most common dirty paint boundary types.
func (p *PipelineOwner) DirtyPaintTypes(limit int) []TypeCount {
	list := make([]RenderObject, 0, len(p.dirtyPaint))
	for obj := range p.dirtyPaint {
		list = append(list, obj)
	}
	return typeCountsForList(list, limit)
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

// FlushPaint processes dirty repaint boundaries in depth order.
// Returns boundaries that need repainting (parents first).
func (p *PipelineOwner) FlushPaint() []RenderObject {
	if !p.needsPaint || len(p.dirtyPaint) == 0 {
		p.dirtyPaint = nil
		p.needsPaint = false
		return nil
	}

	dirty := make([]RenderObject, 0, len(p.dirtyPaint))
	for obj := range p.dirtyPaint {
		dirty = append(dirty, obj)
	}

	// Sort by depth - parents first (same as flushDirtyBoundaries)
	slices.SortFunc(dirty, func(a, b RenderObject) int {
		return getDepth(a) - getDepth(b)
	})

	// Filter to boundaries that still need paint
	result := make([]RenderObject, 0, len(dirty))
	for _, node := range dirty {
		if np, ok := node.(interface{ NeedsPaint() bool }); ok && np.NeedsPaint() {
			result = append(result, node)
		}
	}

	p.dirtyPaint = nil
	p.needsPaint = false
	return result
}

// FlushLayout clears the dirty layout list without performing layout.
func (p *PipelineOwner) FlushLayout() {
	p.dirtyLayout = nil
	p.needsLayout = false
}

// ScheduleSemantics marks a semantics boundary as needing update.
func (p *PipelineOwner) ScheduleSemantics(object RenderObject) {
	if p.dirtySemanticsSet == nil {
		p.dirtySemanticsSet = make(map[RenderObject]bool)
	}
	if p.dirtySemanticsSet[object] {
		return
	}
	p.dirtySemanticsSet[object] = true
	p.dirtySemantics = append(p.dirtySemantics, object)
	p.needsSemantics = true
}

// NeedsSemantics reports if any render objects need semantics update.
func (p *PipelineOwner) NeedsSemantics() bool {
	return p.needsSemantics
}

// DirtySemanticsCount returns the number of scheduled semantics boundaries.
func (p *PipelineOwner) DirtySemanticsCount() int {
	return len(p.dirtySemantics)
}

// DirtySemanticsTypes returns the most common dirty semantics boundary types.
func (p *PipelineOwner) DirtySemanticsTypes(limit int) []TypeCount {
	return typeCountsForList(p.dirtySemantics, limit)
}

func typeCountsForList(items []RenderObject, limit int) []TypeCount {
	if len(items) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = defaultTypeCountLimit
	}

	counts := make(map[string]int)
	for _, obj := range items {
		name := renderTypeName(obj)
		counts[name]++
	}

	result := make([]TypeCount, 0, len(counts))
	for name, count := range counts {
		result = append(result, TypeCount{Type: name, Count: count})
	}

	slices.SortFunc(result, func(a, b TypeCount) int {
		if a.Count == b.Count {
			if a.Type == b.Type {
				return 0
			}
			if a.Type < b.Type {
				return -1
			}
			return 1
		}
		if a.Count > b.Count {
			return -1
		}
		return 1
	})

	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

func renderTypeName(obj RenderObject) string {
	if obj == nil {
		return "<nil>"
	}
	objType := reflect.TypeOf(obj)
	if objType.Kind() == reflect.Pointer {
		objType = objType.Elem()
	}
	if objType.Name() == "" {
		return objType.String()
	}
	return objType.Name()
}

// FlushSemantics returns dirty semantics boundaries sorted by depth.
func (p *PipelineOwner) FlushSemantics() []RenderObject {
	if !p.needsSemantics || len(p.dirtySemantics) == 0 {
		p.dirtySemantics = nil
		p.dirtySemanticsSet = nil
		p.needsSemantics = false
		return nil
	}

	// Sort by depth - parents first
	slices.SortFunc(p.dirtySemantics, func(a, b RenderObject) int {
		return getDepth(a) - getDepth(b)
	})

	// Filter to boundaries that still need update
	result := make([]RenderObject, 0, len(p.dirtySemantics))
	for _, node := range p.dirtySemantics {
		if ns, ok := node.(interface{ NeedsSemanticsUpdate() bool }); ok && ns.NeedsSemanticsUpdate() {
			result = append(result, node)
		}
	}

	p.dirtySemantics = nil
	p.dirtySemanticsSet = nil
	p.needsSemantics = false
	return result
}
