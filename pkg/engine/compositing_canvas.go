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
//
// Scale/Rotate are forwarded to inner but not tracked - platform views operate
// in logical coordinates (device scale is applied on the raw Skia canvas before wrapping).
type CompositingCanvas struct {
	inner     graphics.Canvas
	transform graphics.Offset // accumulated translation
	saveStack []saveState
	clips     []graphics.Rect // intersection-reduced clip stack
	sink      PlatformViewSink
}

type saveState struct {
	transform graphics.Offset
	clipDepth int
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
	c.saveStack = append(c.saveStack, saveState{
		transform: c.transform,
		clipDepth: len(c.clips),
	})
	c.inner.Save()
}

func (c *CompositingCanvas) SaveLayerAlpha(bounds graphics.Rect, alpha float64) {
	c.saveStack = append(c.saveStack, saveState{
		transform: c.transform,
		clipDepth: len(c.clips),
	})
	c.inner.SaveLayerAlpha(bounds, alpha)
}

func (c *CompositingCanvas) SaveLayer(bounds graphics.Rect, paint *graphics.Paint) {
	c.saveStack = append(c.saveStack, saveState{
		transform: c.transform,
		clipDepth: len(c.clips),
	})
	c.inner.SaveLayer(bounds, paint)
}

func (c *CompositingCanvas) Restore() {
	if len(c.saveStack) > 0 {
		state := c.saveStack[len(c.saveStack)-1]
		c.saveStack = c.saveStack[:len(c.saveStack)-1]
		c.transform = state.transform
		c.clips = c.clips[:state.clipDepth]
	}
	c.inner.Restore()
}

func (c *CompositingCanvas) Translate(dx, dy float64) {
	c.transform.X += dx
	c.transform.Y += dy
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
	// Transform to global coordinates
	globalRect := rect.Translate(c.transform.X, c.transform.Y)

	// Intersect with current clip if any
	if len(c.clips) > 0 {
		globalRect = c.clips[len(c.clips)-1].Intersect(globalRect)
	}

	c.clips = append(c.clips, globalRect)
	c.inner.ClipRect(rect)
}

func (c *CompositingCanvas) ClipRRect(rrect graphics.RRect) {
	// Approximate as rect clip for platform view geometry tracking
	globalRect := rrect.Rect.Translate(c.transform.X, c.transform.Y)
	if len(c.clips) > 0 {
		globalRect = c.clips[len(c.clips)-1].Intersect(globalRect)
	}
	c.clips = append(c.clips, globalRect)
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
	c.saveStack = append(c.saveStack, saveState{
		transform: c.transform,
		clipDepth: len(c.clips),
	})
	c.inner.SaveLayerBlur(bounds, sigmaX, sigmaY)
}

func (c *CompositingCanvas) DrawSVG(svgPtr unsafe.Pointer, bounds graphics.Rect) {
	c.inner.DrawSVG(svgPtr, bounds)
}

func (c *CompositingCanvas) DrawSVGTinted(svgPtr unsafe.Pointer, bounds graphics.Rect, tintColor graphics.Color) {
	c.inner.DrawSVGTinted(svgPtr, bounds, tintColor)
}

func (c *CompositingCanvas) EmbedPlatformView(viewID int64, size graphics.Size) {
	if c.sink == nil {
		return
	}

	// Compute global offset from accumulated transform
	offset := c.transform

	// Compute clip bounds if any clips are active.
	// Even fully-clipped (empty) rects are sent so the sink can hide the view.
	var clipBounds *graphics.Rect
	if len(c.clips) > 0 {
		clip := c.clips[len(c.clips)-1]
		clipBounds = &clip
	}

	c.sink.UpdateViewGeometry(viewID, offset, size, clipBounds)
}

func (c *CompositingCanvas) Size() graphics.Size {
	return c.inner.Size()
}
