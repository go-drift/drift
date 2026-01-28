package core

import (
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/graphics"
)

// ScrollOffsetProvider reports a paint-time scroll offset for descendants.
type ScrollOffsetProvider interface {
	ScrollOffset() graphics.Offset
}

// GlobalOffsetOf returns the accumulated offset for an element in the render tree.
func GlobalOffsetOf(element Element) graphics.Offset {
	var offset graphics.Offset
	var lastRenderObject layout.RenderObject
	current := element
	for current != nil {
		if renderElement, ok := current.(interface{ RenderObject() layout.RenderObject }); ok {
			renderObject := renderElement.RenderObject()
			if renderObject != nil && renderObject != lastRenderObject {
				if data, ok := renderObject.ParentData().(*layout.BoxParentData); ok && data != nil {
					offset.X += data.Offset.X
					offset.Y += data.Offset.Y
				}
				if provider, ok := renderObject.(ScrollOffsetProvider); ok {
					scrollOffset := provider.ScrollOffset()
					offset.X += scrollOffset.X
					offset.Y += scrollOffset.Y
				}
				lastRenderObject = renderObject
			}
		}

		if parentProvider, ok := current.(interface{ parentElement() Element }); ok {
			current = parentProvider.parentElement()
		} else {
			break
		}
	}

	return offset
}
