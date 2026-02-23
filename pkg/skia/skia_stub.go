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

// Paragraph wraps a Skia text layout paragraph.
type Paragraph struct{}

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
	return nil, errStubNotSupported
}

// NewVulkanContext creates a Skia GPU context using the provided Vulkan handles.
func NewVulkanContext(instance, physDevice, device, queue uintptr, queueFamilyIndex uint32, getInstanceProcAddr uintptr) (*Context, error) {
	return nil, errStubNotSupported
}

// Destroy releases the Skia context.
func (c *Context) Destroy() {}

// FlushAndSubmit flushes pending GPU work and optionally waits for completion.
func (c *Context) FlushAndSubmit(syncCPU bool) {}

// PurgeGpuResources releases all cached GPU resources.
func (c *Context) PurgeGpuResources() {}

// WarmupShaders pre-compiles common GPU shaders.
func (c *Context) WarmupShaders(backend string) error { return errStubNotSupported }

// MakeMetalSurface creates a Skia surface targeting the provided Metal texture.
func (c *Context) MakeMetalSurface(texture unsafe.Pointer, width, height int) (*Surface, error) {
	return nil, errStubNotSupported
}

// MakeVulkanSurface creates a Skia surface wrapping the provided VkImage.
func (c *Context) MakeVulkanSurface(width, height int, vkImage uintptr, vkFormat uint32) (*Surface, error) {
	return nil, errStubNotSupported
}

// MakeOffscreenSurfaceMetal creates a GPU-backed offscreen surface for Metal.
func (c *Context) MakeOffscreenSurfaceMetal(width, height int) (*Surface, error) {
	return nil, errStubNotSupported
}

// MakeOffscreenSurfaceVulkan creates a GPU-backed offscreen surface for Vulkan.
func (c *Context) MakeOffscreenSurfaceVulkan(width, height int) (*Surface, error) {
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

// CanvasClipPath clips the canvas to an arbitrary path.
func CanvasClipPath(canvas unsafe.Pointer, path *Path, clipOp int32, antialias bool) {}

// CanvasSaveLayer saves a layer with blend mode and alpha compositing.
func CanvasSaveLayer(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	blendMode int32, alpha float32,
) {
}

// CanvasSaveLayerFiltered saves a layer with optional color and image filters.
func CanvasSaveLayerFiltered(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	blendMode int32, alpha float32,
	colorFilterData []float32,
	imageFilterData []float32,
) {
}

// CanvasClear clears the canvas with a solid color.
func CanvasClear(canvas unsafe.Pointer, argb uint32) {}

// CanvasDrawRect draws a rectangle.
func CanvasDrawRect(
	canvas unsafe.Pointer,
	left, top, right, bottom float32,
	argb uint32, style int32, strokeWidth float32, aa bool,
	strokeCap, strokeJoin int32, miterLimit float32,
	dashIntervals []float32, dashPhase float32,
	blendMode int32, alpha float32,
) {
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

// CanvasDrawImageRect draws an RGBA image from srcRect to dstRect with sampling quality.
func CanvasDrawImageRect(
	canvas unsafe.Pointer,
	pixels []uint8, width, height, stride int,
	srcL, srcT, srcR, srcB float32,
	dstL, dstT, dstR, dstB float32,
	filterQuality int,
	cacheKey uintptr,
) {
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
	return nil, errStubNotSupported
}

// NewRichParagraph creates a paragraph with multiple styled spans.
func NewRichParagraph(spans []TextSpanData, maxLines int, textAlign int) (*Paragraph, error) {
	return nil, errStubNotSupported
}

// Layout lays out the paragraph within the given width.
func (p *Paragraph) Layout(width float32) {}

// Metrics returns overall paragraph metrics.
func (p *Paragraph) Metrics() (ParagraphMetrics, error) {
	return ParagraphMetrics{}, errStubNotSupported
}

// LineMetrics returns per-line metrics for the paragraph.
func (p *Paragraph) LineMetrics() (ParagraphLineMetrics, error) {
	return ParagraphLineMetrics{}, errStubNotSupported
}

// Paint renders the paragraph to the canvas at the given position.
func (p *Paragraph) Paint(canvas unsafe.Pointer, x, y float32) {}

// Destroy releases the paragraph resources.
func (p *Paragraph) Destroy() {}

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
func CanvasDrawPath(
	canvas unsafe.Pointer,
	path *Path,
	argb uint32, style int32, strokeWidth float32, aa bool,
	strokeCap, strokeJoin int32, miterLimit float32,
	dashIntervals []float32, dashPhase float32,
	blendMode int32, alpha float32,
) {
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

// SVGDOM wraps a Skia SVG DOM for rendering vector graphics.
type SVGDOM struct{}

// NewSVGDOM creates an SVGDOM from SVG data.
func NewSVGDOM(data []byte) *SVGDOM {
	return nil
}

// NewSVGDOMWithBase creates an SVGDOM with a base path for resolving relative resources.
func NewSVGDOMWithBase(data []byte, basePath string) *SVGDOM {
	return nil
}

// Destroy releases the SVG DOM resources.
func (s *SVGDOM) Destroy() {}

// Ptr returns the underlying C handle for use in DrawSVG.
func (s *SVGDOM) Ptr() unsafe.Pointer {
	return nil
}

// RenderToCanvas renders the SVG directly to a Skia canvas.
func (s *SVGDOM) RenderToCanvas(canvas unsafe.Pointer, width, height float32) {}

// Size returns the intrinsic size of the SVG.
func (s *SVGDOM) Size() (width, height float64) {
	return 0, 0
}

// SVGDOMRender renders an SVG DOM (by C pointer) to a Skia canvas.
func SVGDOMRender(svgPtr, canvasPtr unsafe.Pointer, width, height float32) {}

// SVGDOMRenderTinted renders an SVG DOM with an optional tint color.
// If tintColor is 0, renders without tinting.
func SVGDOMRenderTinted(svgPtr, canvasPtr unsafe.Pointer, width, height float32, tintColor uint32) {}

// SetPreserveAspectRatio sets the preserveAspectRatio attribute on the root SVG element.
func (s *SVGDOM) SetPreserveAspectRatio(align, scale int) {}

// SetSizeToContainer sets the SVG's root width/height to 100%,
// making it scale to fill the container size set via render calls.
func (s *SVGDOM) SetSizeToContainer() {}

// Skottie wraps a Skia Skottie animation (Lottie player).
type Skottie struct{}

// NewSkottie creates a Skottie animation from Lottie JSON data.
func NewSkottie(data []byte) *Skottie {
	return nil
}

// Destroy releases the Skottie animation resources.
func (s *Skottie) Destroy() {}

// Ptr returns the underlying C handle for use in DrawLottie.
func (s *Skottie) Ptr() unsafe.Pointer {
	return nil
}

// Duration returns the animation duration in seconds.
func (s *Skottie) Duration() float64 {
	return 0
}

// Size returns the intrinsic size of the animation.
func (s *Skottie) Size() (width, height float64) {
	return 0, 0
}

// Seek sets the animation to the given normalized time (0.0 to 1.0).
func (s *Skottie) Seek(t float64) {}

// SkottieSeekAndRender seeks to normalized time t and renders the current frame.
func SkottieSeekAndRender(animPtr, canvasPtr unsafe.Pointer, t, width, height float32) {}
