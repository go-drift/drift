//go:build android || darwin || ios
// +build android darwin ios

// Package skia provides CGO bindings to a minimal Skia shim.
//
// The static CGO directives below reference third_party/drift_skia paths relative
// to this source file. These paths work when building directly in the drift repo.
// When building with the drift CLI (drift build android/ios), these paths are
// overridden via CGO_LDFLAGS to use ~/.drift/drift_skia/ where the prebuilt
// binaries are downloaded by fetch_skia_release.sh.
package skia

/*
#cgo CXXFLAGS: -std=c++17

// Bridge code is pre-compiled into libdrift_skia.a - no Skia include paths needed
// CGO only needs our own header
#cgo CFLAGS: -I${SRCDIR}
#cgo CXXFLAGS: -I${SRCDIR}

// Android: link libdrift_skia.a (bridge + Skia combined)
#cgo android,arm64 LDFLAGS: -L${SRCDIR}/../../third_party/drift_skia/android/arm64 -ldrift_skia -lGLESv2 -lEGL -landroid -llog
#cgo android,arm LDFLAGS: -L${SRCDIR}/../../third_party/drift_skia/android/arm -ldrift_skia -lGLESv2 -lEGL -landroid -llog
#cgo android,amd64 LDFLAGS: -L${SRCDIR}/../../third_party/drift_skia/android/amd64 -ldrift_skia -lGLESv2 -lEGL -landroid -llog

// iOS device (GOOS=ios)
#cgo ios,arm64 LDFLAGS: -L${SRCDIR}/../../third_party/drift_skia/ios/arm64 -ldrift_skia -lc++ -framework Metal -framework CoreGraphics -framework Foundation -framework UIKit

// iOS simulator (GOOS=darwin, not ios)
#cgo darwin,!ios,arm64 LDFLAGS: -L${SRCDIR}/../../third_party/drift_skia/ios-simulator/arm64 -ldrift_skia -lc++ -framework Metal -framework CoreGraphics -framework Foundation -framework UIKit
#cgo darwin,!ios,amd64 LDFLAGS: -L${SRCDIR}/../../third_party/drift_skia/ios-simulator/x64 -ldrift_skia -lc++ -framework Metal -framework CoreGraphics -framework Foundation -framework UIKit

#include "skia_bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"unsafe"
)

// Context wraps a Skia GPU context.
type Context struct {
	ptr C.DriftSkiaContext
}

// Surface wraps a Skia GPU surface.
type Surface struct {
	ptr C.DriftSkiaSurface
	ctx *Context
}

// Path wraps a Skia path for vector drawing.
type Path struct {
	ptr C.DriftSkiaPath
}

// NewGLContext creates a Skia GPU context using the current OpenGL context.
func NewGLContext() (*Context, error) {
	ctx := C.drift_skia_context_create_gl()
	if ctx == nil {
		return nil, errors.New("skia: failed to create GL context")
	}
	return &Context{ptr: ctx}, nil
}

// NewMetalContext creates a Skia GPU context using the provided Metal device/queue.
func NewMetalContext(device, queue unsafe.Pointer) (*Context, error) {
	ctx := C.drift_skia_context_create_metal(device, queue)
	if ctx == nil {
		return nil, errors.New("skia: failed to create Metal context")
	}
	return &Context{ptr: ctx}, nil
}

// Destroy releases the Skia context.
func (c *Context) Destroy() {
	if c == nil || c.ptr == nil {
		return
	}
	C.drift_skia_context_destroy(c.ptr)
	c.ptr = nil
}

// MakeGLSurface creates a Skia surface targeting the current GL framebuffer.
func (c *Context) MakeGLSurface(width, height int) (*Surface, error) {
	if c == nil || c.ptr == nil {
		return nil, errors.New("skia: nil context")
	}
	surface := C.drift_skia_surface_create_gl(c.ptr, C.int(width), C.int(height))
	if surface == nil {
		return nil, errors.New("skia: failed to create GL surface")
	}
	return &Surface{ptr: surface, ctx: c}, nil
}

// MakeMetalSurface creates a Skia surface targeting the provided Metal texture.
func (c *Context) MakeMetalSurface(texture unsafe.Pointer, width, height int) (*Surface, error) {
	if c == nil || c.ptr == nil {
		return nil, errors.New("skia: nil context")
	}
	if texture == nil {
		return nil, errors.New("skia: nil texture")
	}
	surface := C.drift_skia_surface_create_metal(c.ptr, texture, C.int(width), C.int(height))
	if surface == nil {
		return nil, errors.New("skia: failed to create Metal surface")
	}
	return &Surface{ptr: surface, ctx: c}, nil
}

// Canvas returns the underlying Skia canvas pointer.
func (s *Surface) Canvas() unsafe.Pointer {
	if s == nil || s.ptr == nil {
		return nil
	}
	canvas := C.drift_skia_surface_get_canvas(s.ptr)
	return unsafe.Pointer(canvas)
}

// Flush submits rendering commands for the surface.
func (s *Surface) Flush() {
	if s == nil || s.ptr == nil || s.ctx == nil || s.ctx.ptr == nil {
		return
	}
	C.drift_skia_surface_flush(s.ctx.ptr, s.ptr)
}

// Destroy releases the surface.
func (s *Surface) Destroy() {
	if s == nil || s.ptr == nil {
		return
	}
	C.drift_skia_surface_destroy(s.ptr)
	s.ptr = nil
}

// CanvasSave pushes the canvas state.
func CanvasSave(canvas unsafe.Pointer) {
	C.drift_skia_canvas_save(C.DriftSkiaCanvas(canvas))
}

// CanvasSaveLayerAlpha saves a layer with the given alpha (0-255).
func CanvasSaveLayerAlpha(canvas unsafe.Pointer, l, t, r, b float32, alpha uint8) {
	C.drift_skia_canvas_save_layer_alpha(C.DriftSkiaCanvas(canvas), C.float(l), C.float(t), C.float(r), C.float(b), C.uint8_t(alpha))
}

// CanvasRestore pops the canvas state.
func CanvasRestore(canvas unsafe.Pointer) {
	C.drift_skia_canvas_restore(C.DriftSkiaCanvas(canvas))
}

// CanvasTranslate translates the canvas.
func CanvasTranslate(canvas unsafe.Pointer, dx, dy float32) {
	C.drift_skia_canvas_translate(C.DriftSkiaCanvas(canvas), C.float(dx), C.float(dy))
}

// CanvasScale scales the canvas.
func CanvasScale(canvas unsafe.Pointer, sx, sy float32) {
	C.drift_skia_canvas_scale(C.DriftSkiaCanvas(canvas), C.float(sx), C.float(sy))
}

// CanvasRotate rotates the canvas.
func CanvasRotate(canvas unsafe.Pointer, radians float32) {
	C.drift_skia_canvas_rotate(C.DriftSkiaCanvas(canvas), C.float(radians))
}

// CanvasClipRect clips the canvas to the provided rect.
func CanvasClipRect(canvas unsafe.Pointer, left, top, right, bottom float32) {
	C.drift_skia_canvas_clip_rect(C.DriftSkiaCanvas(canvas), C.float(left), C.float(top), C.float(right), C.float(bottom))
}

// CanvasClipRRect clips the canvas to the provided rounded rect.
func CanvasClipRRect(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	rx1, ry1 float32,
	rx2, ry2 float32,
	rx3, ry3 float32,
	rx4, ry4 float32,
) {
	C.drift_skia_canvas_clip_rrect(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.float(rx1), C.float(ry1),
		C.float(rx2), C.float(ry2),
		C.float(rx3), C.float(ry3),
		C.float(rx4), C.float(ry4),
	)
}

// CanvasClear clears the canvas with a solid color.
func CanvasClear(canvas unsafe.Pointer, argb uint32) {
	C.drift_skia_canvas_clear(C.DriftSkiaCanvas(canvas), C.uint(argb))
}

// CanvasDrawRect draws a rectangle.
func CanvasDrawRect(canvas unsafe.Pointer, left, top, right, bottom float32, argb uint32, style int32, strokeWidth float32, aa bool) {
	C.drift_skia_canvas_draw_rect(C.DriftSkiaCanvas(canvas), C.float(left), C.float(top), C.float(right), C.float(bottom), C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa))
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
	C.drift_skia_canvas_draw_rrect(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.float(rx1), C.float(ry1),
		C.float(rx2), C.float(ry2),
		C.float(rx3), C.float(ry3),
		C.float(rx4), C.float(ry4),
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
	)
}

// CanvasDrawCircle draws a circle.
func CanvasDrawCircle(canvas unsafe.Pointer, cx, cy, radius float32, argb uint32, style int32, strokeWidth float32, aa bool) {
	C.drift_skia_canvas_draw_circle(C.DriftSkiaCanvas(canvas), C.float(cx), C.float(cy), C.float(radius), C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa))
}

// CanvasDrawLine draws a line segment.
func CanvasDrawLine(canvas unsafe.Pointer, x1, y1, x2, y2 float32, argb uint32, strokeWidth float32, aa bool) {
	C.drift_skia_canvas_draw_line(C.DriftSkiaCanvas(canvas), C.float(x1), C.float(y1), C.float(x2), C.float(y2), C.uint(argb), C.float(strokeWidth), boolToInt(aa))
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
	cColors, cPositions, count := gradientData(colors, positions)
	C.drift_skia_canvas_draw_rect_gradient(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
		C.int(gradientType),
		C.float(startX), C.float(startY), C.float(endX), C.float(endY),
		C.float(centerX), C.float(centerY), C.float(radius),
		cColors, cPositions, count,
	)
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
	cColors, cPositions, count := gradientData(colors, positions)
	C.drift_skia_canvas_draw_rrect_gradient(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.float(rx1), C.float(ry1), C.float(rx2), C.float(ry2),
		C.float(rx3), C.float(ry3), C.float(rx4), C.float(ry4),
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
		C.int(gradientType),
		C.float(startX), C.float(startY), C.float(endX), C.float(endY),
		C.float(centerX), C.float(centerY), C.float(radius),
		cColors, cPositions, count,
	)
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
	cColors, cPositions, count := gradientData(colors, positions)
	C.drift_skia_canvas_draw_circle_gradient(
		C.DriftSkiaCanvas(canvas),
		C.float(cx), C.float(cy), C.float(radius),
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
		C.int(gradientType),
		C.float(startX), C.float(startY), C.float(endX), C.float(endY),
		C.float(centerX), C.float(centerY), C.float(gradientRadius),
		cColors, cPositions, count,
	)
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
	cColors, cPositions, count := gradientData(colors, positions)
	C.drift_skia_canvas_draw_line_gradient(
		C.DriftSkiaCanvas(canvas),
		C.float(x1), C.float(y1), C.float(x2), C.float(y2),
		C.uint(argb), C.float(strokeWidth), boolToInt(aa),
		C.int(gradientType),
		C.float(startX), C.float(startY), C.float(endX), C.float(endY),
		C.float(centerX), C.float(centerY), C.float(radius),
		cColors, cPositions, count,
	)
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
	if path == nil || path.ptr == nil {
		return
	}
	cColors, cPositions, count := gradientData(colors, positions)
	C.drift_skia_canvas_draw_path_gradient(
		C.DriftSkiaCanvas(canvas),
		path.ptr,
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
		C.int(gradientType),
		C.float(startX), C.float(startY), C.float(endX), C.float(endY),
		C.float(centerX), C.float(centerY), C.float(radius),
		cColors, cPositions, count,
	)
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
	cColors, cPositions, count := gradientData(colors, positions)
	cstr := C.CString(text)
	defer C.free(unsafe.Pointer(cstr))
	var cfamily *C.char
	if family != "" {
		cfamily = C.CString(family)
		defer C.free(unsafe.Pointer(cfamily))
	}
	C.drift_skia_canvas_draw_text_gradient(
		C.DriftSkiaCanvas(canvas),
		cstr,
		cfamily,
		C.float(x), C.float(y), C.float(size),
		C.uint(argb), C.int(weight), C.int(style),
		C.int(gradientType),
		C.float(startX), C.float(startY), C.float(endX), C.float(endY),
		C.float(centerX), C.float(centerY), C.float(radius),
		cColors, cPositions, count,
	)
}

// CanvasDrawText draws UTF-8 text with the requested typeface.
func CanvasDrawText(canvas unsafe.Pointer, text, family string, x, y, size float32, argb uint32, weight int, style int) {
	cstr := C.CString(text)
	defer C.free(unsafe.Pointer(cstr))
	var cfamily *C.char
	if family != "" {
		cfamily = C.CString(family)
		defer C.free(unsafe.Pointer(cfamily))
	}
	C.drift_skia_canvas_draw_text(C.DriftSkiaCanvas(canvas), cstr, cfamily, C.float(x), C.float(y), C.float(size), C.uint(argb), C.int(weight), C.int(style))
}

// CanvasDrawImageRGBA draws an RGBA image at the provided offset.
func CanvasDrawImageRGBA(canvas unsafe.Pointer, pixels []uint8, width, height, stride int, x, y float32) {
	if len(pixels) == 0 {
		return
	}
	C.drift_skia_canvas_draw_image_rgba(
		C.DriftSkiaCanvas(canvas),
		(*C.uchar)(unsafe.Pointer(&pixels[0])),
		C.int(width),
		C.int(height),
		C.int(stride),
		C.float(x),
		C.float(y),
	)
}

// TextMetrics reports font metrics for a typeface.
type TextMetrics struct {
	Ascent  float64
	Descent float64
	Leading float64
}

// RegisterFont registers a font family with the Skia backend.
func RegisterFont(name string, data []byte) error {
	if name == "" {
		return errors.New("font name required")
	}
	if len(data) == 0 {
		return errors.New("font data required")
	}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	result := C.drift_skia_register_font(cname, (*C.uchar)(unsafe.Pointer(&data[0])), C.int(len(data)))
	if result == 0 {
		return errors.New("skia: failed to register font")
	}
	return nil
}

// MeasureTextWidth returns the advance width for the text.
func MeasureTextWidth(text, family string, size float64, weight int, style int) (float64, error) {
	var width C.float
	cstr := C.CString(text)
	defer C.free(unsafe.Pointer(cstr))
	var cfamily *C.char
	if family != "" {
		cfamily = C.CString(family)
		defer C.free(unsafe.Pointer(cfamily))
	}
	result := C.drift_skia_measure_text(cstr, cfamily, C.float(size), C.int(weight), C.int(style), &width)
	if result == 0 {
		return 0, errors.New("skia: failed to measure text")
	}
	return float64(width), nil
}

// FontMetrics returns ascent, descent, and leading for a font.
func FontMetrics(family string, size float64, weight int, style int) (TextMetrics, error) {
	var ascent C.float
	var descent C.float
	var leading C.float
	var cfamily *C.char
	if family != "" {
		cfamily = C.CString(family)
		defer C.free(unsafe.Pointer(cfamily))
	}
	result := C.drift_skia_font_metrics(cfamily, C.float(size), C.int(weight), C.int(style), &ascent, &descent, &leading)
	if result == 0 {
		return TextMetrics{}, errors.New("skia: failed to get font metrics")
	}
	return TextMetrics{Ascent: float64(ascent), Descent: float64(descent), Leading: float64(leading)}, nil
}

func boolToInt(value bool) C.int {
	if value {
		return 1
	}
	return 0
}

func gradientData(colors []uint32, positions []float32) (*C.uint, *C.float, C.int) {
	if len(colors) == 0 || len(colors) != len(positions) {
		return nil, nil, 0
	}
	return (*C.uint)(unsafe.Pointer(&colors[0])), (*C.float)(unsafe.Pointer(&positions[0])), C.int(len(colors))
}

// FillType constants for path fill rules.
const (
	FillTypeWinding = 0
	FillTypeEvenOdd = 1
)

// NewPath creates a new empty path with the specified fill type.
// Use FillTypeWinding (0) for nonzero winding rule, FillTypeEvenOdd (1) for even-odd rule.
func NewPath(fillType int) *Path {
	return &Path{ptr: C.drift_skia_path_create(C.int(fillType))}
}

// Destroy releases the path.
func (p *Path) Destroy() {
	if p == nil || p.ptr == nil {
		return
	}
	C.drift_skia_path_destroy(p.ptr)
	p.ptr = nil
}

// MoveTo starts a new subpath at the given point.
func (p *Path) MoveTo(x, y float32) {
	if p == nil || p.ptr == nil {
		return
	}
	C.drift_skia_path_move_to(p.ptr, C.float(x), C.float(y))
}

// LineTo adds a line segment to the path.
func (p *Path) LineTo(x, y float32) {
	if p == nil || p.ptr == nil {
		return
	}
	C.drift_skia_path_line_to(p.ptr, C.float(x), C.float(y))
}

// QuadTo adds a quadratic bezier segment to the path.
func (p *Path) QuadTo(x1, y1, x2, y2 float32) {
	if p == nil || p.ptr == nil {
		return
	}
	C.drift_skia_path_quad_to(p.ptr, C.float(x1), C.float(y1), C.float(x2), C.float(y2))
}

// CubicTo adds a cubic bezier segment to the path.
func (p *Path) CubicTo(x1, y1, x2, y2, x3, y3 float32) {
	if p == nil || p.ptr == nil {
		return
	}
	C.drift_skia_path_cubic_to(p.ptr, C.float(x1), C.float(y1), C.float(x2), C.float(y2), C.float(x3), C.float(y3))
}

// Close closes the current subpath.
func (p *Path) Close() {
	if p == nil || p.ptr == nil {
		return
	}
	C.drift_skia_path_close(p.ptr)
}

// CanvasDrawPath draws a path with the provided paint settings.
func CanvasDrawPath(canvas unsafe.Pointer, path *Path, argb uint32, style int32, strokeWidth float32, aa bool) {
	if path == nil || path.ptr == nil {
		return
	}
	C.drift_skia_canvas_draw_path(C.DriftSkiaCanvas(canvas), path.ptr, C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa))
}
