package engine

import (
	"image"
	"unsafe"

	"github.com/go-drift/drift/pkg/graphics"
)

// GeometryCanvas is a no-op canvas that tracks only translation and clip state.
// When EmbedPlatformView is called, it resolves the global offset and clip and
// reports the geometry to the sink. All draw operations are empty.
//
// Uses the same transform/clip logic as CompositingCanvas (via transformTracker)
// but without forwarding to any inner canvas.
type GeometryCanvas struct {
	tracker transformTracker
	sink    PlatformViewSink
	size    graphics.Size
}

// NewGeometryCanvas creates a geometry-only canvas that reports platform view
// positions to the given sink.
func NewGeometryCanvas(size graphics.Size, sink PlatformViewSink) *GeometryCanvas {
	return &GeometryCanvas{
		size: size,
		sink: sink,
	}
}

func (c *GeometryCanvas) Save()                                        { c.tracker.save() }
func (c *GeometryCanvas) SaveLayerAlpha(_ graphics.Rect, _ float64)    { c.tracker.save() }
func (c *GeometryCanvas) SaveLayer(_ graphics.Rect, _ *graphics.Paint) { c.tracker.save() }
func (c *GeometryCanvas) Restore()                                     { c.tracker.restore() }
func (c *GeometryCanvas) Translate(dx, dy float64)                     { c.tracker.translate(dx, dy) }
func (c *GeometryCanvas) ClipRect(rect graphics.Rect)                  { c.tracker.clipRect(rect) }
func (c *GeometryCanvas) ClipRRect(rrect graphics.RRect)               { c.tracker.clipRRect(rrect) }
func (c *GeometryCanvas) SaveLayerBlur(_ graphics.Rect, _, _ float64)  { c.tracker.save() }

// Scale is a no-op. Platform view geometry is reported in logical coordinates;
// the consumer (e.g. Android UI thread) applies device density scaling.
func (c *GeometryCanvas) Scale(_, _ float64) {}
func (c *GeometryCanvas) Rotate(_ float64)   {}

func (c *GeometryCanvas) ClipPath(_ *graphics.Path, _ graphics.ClipOp, _ bool) {}

func (c *GeometryCanvas) Clear(_ graphics.Color)                                    {}
func (c *GeometryCanvas) DrawRect(_ graphics.Rect, _ graphics.Paint)                {}
func (c *GeometryCanvas) DrawRRect(_ graphics.RRect, _ graphics.Paint)              {}
func (c *GeometryCanvas) DrawCircle(_ graphics.Offset, _ float64, _ graphics.Paint) {}
func (c *GeometryCanvas) DrawLine(_, _ graphics.Offset, _ graphics.Paint)           {}
func (c *GeometryCanvas) DrawText(_ *graphics.TextLayout, _ graphics.Offset)        {}
func (c *GeometryCanvas) DrawImage(_ image.Image, _ graphics.Offset)                {}
func (c *GeometryCanvas) DrawImageRect(_ image.Image, _, _ graphics.Rect, _ graphics.FilterQuality, _ uintptr) {
}
func (c *GeometryCanvas) DrawPath(_ *graphics.Path, _ graphics.Paint)                       {}
func (c *GeometryCanvas) DrawRectShadow(_ graphics.Rect, _ graphics.BoxShadow)              {}
func (c *GeometryCanvas) DrawRRectShadow(_ graphics.RRect, _ graphics.BoxShadow)            {}
func (c *GeometryCanvas) DrawSVG(_ unsafe.Pointer, _ graphics.Rect)                         {}
func (c *GeometryCanvas) DrawSVGTinted(_ unsafe.Pointer, _ graphics.Rect, _ graphics.Color) {}

func (c *GeometryCanvas) EmbedPlatformView(viewID int64, size graphics.Size) {
	c.tracker.embedPlatformView(c.sink, viewID, size)
}

func (c *GeometryCanvas) Size() graphics.Size {
	return c.size
}
