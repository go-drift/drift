package graphics

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
