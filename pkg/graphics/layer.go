package graphics

import "fmt"

// Layer Tree Invariants
//
// The layer tree system provides efficient incremental repainting by caching
// rendered content at repaint boundaries. The following invariants must be maintained:
//
// 1. ROOT BOUNDARY: The root render object must be a repaint boundary.
//    compositeLayerTree starts from root and expects it to have a layer.
//    All paint scheduling walks up to a repaint boundary - root must be one.
//
// 2. STABLE IDENTITY: Layers have stable pointer identity. Never replace a layer
//    object, only mark it dirty. Parent layers hold DrawChildLayer ops that reference
//    child layers by pointer - replacing the object would break these references.
//
// 3. CHILDREN BEFORE PARENTS: During recording, child layers must be recorded before
//    their parent layers. This ensures DrawChildLayer ops reference valid content.
//    recordDirtyLayers processes boundaries in reverse depth order to achieve this.
//
// 4. DIRTY FLAG SYNC: layer.Dirty must stay in sync with RenderBoxBase.needsPaint.
//    MarkNeedsPaint ensures the layer exists for boundaries before marking dirty.
//
// 5. DISPOSAL: When a render object is removed from the tree, its layer must be
//    disposed to release GPU resources. RenderBoxBase.Dispose handles this.

// Layer represents a cached drawing surface for a repaint boundary.
// Layers have stable identity - never replace the object, only mark dirty.
// Parent layers contain references to child layers via DrawChildLayer ops,
// not embedded content. This allows child content to change without
// requiring parent re-recording.
type Layer struct {
	// Content is the recorded display list (includes DrawChildLayer ops)
	Content *DisplayList

	// Dirty indicates this layer needs re-recording
	Dirty bool

	// Size of this layer's bounds
	Size Size
}

// String returns a debug representation of the layer.
func (l *Layer) String() string {
	if l == nil {
		return "Layer(nil)"
	}
	hasContent := l.Content != nil
	return fmt.Sprintf("Layer{dirty=%v, size=%.0fx%.0f, hasContent=%v}", l.Dirty, l.Size.Width, l.Size.Height, hasContent)
}

// Composite draws this layer to the canvas.
// Child layers are drawn via DrawChildLayer ops within Content.
func (l *Layer) Composite(canvas Canvas) {
	if l.Content != nil {
		l.Content.Paint(canvas)
	}
}

// MarkDirty marks this layer for re-recording.
func (l *Layer) MarkDirty() {
	l.Dirty = true
}

// SetContent replaces the layer's content, disposing the old content first.
// This should be called when re-recording a layer.
func (l *Layer) SetContent(content *DisplayList) {
	if l.Content != nil {
		l.Content.Dispose()
	}
	l.Content = content
	l.Dirty = false
}

// Dispose releases resources held by this layer.
// Call this when the layer is no longer needed (e.g., boundary removed from tree).
func (l *Layer) Dispose() {
	if l.Content != nil {
		l.Content.Dispose()
		l.Content = nil
	}
}
