package engine

import (
	"image"
	"unsafe"

	"github.com/go-drift/drift/pkg/graphics"
)

// PlatformViewSink receives resolved platform view geometry during compositing.
type PlatformViewSink interface {
	UpdateViewGeometry(viewID int64, offset graphics.Offset, size graphics.Size, clipBounds *graphics.Rect) error
}

// CompositingCanvas wraps an inner canvas and tracks transform + clip state
// so that EmbedPlatformView can resolve platform view geometry in global coordinates.
// Used in tests; production geometry resolution uses GeometryCanvas in StepFrame.
//
// Scale/Rotate are forwarded to inner but not tracked - platform views operate
// in logical coordinates (device scale is applied on the raw Skia canvas before wrapping).
type CompositingCanvas struct {
	inner   graphics.Canvas
	tracker transformTracker
	sink    PlatformViewSink
}

// NewCompositingCanvas creates a compositing canvas that wraps inner and reports
// platform view geometry to sink.
func NewCompositingCanvas(inner graphics.Canvas, sink PlatformViewSink) *CompositingCanvas {
	return &CompositingCanvas{
		inner: inner,
		sink:  sink,
	}
}

func (c *CompositingCanvas) Save() {
	c.tracker.save()
	c.inner.Save()
}

func (c *CompositingCanvas) SaveLayerAlpha(bounds graphics.Rect, alpha float64) {
	c.tracker.save()
	c.inner.SaveLayerAlpha(bounds, alpha)
}

func (c *CompositingCanvas) SaveLayer(bounds graphics.Rect, paint *graphics.Paint) {
	c.tracker.save()
	c.inner.SaveLayer(bounds, paint)
}

func (c *CompositingCanvas) Restore() {
	c.tracker.restore()
	c.inner.Restore()
}

func (c *CompositingCanvas) Translate(dx, dy float64) {
	c.tracker.translate(dx, dy)
	c.inner.Translate(dx, dy)
}

// Scale forwards to inner canvas but is NOT tracked for platform view geometry.
// Platform views operate in logical coordinates; device scale is applied on the
// raw Skia canvas before wrapping with CompositingCanvas.
func (c *CompositingCanvas) Scale(sx, sy float64) {
	c.inner.Scale(sx, sy)
}

// Rotate forwards to inner canvas but is NOT tracked for platform view geometry.
// Native views cannot be rotated — they are always axis-aligned rectangles.
func (c *CompositingCanvas) Rotate(radians float64) {
	c.inner.Rotate(radians)
}

func (c *CompositingCanvas) ClipRect(rect graphics.Rect) {
	c.tracker.clipRect(rect)
	c.inner.ClipRect(rect)
}

func (c *CompositingCanvas) ClipRRect(rrect graphics.RRect) {
	c.tracker.clipRRect(rrect)
	c.inner.ClipRRect(rrect)
}

// ClipPath forwards to inner canvas but is NOT tracked for platform view geometry.
// Native views only support rectangular clipping; path clips cannot be applied to them.
func (c *CompositingCanvas) ClipPath(path *graphics.Path, op graphics.ClipOp, antialias bool) {
	c.inner.ClipPath(path, op, antialias)
}

func (c *CompositingCanvas) Clear(color graphics.Color) {
	c.inner.Clear(color)
}

func (c *CompositingCanvas) DrawRect(rect graphics.Rect, paint graphics.Paint) {
	c.inner.DrawRect(rect, paint)
}

func (c *CompositingCanvas) DrawRRect(rrect graphics.RRect, paint graphics.Paint) {
	c.inner.DrawRRect(rrect, paint)
}

func (c *CompositingCanvas) DrawCircle(center graphics.Offset, radius float64, paint graphics.Paint) {
	c.inner.DrawCircle(center, radius, paint)
}

func (c *CompositingCanvas) DrawLine(start, end graphics.Offset, paint graphics.Paint) {
	c.inner.DrawLine(start, end, paint)
}

func (c *CompositingCanvas) DrawText(layout *graphics.TextLayout, position graphics.Offset) {
	c.inner.DrawText(layout, position)
}

func (c *CompositingCanvas) DrawImage(img image.Image, position graphics.Offset) {
	c.inner.DrawImage(img, position)
}

func (c *CompositingCanvas) DrawImageRect(img image.Image, srcRect, dstRect graphics.Rect, quality graphics.FilterQuality, cacheKey uintptr) {
	c.inner.DrawImageRect(img, srcRect, dstRect, quality, cacheKey)
}

func (c *CompositingCanvas) DrawPath(path *graphics.Path, paint graphics.Paint) {
	c.inner.DrawPath(path, paint)
}

func (c *CompositingCanvas) DrawRectShadow(rect graphics.Rect, shadow graphics.BoxShadow) {
	c.inner.DrawRectShadow(rect, shadow)
}

func (c *CompositingCanvas) DrawRRectShadow(rrect graphics.RRect, shadow graphics.BoxShadow) {
	c.inner.DrawRRectShadow(rrect, shadow)
}

func (c *CompositingCanvas) SaveLayerBlur(bounds graphics.Rect, sigmaX, sigmaY float64) {
	c.tracker.save()
	c.inner.SaveLayerBlur(bounds, sigmaX, sigmaY)
}

func (c *CompositingCanvas) DrawSVG(svgPtr unsafe.Pointer, bounds graphics.Rect) {
	c.inner.DrawSVG(svgPtr, bounds)
}

func (c *CompositingCanvas) DrawSVGTinted(svgPtr unsafe.Pointer, bounds graphics.Rect, tintColor graphics.Color) {
	c.inner.DrawSVGTinted(svgPtr, bounds, tintColor)
}

func (c *CompositingCanvas) EmbedPlatformView(viewID int64, size graphics.Size) {
	c.tracker.embedPlatformView(c.sink, viewID, size)
}

func (c *CompositingCanvas) Size() graphics.Size {
	return c.inner.Size()
}
