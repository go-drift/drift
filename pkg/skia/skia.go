//go:build android || darwin || ios

// Package skia provides CGO bindings to a minimal Skia shim.
//
// The static CGO directives below reference third_party/drift_skia paths relative
// to this source file. These paths work when building directly in the drift repo.
// When building with the drift CLI (drift build android/ios), these paths are
// overridden via CGO_LDFLAGS to use prebuilt binaries from ~/.drift/lib/.
package skia

/*
#cgo CXXFLAGS: -std=c++17

// Bridge code is pre-compiled into libdrift_skia.a - no Skia include paths needed
// CGO only needs our own header
#cgo CFLAGS: -I${SRCDIR}
#cgo CXXFLAGS: -I${SRCDIR}

// Android: link libdrift_skia.a (bridge + Skia combined)
#cgo android,arm64 LDFLAGS: -L${SRCDIR}/../../third_party/drift_skia/android/arm64 -ldrift_skia -lc++_shared -lvulkan -landroid -llog -lm
#cgo android,arm LDFLAGS: -L${SRCDIR}/../../third_party/drift_skia/android/arm -ldrift_skia -lc++_shared -lvulkan -landroid -llog -lm
#cgo android,amd64 LDFLAGS: -L${SRCDIR}/../../third_party/drift_skia/android/amd64 -ldrift_skia -lc++_shared -lvulkan -landroid -llog -lm

// iOS device (GOOS=ios)
#cgo ios,arm64 LDFLAGS: -L${SRCDIR}/../../third_party/drift_skia/ios/arm64 -ldrift_skia -lc++ -framework Metal -framework CoreGraphics -framework Foundation -framework UIKit

// iOS simulator (GOOS=darwin, not ios)
#cgo darwin,!ios,arm64 LDFLAGS: -L${SRCDIR}/../../third_party/drift_skia/ios-simulator/arm64 -ldrift_skia -lc++ -framework Metal -framework CoreGraphics -framework Foundation -framework UIKit
#cgo darwin,!ios,amd64 LDFLAGS: -L${SRCDIR}/../../third_party/drift_skia/ios-simulator/amd64 -ldrift_skia -lc++ -framework Metal -framework CoreGraphics -framework Foundation -framework UIKit

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

// Paragraph wraps a Skia text layout paragraph.
type Paragraph struct {
	ptr C.DriftSkiaParagraph
}

// ParagraphShadow describes a paragraph shadow effect.
type ParagraphShadow struct {
	Color   uint32
	OffsetX float32
	OffsetY float32
	Sigma   float32
}

// ParagraphMetrics reports paragraph layout metrics.
type ParagraphMetrics struct {
	Height            float64
	LongestLine       float64
	MaxIntrinsicWidth float64
	LineCount         int
}

// ParagraphLineMetrics reports per-line layout metrics.
type ParagraphLineMetrics struct {
	Widths   []float64
	Ascents  []float64
	Descents []float64
	Heights  []float64
}

// NewMetalContext creates a Skia GPU context using the provided Metal device/queue.
func NewMetalContext(device, queue unsafe.Pointer) (*Context, error) {
	ctx := C.drift_skia_context_create_metal(device, queue)
	if ctx == nil {
		return nil, errors.New("skia: failed to create Metal context")
	}
	return &Context{ptr: ctx}, nil
}

// NewVulkanContext creates a Skia GPU context using the provided Vulkan handles.
func NewVulkanContext(instance, physDevice, device, queue uintptr, queueFamilyIndex uint32, getInstanceProcAddr uintptr) (*Context, error) {
	ctx := C.drift_skia_context_create_vulkan(
		C.uintptr_t(instance),
		C.uintptr_t(physDevice),
		C.uintptr_t(device),
		C.uintptr_t(queue),
		C.uint32_t(queueFamilyIndex),
		C.uintptr_t(getInstanceProcAddr),
	)
	if ctx == nil {
		return nil, errors.New("skia: failed to create Vulkan context")
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

// FlushAndSubmit flushes pending GPU work and optionally waits for completion.
// If syncCPU is true, blocks until all GPU work finishes.
func (c *Context) FlushAndSubmit(syncCPU bool) {
	if c == nil || c.ptr == nil {
		return
	}
	sync := C.int(0)
	if syncCPU {
		sync = 1
	}
	C.drift_skia_context_flush_and_submit(c.ptr, sync)
}

// PurgeGpuResources releases all cached GPU resources (glyph atlases, texture caches, etc.). Call this after events
// that may invalidate GPU memory, such as sleep/wake or surface recreation.
func (c *Context) PurgeGpuResources() {
	if c == nil || c.ptr == nil {
		return
	}
	C.drift_skia_context_purge_resources(c.ptr)
}

// MakeOffscreenSurfaceMetal creates a GPU-backed offscreen surface for Metal.
func (c *Context) MakeOffscreenSurfaceMetal(width, height int) (*Surface, error) {
	if c == nil || c.ptr == nil {
		return nil, errors.New("skia: nil context")
	}
	surface := C.drift_skia_surface_create_offscreen_metal(c.ptr, C.int(width), C.int(height))
	if surface == nil {
		return nil, errors.New("skia: failed to create offscreen Metal surface")
	}
	return &Surface{ptr: surface, ctx: c}, nil
}

// MakeVulkanSurface creates a Skia surface wrapping the provided VkImage.
func (c *Context) MakeVulkanSurface(width, height int, vkImage uintptr, vkFormat uint32) (*Surface, error) {
	if c == nil || c.ptr == nil {
		return nil, errors.New("skia: nil context")
	}
	surface := C.drift_skia_surface_create_vulkan(c.ptr, C.int(width), C.int(height), C.uintptr_t(vkImage), C.uint32_t(vkFormat))
	if surface == nil {
		return nil, errors.New("skia: failed to create Vulkan surface")
	}
	return &Surface{ptr: surface, ctx: c}, nil
}

// MakeOffscreenSurfaceVulkan creates a GPU-backed offscreen surface for Vulkan.
func (c *Context) MakeOffscreenSurfaceVulkan(width, height int) (*Surface, error) {
	if c == nil || c.ptr == nil {
		return nil, errors.New("skia: nil context")
	}
	surface := C.drift_skia_surface_create_offscreen_vulkan(c.ptr, C.int(width), C.int(height))
	if surface == nil {
		return nil, errors.New("skia: failed to create offscreen Vulkan surface")
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

// CanvasClipPath clips the canvas to an arbitrary path.
func CanvasClipPath(canvas unsafe.Pointer, path *Path, clipOp int32, antialias bool) {
	if path == nil || path.ptr == nil {
		return
	}
	aa := C.int(0)
	if antialias {
		aa = 1
	}
	C.drift_skia_canvas_clip_path(
		C.DriftSkiaCanvas(canvas),
		path.ptr,
		C.int(clipOp),
		aa,
	)
}

// CanvasSaveLayer saves a layer with blend mode and alpha compositing.
func CanvasSaveLayer(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	blendMode int32, alpha float32,
) {
	C.drift_skia_canvas_save_layer(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.int(blendMode), C.float(alpha),
	)
}

// CanvasSaveLayerFiltered saves a layer with optional color and image filters.
func CanvasSaveLayerFiltered(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	blendMode int32, alpha float32,
	colorFilterData []float32,
	imageFilterData []float32,
) {
	var cfPtr *C.float
	cfLen := C.int(0)
	if len(colorFilterData) > 0 {
		cfPtr = (*C.float)(unsafe.Pointer(&colorFilterData[0]))
		cfLen = C.int(len(colorFilterData))
	}
	var ifPtr *C.float
	ifLen := C.int(0)
	if len(imageFilterData) > 0 {
		ifPtr = (*C.float)(unsafe.Pointer(&imageFilterData[0]))
		ifLen = C.int(len(imageFilterData))
	}
	C.drift_skia_canvas_save_layer_filtered(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.int(blendMode), C.float(alpha),
		cfPtr, cfLen,
		ifPtr, ifLen,
	)
}

// CanvasClear clears the canvas with a solid color.
func CanvasClear(canvas unsafe.Pointer, argb uint32) {
	C.drift_skia_canvas_clear(C.DriftSkiaCanvas(canvas), C.uint(argb))
}

// CanvasDrawRect draws a rectangle.
func CanvasDrawRect(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	argb uint32, style int32, strokeWidth float32, aa bool,
	strokeCap, strokeJoin int32, miterLimit float32,
	dashIntervals []float32, dashPhase float32,
	blendMode int32, alpha float32,
) {
	var dashPtr *C.float
	dashCount := C.int(0)
	if len(dashIntervals) >= 2 {
		dashPtr = (*C.float)(unsafe.Pointer(&dashIntervals[0]))
		dashCount = C.int(len(dashIntervals))
	}
	C.drift_skia_canvas_draw_rect(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
		C.int(strokeCap), C.int(strokeJoin), C.float(miterLimit),
		dashPtr, dashCount, C.float(dashPhase),
		C.int(blendMode), C.float(alpha),
	)
}

// CanvasDrawRRect draws a rounded rectangle with per-corner radii.
func CanvasDrawRRect(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	rx1, ry1, rx2, ry2, rx3, ry3, rx4, ry4 float32,
	argb uint32, style int32, strokeWidth float32, aa bool,
	strokeCap, strokeJoin int32, miterLimit float32,
	dashIntervals []float32, dashPhase float32,
	blendMode int32, alpha float32,
) {
	var dashPtr *C.float
	dashCount := C.int(0)
	if len(dashIntervals) >= 2 {
		dashPtr = (*C.float)(unsafe.Pointer(&dashIntervals[0]))
		dashCount = C.int(len(dashIntervals))
	}
	C.drift_skia_canvas_draw_rrect(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.float(rx1), C.float(ry1),
		C.float(rx2), C.float(ry2),
		C.float(rx3), C.float(ry3),
		C.float(rx4), C.float(ry4),
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
		C.int(strokeCap), C.int(strokeJoin), C.float(miterLimit),
		dashPtr, dashCount, C.float(dashPhase),
		C.int(blendMode), C.float(alpha),
	)
}

// CanvasDrawCircle draws a circle.
func CanvasDrawCircle(
	canvas unsafe.Pointer,
	cx, cy, radius float32,
	argb uint32, style int32, strokeWidth float32, aa bool,
	strokeCap, strokeJoin int32, miterLimit float32,
	dashIntervals []float32, dashPhase float32,
	blendMode int32, alpha float32,
) {
	var dashPtr *C.float
	dashCount := C.int(0)
	if len(dashIntervals) >= 2 {
		dashPtr = (*C.float)(unsafe.Pointer(&dashIntervals[0]))
		dashCount = C.int(len(dashIntervals))
	}
	C.drift_skia_canvas_draw_circle(
		C.DriftSkiaCanvas(canvas),
		C.float(cx), C.float(cy), C.float(radius),
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
		C.int(strokeCap), C.int(strokeJoin), C.float(miterLimit),
		dashPtr, dashCount, C.float(dashPhase),
		C.int(blendMode), C.float(alpha),
	)
}

// CanvasDrawLine draws a line segment.
func CanvasDrawLine(
	canvas unsafe.Pointer,
	x1, y1, x2, y2 float32,
	argb uint32, strokeWidth float32, aa bool,
	strokeCap, strokeJoin int32, miterLimit float32,
	dashIntervals []float32, dashPhase float32,
	blendMode int32, alpha float32,
) {
	var dashPtr *C.float
	dashCount := C.int(0)
	if len(dashIntervals) >= 2 {
		dashPtr = (*C.float)(unsafe.Pointer(&dashIntervals[0]))
		dashCount = C.int(len(dashIntervals))
	}
	C.drift_skia_canvas_draw_line(
		C.DriftSkiaCanvas(canvas),
		C.float(x1), C.float(y1), C.float(x2), C.float(y2),
		C.uint(argb), C.float(strokeWidth), boolToInt(aa),
		C.int(strokeCap), C.int(strokeJoin), C.float(miterLimit),
		dashPtr, dashCount, C.float(dashPhase),
		C.int(blendMode), C.float(alpha),
	)
}

// CanvasDrawRectGradient draws a rectangle with a gradient shader.
func CanvasDrawRectGradient(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	argb uint32, style int32, strokeWidth float32, aa bool,
	strokeCap, strokeJoin int32, miterLimit float32,
	dashIntervals []float32, dashPhase float32,
	blendMode int32, alpha float32,
	gradientType int32,
	startX, startY, endX, endY float32,
	centerX, centerY, radius float32,
	colors []uint32, positions []float32,
) {
	var dashPtr *C.float
	dashCount := C.int(0)
	if len(dashIntervals) >= 2 {
		dashPtr = (*C.float)(unsafe.Pointer(&dashIntervals[0]))
		dashCount = C.int(len(dashIntervals))
	}
	cColors, cPositions, count := gradientData(colors, positions)
	C.drift_skia_canvas_draw_rect_gradient(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
		C.int(strokeCap), C.int(strokeJoin), C.float(miterLimit),
		dashPtr, dashCount, C.float(dashPhase),
		C.int(blendMode), C.float(alpha),
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
	argb uint32, style int32, strokeWidth float32, aa bool,
	strokeCap, strokeJoin int32, miterLimit float32,
	dashIntervals []float32, dashPhase float32,
	blendMode int32, alpha float32,
	gradientType int32,
	startX, startY, endX, endY float32,
	centerX, centerY, radius float32,
	colors []uint32, positions []float32,
) {
	var dashPtr *C.float
	dashCount := C.int(0)
	if len(dashIntervals) >= 2 {
		dashPtr = (*C.float)(unsafe.Pointer(&dashIntervals[0]))
		dashCount = C.int(len(dashIntervals))
	}
	cColors, cPositions, count := gradientData(colors, positions)
	C.drift_skia_canvas_draw_rrect_gradient(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.float(rx1), C.float(ry1), C.float(rx2), C.float(ry2),
		C.float(rx3), C.float(ry3), C.float(rx4), C.float(ry4),
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
		C.int(strokeCap), C.int(strokeJoin), C.float(miterLimit),
		dashPtr, dashCount, C.float(dashPhase),
		C.int(blendMode), C.float(alpha),
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
	argb uint32, style int32, strokeWidth float32, aa bool,
	strokeCap, strokeJoin int32, miterLimit float32,
	dashIntervals []float32, dashPhase float32,
	blendMode int32, alpha float32,
	gradientType int32,
	startX, startY, endX, endY float32,
	centerX, centerY, gradientRadius float32,
	colors []uint32, positions []float32,
) {
	var dashPtr *C.float
	dashCount := C.int(0)
	if len(dashIntervals) >= 2 {
		dashPtr = (*C.float)(unsafe.Pointer(&dashIntervals[0]))
		dashCount = C.int(len(dashIntervals))
	}
	cColors, cPositions, count := gradientData(colors, positions)
	C.drift_skia_canvas_draw_circle_gradient(
		C.DriftSkiaCanvas(canvas),
		C.float(cx), C.float(cy), C.float(radius),
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
		C.int(strokeCap), C.int(strokeJoin), C.float(miterLimit),
		dashPtr, dashCount, C.float(dashPhase),
		C.int(blendMode), C.float(alpha),
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
	argb uint32, strokeWidth float32, aa bool,
	strokeCap, strokeJoin int32, miterLimit float32,
	dashIntervals []float32, dashPhase float32,
	blendMode int32, alpha float32,
	gradientType int32,
	startX, startY, endX, endY float32,
	centerX, centerY, radius float32,
	colors []uint32, positions []float32,
) {
	var dashPtr *C.float
	dashCount := C.int(0)
	if len(dashIntervals) >= 2 {
		dashPtr = (*C.float)(unsafe.Pointer(&dashIntervals[0]))
		dashCount = C.int(len(dashIntervals))
	}
	cColors, cPositions, count := gradientData(colors, positions)
	C.drift_skia_canvas_draw_line_gradient(
		C.DriftSkiaCanvas(canvas),
		C.float(x1), C.float(y1), C.float(x2), C.float(y2),
		C.uint(argb), C.float(strokeWidth), boolToInt(aa),
		C.int(strokeCap), C.int(strokeJoin), C.float(miterLimit),
		dashPtr, dashCount, C.float(dashPhase),
		C.int(blendMode), C.float(alpha),
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
	argb uint32, style int32, strokeWidth float32, aa bool,
	strokeCap, strokeJoin int32, miterLimit float32,
	dashIntervals []float32, dashPhase float32,
	blendMode int32, alpha float32,
	gradientType int32,
	startX, startY, endX, endY float32,
	centerX, centerY, radius float32,
	colors []uint32, positions []float32,
) {
	if path == nil || path.ptr == nil {
		return
	}
	var dashPtr *C.float
	dashCount := C.int(0)
	if len(dashIntervals) >= 2 {
		dashPtr = (*C.float)(unsafe.Pointer(&dashIntervals[0]))
		dashCount = C.int(len(dashIntervals))
	}
	cColors, cPositions, count := gradientData(colors, positions)
	C.drift_skia_canvas_draw_path_gradient(
		C.DriftSkiaCanvas(canvas),
		path.ptr,
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
		C.int(strokeCap), C.int(strokeJoin), C.float(miterLimit),
		dashPtr, dashCount, C.float(dashPhase),
		C.int(blendMode), C.float(alpha),
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

// CanvasDrawTextShadow draws UTF-8 text with an optional blur mask filter for shadow effects.
func CanvasDrawTextShadow(canvas unsafe.Pointer, text, family string, x, y, size float32, color uint32, sigma float32, weight int, style int) {
	cstr := C.CString(text)
	defer C.free(unsafe.Pointer(cstr))
	var cfamily *C.char
	if family != "" {
		cfamily = C.CString(family)
		defer C.free(unsafe.Pointer(cfamily))
	}
	C.drift_skia_canvas_draw_text_shadow(C.DriftSkiaCanvas(canvas), cstr, cfamily, C.float(x), C.float(y), C.float(size), C.uint(color), C.float(sigma), C.int(weight), C.int(style))
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

// CanvasDrawImageRect draws an RGBA image from srcRect to dstRect with sampling quality.
func CanvasDrawImageRect(
	canvas unsafe.Pointer,
	pixels []uint8, width, height, stride int,
	srcL, srcT, srcR, srcB float32,
	dstL, dstT, dstR, dstB float32,
	filterQuality int,
	cacheKey uintptr,
) {
	if len(pixels) == 0 || stride <= 0 {
		return
	}
	C.drift_skia_canvas_draw_image_rect(
		C.DriftSkiaCanvas(canvas),
		(*C.uchar)(unsafe.Pointer(&pixels[0])),
		C.int(width), C.int(height), C.int(stride),
		C.float(srcL), C.float(srcT), C.float(srcR), C.float(srcB),
		C.float(dstL), C.float(dstT), C.float(dstR), C.float(dstB),
		C.int(filterQuality),
		C.uintptr_t(cacheKey),
	)
}

// NewParagraph creates a paragraph layout with shaping support.
func NewParagraph(
	text, family string,
	size float32,
	weight int,
	style int,
	color uint32,
	maxLines int,
	gradientType int32,
	startX, startY, endX, endY float32,
	centerX, centerY, radius float32,
	colors []uint32,
	positions []float32,
	shadow *ParagraphShadow,
	textAlign int,
) (*Paragraph, error) {
	cstr := C.CString(text)
	defer C.free(unsafe.Pointer(cstr))
	var cfamily *C.char
	if family != "" {
		cfamily = C.CString(family)
		defer C.free(unsafe.Pointer(cfamily))
	}
	var shadowEnabled C.int
	var shadowColor C.uint
	var shadowDx C.float
	var shadowDy C.float
	var shadowSigma C.float
	if shadow != nil {
		shadowEnabled = 1
		shadowColor = C.uint(shadow.Color)
		shadowDx = C.float(shadow.OffsetX)
		shadowDy = C.float(shadow.OffsetY)
		shadowSigma = C.float(shadow.Sigma)
	}
	cColors, cPositions, count := gradientData(colors, positions)
	paragraph := C.drift_skia_paragraph_create(
		cstr,
		cfamily,
		C.float(size),
		C.int(weight),
		C.int(style),
		C.uint(color),
		C.int(maxLines),
		C.int(gradientType),
		C.float(startX),
		C.float(startY),
		C.float(endX),
		C.float(endY),
		C.float(centerX),
		C.float(centerY),
		C.float(radius),
		cColors,
		cPositions,
		count,
		shadowEnabled,
		shadowColor,
		shadowDx,
		shadowDy,
		shadowSigma,
		C.int(textAlign),
	)
	if paragraph == nil {
		return nil, errors.New("skia: failed to create paragraph")
	}
	return &Paragraph{ptr: paragraph}, nil
}

// NewRichParagraph creates a paragraph with multiple styled spans.
func NewRichParagraph(spans []TextSpanData, maxLines int, textAlign int) (*Paragraph, error) {
	if len(spans) == 0 {
		return nil, errors.New("skia: no spans provided")
	}
	cSpans := make([]C.DriftTextSpan, len(spans))
	cStrings := make([]*C.char, 0, len(spans)*2)
	for i, s := range spans {
		cText := C.CString(s.Text)
		cStrings = append(cStrings, cText)
		cSpans[i].text = cText
		if s.Family != "" {
			cFamily := C.CString(s.Family)
			cStrings = append(cStrings, cFamily)
			cSpans[i].family = cFamily
		}
		cSpans[i].size = C.float(s.Size)
		cSpans[i].weight = C.int(s.Weight)
		cSpans[i].style = C.int(s.Style)
		cSpans[i].color = C.uint32_t(s.Color)
		cSpans[i].decoration = C.int(s.Decoration)
		cSpans[i].decoration_color = C.uint32_t(s.DecorationColor)
		cSpans[i].decoration_style = C.int(s.DecorationStyle)
		cSpans[i].letter_spacing = C.float(s.LetterSpacing)
		cSpans[i].word_spacing = C.float(s.WordSpacing)
		cSpans[i].height = C.float(s.Height)
		if s.HasBackground {
			cSpans[i].has_background = 1
		}
		cSpans[i].background_color = C.uint32_t(s.BackgroundColor)
	}
	defer func() {
		for _, cs := range cStrings {
			C.free(unsafe.Pointer(cs))
		}
	}()
	paragraph := C.drift_skia_rich_paragraph_create(
		&cSpans[0],
		C.int(len(spans)),
		C.int(maxLines),
		C.int(textAlign),
	)
	if paragraph == nil {
		return nil, errors.New("skia: failed to create rich paragraph")
	}
	return &Paragraph{ptr: paragraph}, nil
}

// Layout lays out the paragraph within the given width.
func (p *Paragraph) Layout(width float32) {
	if p == nil || p.ptr == nil {
		return
	}
	C.drift_skia_paragraph_layout(p.ptr, C.float(width))
}

// Metrics returns overall paragraph metrics.
func (p *Paragraph) Metrics() (ParagraphMetrics, error) {
	if p == nil || p.ptr == nil {
		return ParagraphMetrics{}, errors.New("skia: nil paragraph")
	}
	var height C.float
	var longestLine C.float
	var maxIntrinsic C.float
	var lineCount C.int
	result := C.drift_skia_paragraph_get_metrics(p.ptr, &height, &longestLine, &maxIntrinsic, &lineCount)
	if result == 0 {
		return ParagraphMetrics{}, errors.New("skia: failed to get paragraph metrics")
	}
	return ParagraphMetrics{
		Height:            float64(height),
		LongestLine:       float64(longestLine),
		MaxIntrinsicWidth: float64(maxIntrinsic),
		LineCount:         int(lineCount),
	}, nil
}

// LineMetrics returns per-line metrics for the paragraph.
func (p *Paragraph) LineMetrics() (ParagraphLineMetrics, error) {
	metrics, err := p.Metrics()
	if err != nil {
		return ParagraphLineMetrics{}, err
	}
	if metrics.LineCount == 0 {
		return ParagraphLineMetrics{}, nil
	}
	widths := make([]float32, metrics.LineCount)
	ascents := make([]float32, metrics.LineCount)
	descents := make([]float32, metrics.LineCount)
	heights := make([]float32, metrics.LineCount)
	result := C.drift_skia_paragraph_get_line_metrics(
		p.ptr,
		(*C.float)(unsafe.Pointer(&widths[0])),
		(*C.float)(unsafe.Pointer(&ascents[0])),
		(*C.float)(unsafe.Pointer(&descents[0])),
		(*C.float)(unsafe.Pointer(&heights[0])),
		C.int(metrics.LineCount),
	)
	if result == 0 {
		return ParagraphLineMetrics{}, errors.New("skia: failed to get paragraph line metrics")
	}
	out := ParagraphLineMetrics{
		Widths:   make([]float64, metrics.LineCount),
		Ascents:  make([]float64, metrics.LineCount),
		Descents: make([]float64, metrics.LineCount),
		Heights:  make([]float64, metrics.LineCount),
	}
	for i := 0; i < metrics.LineCount; i++ {
		out.Widths[i] = float64(widths[i])
		out.Ascents[i] = float64(ascents[i])
		out.Descents[i] = float64(descents[i])
		out.Heights[i] = float64(heights[i])
	}
	return out, nil
}

// Paint renders the paragraph to the canvas at the given position.
func (p *Paragraph) Paint(canvas unsafe.Pointer, x, y float32) {
	if p == nil || p.ptr == nil || canvas == nil {
		return
	}
	C.drift_skia_paragraph_paint(p.ptr, C.DriftSkiaCanvas(canvas), C.float(x), C.float(y))
}

// Destroy releases the paragraph resources.
func (p *Paragraph) Destroy() {
	if p == nil || p.ptr == nil {
		return
	}
	C.drift_skia_paragraph_destroy(p.ptr)
	p.ptr = nil
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
func CanvasDrawPath(
	canvas unsafe.Pointer,
	path *Path,
	argb uint32, style int32, strokeWidth float32, aa bool,
	strokeCap, strokeJoin int32, miterLimit float32,
	dashIntervals []float32, dashPhase float32,
	blendMode int32, alpha float32,
) {
	if path == nil || path.ptr == nil {
		return
	}
	var dashPtr *C.float
	dashCount := C.int(0)
	if len(dashIntervals) >= 2 {
		dashPtr = (*C.float)(unsafe.Pointer(&dashIntervals[0]))
		dashCount = C.int(len(dashIntervals))
	}
	C.drift_skia_canvas_draw_path(
		C.DriftSkiaCanvas(canvas),
		path.ptr,
		C.uint(argb), C.int(style), C.float(strokeWidth), boolToInt(aa),
		C.int(strokeCap), C.int(strokeJoin), C.float(miterLimit),
		dashPtr, dashCount, C.float(dashPhase),
		C.int(blendMode), C.float(alpha),
	)
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
	C.drift_skia_canvas_draw_rect_shadow(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.uint(color), C.float(sigma), C.float(dx), C.float(dy), C.float(spread), C.int(blurStyle),
	)
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
	C.drift_skia_canvas_draw_rrect_shadow(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.float(rx1), C.float(ry1),
		C.float(rx2), C.float(ry2),
		C.float(rx3), C.float(ry3),
		C.float(rx4), C.float(ry4),
		C.uint(color), C.float(sigma), C.float(dx), C.float(dy), C.float(spread), C.int(blurStyle),
	)
}

// CanvasSaveLayerBlur saves a layer with a backdrop blur effect.
func CanvasSaveLayerBlur(canvas unsafe.Pointer, left, top, right, bottom, sigmaX, sigmaY float32) {
	C.drift_skia_canvas_save_layer_blur(
		C.DriftSkiaCanvas(canvas),
		C.float(left), C.float(top), C.float(right), C.float(bottom),
		C.float(sigmaX), C.float(sigmaY),
	)
}

// SVGDOM wraps a Skia SVG DOM for rendering vector graphics.
type SVGDOM struct {
	ptr C.DriftSkiaSVGDOM
}

// NewSVGDOM creates an SVGDOM from SVG data.
func NewSVGDOM(data []byte) *SVGDOM {
	if len(data) == 0 {
		return nil
	}
	ptr := C.drift_skia_svg_dom_create(
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.int(len(data)),
	)
	if ptr == nil {
		return nil
	}
	return &SVGDOM{ptr: ptr}
}

// NewSVGDOMWithBase creates an SVGDOM with a base path for resolving relative resources.
// If basePath is empty, this is equivalent to NewSVGDOM.
func NewSVGDOMWithBase(data []byte, basePath string) *SVGDOM {
	if basePath == "" {
		return NewSVGDOM(data)
	}
	if len(data) == 0 {
		return nil
	}
	cBasePath := C.CString(basePath)
	defer C.free(unsafe.Pointer(cBasePath))
	ptr := C.drift_skia_svg_dom_create_with_base(
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.int(len(data)),
		cBasePath,
	)
	if ptr == nil {
		return nil
	}
	return &SVGDOM{ptr: ptr}
}

// Destroy releases the SVG DOM resources.
//
// Note: Prefer using svg.Icon.Destroy() instead, which includes debug tracking
// to detect use-after-free in svgdebug builds. Direct SVGDOM.Destroy() bypasses
// those checks.
func (s *SVGDOM) Destroy() {
	if s == nil || s.ptr == nil {
		return
	}
	C.drift_skia_svg_dom_destroy(s.ptr)
	s.ptr = nil
}

// Ptr returns the underlying C handle for use in DrawSVG.
// The returned pointer is stable (not subject to Go GC).
// Returns nil if the SVGDOM is nil or has been destroyed.
func (s *SVGDOM) Ptr() unsafe.Pointer {
	if s == nil || s.ptr == nil {
		return nil
	}
	return unsafe.Pointer(s.ptr)
}

// RenderToCanvas renders the SVG directly to a Skia canvas.
// For most use cases, prefer canvas.DrawSVG() instead.
func (s *SVGDOM) RenderToCanvas(canvas unsafe.Pointer, width, height float32) {
	if s == nil || s.ptr == nil || canvas == nil {
		return
	}
	C.drift_skia_svg_dom_render(s.ptr, C.DriftSkiaCanvas(canvas), C.float(width), C.float(height))
}

// Size returns the intrinsic size of the SVG.
func (s *SVGDOM) Size() (width, height float64) {
	if s == nil || s.ptr == nil {
		return 0, 0
	}
	var w, h C.float
	if C.drift_skia_svg_dom_get_size(s.ptr, &w, &h) == 0 {
		return 0, 0
	}
	return float64(w), float64(h)
}

// SVGDOMRender renders an SVG DOM (by C pointer) to a Skia canvas.
// Used internally by display list playback. The svgPtr must be a valid DriftSkiaSVGDOM handle.
func SVGDOMRender(svgPtr, canvasPtr unsafe.Pointer, width, height float32) {
	if svgPtr == nil || canvasPtr == nil {
		return
	}
	C.drift_skia_svg_dom_render(
		C.DriftSkiaSVGDOM(svgPtr),
		C.DriftSkiaCanvas(canvasPtr),
		C.float(width),
		C.float(height),
	)
}

// SetPreserveAspectRatio sets the preserveAspectRatio attribute on the root SVG element.
// align: 0=xMidYMid(default), 1=xMinYMin, 2=xMidYMin, 3=xMaxYMin, 4=xMinYMid,
//
//	5=xMaxYMid, 6=xMinYMax, 7=xMidYMax, 8=xMaxYMax, 9=none
//
// scale: 0=meet(contain), 1=slice(cover)
func (s *SVGDOM) SetPreserveAspectRatio(align, scale int) {
	if s == nil || s.ptr == nil {
		return
	}
	C.drift_skia_svg_dom_set_preserve_aspect_ratio(s.ptr, C.int(align), C.int(scale))
}

// SetSizeToContainer sets the SVG's root width/height to 100%,
// making it scale to fill the container size set via render calls.
func (s *SVGDOM) SetSizeToContainer() {
	if s == nil || s.ptr == nil {
		return
	}
	C.drift_skia_svg_dom_set_size_to_container(s.ptr)
}

// SVGDOMRenderTinted renders an SVG DOM with an optional tint color.
// If tintColor is 0, renders without tinting.
func SVGDOMRenderTinted(svgPtr, canvasPtr unsafe.Pointer, width, height float32, tintColor uint32) {
	if svgPtr == nil || canvasPtr == nil {
		return
	}
	C.drift_skia_svg_dom_render_tinted(
		C.DriftSkiaSVGDOM(svgPtr), C.DriftSkiaCanvas(canvasPtr),
		C.float(width), C.float(height), C.uint(tintColor))
}

// Skottie wraps a Skia Skottie animation (Lottie player).
type Skottie struct {
	ptr C.DriftSkiaSkottie
}

// NewSkottie creates a Skottie animation from Lottie JSON data.
// Returns nil if parsing fails.
func NewSkottie(data []byte) *Skottie {
	if len(data) == 0 {
		return nil
	}
	ptr := C.drift_skia_skottie_create(
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.int(len(data)),
	)
	if ptr == nil {
		return nil
	}
	return &Skottie{ptr: ptr}
}

// Destroy releases the Skottie animation resources.
func (s *Skottie) Destroy() {
	if s == nil || s.ptr == nil {
		return
	}
	C.drift_skia_skottie_destroy(s.ptr)
	s.ptr = nil
}

// Ptr returns the underlying C handle for use in DrawLottie.
// The returned pointer is stable (not subject to Go GC).
func (s *Skottie) Ptr() unsafe.Pointer {
	if s == nil || s.ptr == nil {
		return nil
	}
	return unsafe.Pointer(s.ptr)
}

// Duration returns the animation duration in seconds.
func (s *Skottie) Duration() float64 {
	if s == nil || s.ptr == nil {
		return 0
	}
	var dur C.float
	if C.drift_skia_skottie_get_duration(s.ptr, &dur) == 0 {
		return 0
	}
	return float64(dur)
}

// Size returns the intrinsic size of the animation.
func (s *Skottie) Size() (width, height float64) {
	if s == nil || s.ptr == nil {
		return 0, 0
	}
	var w, h C.float
	if C.drift_skia_skottie_get_size(s.ptr, &w, &h) == 0 {
		return 0, 0
	}
	return float64(w), float64(h)
}

// Seek sets the animation to the given normalized time (0.0 to 1.0).
func (s *Skottie) Seek(t float64) {
	if s == nil || s.ptr == nil {
		return
	}
	C.drift_skia_skottie_seek(s.ptr, C.float(t))
}

// SkottieSeekAndRender seeks to normalized time t and renders the current frame.
// Used internally by display list playback.
func SkottieSeekAndRender(animPtr, canvasPtr unsafe.Pointer, t, width, height float32) {
	if animPtr == nil || canvasPtr == nil {
		return
	}
	C.drift_skia_skottie_seek(C.DriftSkiaSkottie(animPtr), C.float(t))
	C.drift_skia_skottie_render(
		C.DriftSkiaSkottie(animPtr),
		C.DriftSkiaCanvas(canvasPtr),
		C.float(width),
		C.float(height),
	)
}
