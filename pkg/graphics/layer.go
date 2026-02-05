package graphics

import "fmt"

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
	return fmt.Sprintf("Layer{dirty=%v, size=%v, hasContent=%v}", l.Dirty, l.Size, hasContent)
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
