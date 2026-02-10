package core

import (
	"slices"
	"sync"

	"github.com/go-drift/drift/pkg/layout"
)

// BuildOwner tracks dirty elements that need rebuilding.
type BuildOwner struct {
	dirty    []Element
	pipeline *layout.PipelineOwner
	mu       sync.Mutex

	// OnNeedsFrame is called when a new element is scheduled for rebuild,
	// signalling the platform that a frame should be rendered. This is
	// necessary for on-demand frame scheduling where the display link is
	// paused until explicitly requested.
	OnNeedsFrame func()
}

// NewBuildOwner creates a new BuildOwner.
func NewBuildOwner() *BuildOwner {
	return &BuildOwner{
		pipeline: &layout.PipelineOwner{},
	}
}

// Pipeline returns the PipelineOwner for render object scheduling.
func (b *BuildOwner) Pipeline() *layout.PipelineOwner {
	return b.pipeline
}

// ScheduleBuild marks an element as needing rebuild.
func (b *BuildOwner) ScheduleBuild(element Element) {
	added := func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		if slices.Contains(b.dirty, element) {
			return false
		}
		b.dirty = append(b.dirty, element)
		return true
	}()

	if added && b.OnNeedsFrame != nil {
		b.OnNeedsFrame()
	}
}

// NeedsWork returns true if there are dirty elements or pending layout/paint.
func (b *BuildOwner) NeedsWork() bool {
	b.mu.Lock()
	hasDirty := len(b.dirty) > 0
	b.mu.Unlock()
	if hasDirty {
		return true
	}
	return b.pipeline.NeedsLayout() || b.pipeline.NeedsPaint()
}

// FlushBuild rebuilds all dirty elements in depth order.
func (b *BuildOwner) FlushBuild() {
	for {
		b.mu.Lock()
		if len(b.dirty) == 0 {
			b.mu.Unlock()
			return
		}

		slices.SortFunc(b.dirty, func(a, b Element) int {
			return a.Depth() - b.Depth()
		})

		dirty := b.dirty
		b.dirty = nil
		b.mu.Unlock()

		for _, element := range dirty {
			if mountable, ok := element.(interface{ isMounted() bool }); ok && !mountable.isMounted() {
				continue
			}
			element.RebuildIfNeeded()
		}
	}
}
