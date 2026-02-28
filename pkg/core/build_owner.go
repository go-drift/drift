package core

import (
	"slices"
	"sync"

	"github.com/go-drift/drift/pkg/layout"
)

// BuildOwner tracks dirty elements that need rebuilding.
type BuildOwner struct {
	dirty      []Element
	dirtySet   map[Element]bool
	pipeline   *layout.PipelineOwner
	globalKeys map[any]Element
	mu         sync.Mutex

	// OnNeedsFrame is called when a new element is scheduled for rebuild,
	// signalling the platform that a frame should be rendered. This is
	// necessary for on-demand frame scheduling where the display link is
	// paused until explicitly requested.
	OnNeedsFrame func()
}

// RegisterGlobalKey associates a global key identity with an element in the
// owner's registry. This is called automatically by the framework when an
// element whose widget returns a [GlobalKey] is mounted. If a key is already
// registered, the new element silently replaces the previous entry.
//
// The key parameter is the inner identity pointer obtained from
// globalKeyRegistry.globalKeyImpl(), not the GlobalKey value itself.
func (b *BuildOwner) RegisterGlobalKey(key any, element Element) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.globalKeys == nil {
		b.globalKeys = make(map[any]Element)
	}
	b.globalKeys[key] = element
}

// UnregisterGlobalKey removes a global key registration, but only if the
// currently registered element matches the provided element. This guard
// prevents a stale unmount from removing a registration that has already been
// claimed by a newly mounted element with the same key.
func (b *BuildOwner) UnregisterGlobalKey(key any, element Element) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.globalKeys[key] == element {
		delete(b.globalKeys, key)
	}
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
		if b.dirtySet[element] {
			return false
		}
		if b.dirtySet == nil {
			b.dirtySet = make(map[Element]bool)
		}
		b.dirtySet[element] = true
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
		clear(b.dirtySet)
		b.mu.Unlock()

		for _, element := range dirty {
			if mountable, ok := element.(interface{ isMounted() bool }); ok && !mountable.isMounted() {
				continue
			}
			element.RebuildIfNeeded()
		}
	}
}
