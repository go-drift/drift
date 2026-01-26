//go:build !android && !darwin && !ios

// Package skia provides a stub implementation for non-supported platforms.
// This allows the package to compile on platforms like linux for testing
// and development purposes, but the rendering functions are no-ops.
package skia

import (
	"errors"
	"unsafe"
)

var errStubNotSupported = errors.New("skia: not supported on this platform")

// Context wraps a Skia GPU context.
type Context struct{}

// Surface wraps a Skia GPU surface.
type Surface struct{}

// Path wraps a Skia path for vector drawing.
type Path struct{}

// NewGLContext creates a Skia GPU context using the current OpenGL context.
func NewGLContext() (*Context, error) {
	return nil, errStubNotSupported
}

// NewMetalContext creates a Skia GPU context using the provided Metal device/queue.
func NewMetalContext(device, queue unsafe.Pointer) (*Context, error) {
	return nil, errStubNotSupported
}

// Destroy releases the Skia context.
func (c *Context) Destroy() {}

// MakeGLSurface creates a Skia surface targeting the current GL framebuffer.
func (c *Context) MakeGLSurface(width, height int) (*Surface, error) {
	return nil, errStubNotSupported
}

// MakeMetalSurface creates a Skia surface targeting the provided Metal texture.
func (c *Context) MakeMetalSurface(texture unsafe.Pointer, width, height int) (*Surface, error) {
	return nil, errStubNotSupported
}

// Canvas returns the underlying Skia canvas pointer.
func (s *Surface) Canvas() unsafe.Pointer { return nil }

// Flush submits rendering commands for the surface.
func (s *Surface) Flush() {}

// Destroy releases the surface.
func (s *Surface) Destroy() {}

// CanvasSave pushes the canvas state.
func CanvasSave(canvas unsafe.Pointer) {}

// CanvasSaveLayerAlpha saves a layer with the given alpha (0-255).
func CanvasSaveLayerAlpha(canvas unsafe.Pointer, l, t, r, b float32, alpha uint8) {}

// CanvasRestore pops the canvas state.
func CanvasRestore(canvas unsafe.Pointer) {}

// CanvasTranslate translates the canvas.
func CanvasTranslate(canvas unsafe.Pointer, dx, dy float32) {}

// CanvasScale scales the canvas.
func CanvasScale(canvas unsafe.Pointer, sx, sy float32) {}

// CanvasRotate rotates the canvas.
func CanvasRotate(canvas unsafe.Pointer, radians float32) {}

// CanvasClipRect clips the canvas to the provided rect.
func CanvasClipRect(canvas unsafe.Pointer, left, top, right, bottom float32) {}

// CanvasClipRRect clips the canvas to the provided rounded rect.
func CanvasClipRRect(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	rx1, ry1 float32,
	rx2, ry2 float32,
	rx3, ry3 float32,
	rx4, ry4 float32,
) {
}

// CanvasClear clears the canvas with a solid color.
func CanvasClear(canvas unsafe.Pointer, argb uint32) {}

// CanvasDrawRect draws a rectangle.
func CanvasDrawRect(canvas unsafe.Pointer, left, top, right, bottom float32, argb uint32, style int32, strokeWidth float32, aa bool) {
}

// CanvasDrawRRect draws a rounded rectangle with per-corner radii.
func CanvasDrawRRect(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	rx1, ry1 float32,
	rx2, ry2 float32,
	rx3, ry3 float32,
	rx4, ry4 float32,
	argb uint32,
	style int32,
	strokeWidth float32,
	aa bool,
) {
}

// CanvasDrawCircle draws a circle.
func CanvasDrawCircle(canvas unsafe.Pointer, cx, cy, radius float32, argb uint32, style int32, strokeWidth float32, aa bool) {
}

// CanvasDrawLine draws a line segment.
func CanvasDrawLine(canvas unsafe.Pointer, x1, y1, x2, y2 float32, argb uint32, strokeWidth float32, aa bool) {
}

// CanvasDrawRectGradient draws a rectangle with a gradient shader.
func CanvasDrawRectGradient(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	argb uint32,
	style int32,
	strokeWidth float32,
	aa bool,
	gradientType int32,
	startX, startY, endX, endY float32,
	centerX, centerY, radius float32,
	colors []uint32,
	positions []float32,
) {
}

// CanvasDrawRRectGradient draws a rounded rectangle with a gradient shader.
func CanvasDrawRRectGradient(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	rx1, ry1, rx2, ry2, rx3, ry3, rx4, ry4 float32,
	argb uint32,
	style int32,
	strokeWidth float32,
	aa bool,
	gradientType int32,
	startX, startY, endX, endY float32,
	centerX, centerY, radius float32,
	colors []uint32,
	positions []float32,
) {
}

// CanvasDrawCircleGradient draws a circle with a gradient shader.
func CanvasDrawCircleGradient(
	canvas unsafe.Pointer,
	cx, cy, radius float32,
	argb uint32,
	style int32,
	strokeWidth float32,
	aa bool,
	gradientType int32,
	startX, startY, endX, endY float32,
	centerX, centerY, gradientRadius float32,
	colors []uint32,
	positions []float32,
) {
}

// CanvasDrawLineGradient draws a line with a gradient shader.
func CanvasDrawLineGradient(
	canvas unsafe.Pointer,
	x1, y1, x2, y2 float32,
	argb uint32,
	strokeWidth float32,
	aa bool,
	gradientType int32,
	startX, startY, endX, endY float32,
	centerX, centerY, radius float32,
	colors []uint32,
	positions []float32,
) {
}

// CanvasDrawPathGradient draws a path with a gradient shader.
func CanvasDrawPathGradient(
	canvas unsafe.Pointer,
	path *Path,
	argb uint32,
	style int32,
	strokeWidth float32,
	aa bool,
	gradientType int32,
	startX, startY, endX, endY float32,
	centerX, centerY, radius float32,
	colors []uint32,
	positions []float32,
) {
}

// CanvasDrawTextGradient draws UTF-8 text with a gradient shader.
func CanvasDrawTextGradient(
	canvas unsafe.Pointer,
	text, family string,
	x, y, size float32,
	argb uint32,
	weight int,
	style int,
	gradientType int32,
	startX, startY, endX, endY float32,
	centerX, centerY, radius float32,
	colors []uint32,
	positions []float32,
) {
}

// CanvasDrawText draws UTF-8 text with the requested typeface.
func CanvasDrawText(canvas unsafe.Pointer, text, family string, x, y, size float32, argb uint32, weight int, style int) {
}

// CanvasDrawTextShadow draws UTF-8 text with an optional blur mask filter for shadow effects.
func CanvasDrawTextShadow(canvas unsafe.Pointer, text, family string, x, y, size float32, color uint32, sigma float32, weight int, style int) {
}

// CanvasDrawImageRGBA draws an RGBA image at the provided offset.
func CanvasDrawImageRGBA(canvas unsafe.Pointer, pixels []uint8, width, height, stride int, x, y float32) {
}

// TextMetrics reports font metrics for a typeface.
type TextMetrics struct {
	Ascent  float64
	Descent float64
	Leading float64
}

// RegisterFont registers a font family with the Skia backend.
func RegisterFont(name string, data []byte) error {
	return errStubNotSupported
}

// MeasureTextWidth returns the advance width for the text.
func MeasureTextWidth(text, family string, size float64, weight int, style int) (float64, error) {
	return 0, errStubNotSupported
}

// FontMetrics returns ascent, descent, and leading for a font.
func FontMetrics(family string, size float64, weight int, style int) (TextMetrics, error) {
	return TextMetrics{}, errStubNotSupported
}

// FillType constants for path fill rules.
const (
	FillTypeWinding = 0
	FillTypeEvenOdd = 1
)

// NewPath creates a new empty path with the specified fill type.
func NewPath(fillType int) *Path {
	return &Path{}
}

// Destroy releases the path.
func (p *Path) Destroy() {}

// MoveTo starts a new subpath at the given point.
func (p *Path) MoveTo(x, y float32) {}

// LineTo adds a line segment to the path.
func (p *Path) LineTo(x, y float32) {}

// QuadTo adds a quadratic bezier segment to the path.
func (p *Path) QuadTo(x1, y1, x2, y2 float32) {}

// CubicTo adds a cubic bezier segment to the path.
func (p *Path) CubicTo(x1, y1, x2, y2, x3, y3 float32) {}

// Close closes the current subpath.
func (p *Path) Close() {}

// CanvasDrawPath draws a path with the provided paint settings.
func CanvasDrawPath(canvas unsafe.Pointer, path *Path, argb uint32, style int32, strokeWidth float32, aa bool) {
}

// CanvasDrawRectShadow draws a shadow behind a rectangle.
func CanvasDrawRectShadow(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	color uint32,
	sigma float32,
	dx, dy float32,
	spread float32,
	blurStyle int32,
) {
}

// CanvasDrawRRectShadow draws a shadow behind a rounded rectangle.
func CanvasDrawRRectShadow(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	rx1, ry1, rx2, ry2, rx3, ry3, rx4, ry4 float32,
	color uint32,
	sigma float32,
	dx, dy float32,
	spread float32,
	blurStyle int32,
) {
}

// CanvasSaveLayerBlur saves a layer with a backdrop blur effect.
func CanvasSaveLayerBlur(canvas unsafe.Pointer, left, top, right, bottom, sigmaX, sigmaY float32) {
}
