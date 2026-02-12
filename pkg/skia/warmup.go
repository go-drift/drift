//go:build android || darwin || ios

package skia

import (
	"errors"
	"unsafe"
)

// WarmupShaders pre-compiles common GPU shaders by drawing primitives to an
// offscreen surface. This eliminates shader compilation jank during the first
// visible frames. Should be called during context initialization while the
// GL/Metal context is current.
func (c *Context) WarmupShaders(backend string) error {
	if c == nil || c.ptr == nil {
		return errors.New("skia: nil context")
	}

	switch backend {
	case "gl":
		return c.warmupShadersGL()
	case "metal":
		return c.warmupShadersMetal()
	default:
		return errors.New("skia: unknown backend: " + backend)
	}
}

func (c *Context) warmupShadersGL() error {
	// Save current framebuffer binding to restore after warmup.
	// This avoids clobbering state if embedder had a non-default FBO bound.
	// Use defer to ensure restore happens on all exit paths.
	savedFBO := GLGetFramebufferBinding()
	defer GLBindFramebuffer(savedFBO)

	surface, err := c.MakeOffscreenSurfaceGL(16, 16)
	if err != nil {
		return err
	}
	defer surface.Destroy()

	canvas := surface.Canvas()
	if canvas == nil {
		return errors.New("skia: failed to get canvas from offscreen surface")
	}

	warmupPrimitives(canvas)

	// Flush surface work to the context, then submit to GPU synchronously.
	// surface.Flush() ensures draw ops are pushed to the context before
	// FlushAndSubmit forces shader compilation.
	surface.Flush()
	c.FlushAndSubmit(true)

	return nil
}

func (c *Context) warmupShadersMetal() error {
	surface, err := c.MakeOffscreenSurfaceMetal(16, 16)
	if err != nil {
		return err
	}
	defer surface.Destroy()

	canvas := surface.Canvas()
	if canvas == nil {
		return errors.New("skia: failed to get canvas from offscreen surface")
	}

	warmupPrimitives(canvas)

	// Flush surface work to the context, then submit to GPU synchronously.
	surface.Flush()
	c.FlushAndSubmit(true)

	return nil
}

// warmupPrimitives draws all common primitives to trigger shader compilation.
func warmupPrimitives(canvas unsafe.Pointer) {
	const (
		white       = 0xFFFFFFFF
		black       = 0xFF000000
		red         = 0xFFFF0000
		blue        = 0xFF0000FF
		transparent = 0x80000000
	)

	// Style constants
	const (
		styleFill   int32 = 0
		styleStroke int32 = 1
	)

	// Stroke cap/join constants
	const (
		capButt   int32 = 0
		capRound  int32 = 1
		capSquare int32 = 2
		joinMiter int32 = 0
		joinRound int32 = 1
	)

	// Gradient types
	const (
		gradientNone   int32 = 0
		gradientLinear int32 = 1
		gradientRadial int32 = 2
	)

	// BlendMode constants
	const (
		blendSrcOver int32 = 3
	)

	// Gradient colors and positions
	gradientColors := []uint32{red, blue}
	gradientPositions := []float32{0.0, 1.0}

	// 1. Clear - basic fill shader
	CanvasClear(canvas, white)

	// 2. DrawRect (solid) - solid color shader
	CanvasDrawRect(canvas, 0, 0, 8, 8, black, styleFill, 0, true,
		capButt, joinMiter, 4, nil, 0, blendSrcOver, 1.0)

	// 3. DrawRRect - rounded rect shader
	CanvasDrawRRect(canvas, 0, 0, 8, 8, 2, 2, 2, 2, 2, 2, 2, 2,
		black, styleFill, 0, true, capButt, joinMiter, 4, nil, 0, blendSrcOver, 1.0)

	// 4. DrawCircle - circle shader
	CanvasDrawCircle(canvas, 8, 8, 4, black, styleFill, 0, true,
		capButt, joinMiter, 4, nil, 0, blendSrcOver, 1.0)

	// 5. DrawRectGradient (linear) - linear gradient shader
	CanvasDrawRectGradient(canvas, 0, 0, 8, 8, white, styleFill, 0, true,
		capButt, joinMiter, 4, nil, 0, blendSrcOver, 1.0,
		gradientLinear, 0, 0, 8, 8, 0, 0, 0,
		gradientColors, gradientPositions)

	// 6. DrawCircleGradient (radial) - radial gradient shader
	CanvasDrawCircleGradient(canvas, 8, 8, 4, white, styleFill, 0, true,
		capButt, joinMiter, 4, nil, 0, blendSrcOver, 1.0,
		gradientRadial, 0, 0, 0, 0, 8, 8, 4,
		gradientColors, gradientPositions)

	// 7. DrawText (default font) - text/glyph atlas shader
	// Use "Wg" to cover both ascenders and descenders
	CanvasDrawText(canvas, "Wg", "", 0, 10, 10, black, 400, 0)

	// 8. DrawImageRect (low quality) - image sampling shader
	// Create a minimal 4x4 RGBA image
	imagePixels := make([]uint8, 4*4*4)
	for i := range imagePixels {
		imagePixels[i] = 255
	}
	CanvasDrawImageRect(canvas, imagePixels, 4, 4, 16,
		0, 0, 0, 0, 0, 0, 8, 8, 1, 0) // FilterQuality=Low

	// 9. DrawImageRect (high quality) - mipmapped sampling shader
	CanvasDrawImageRect(canvas, imagePixels, 4, 4, 16,
		0, 0, 0, 0, 0, 0, 8, 8, 2, 0) // FilterQuality=High (mipmap)

	// 10. DrawRectShadow - blur mask filter shader
	CanvasDrawRectShadow(canvas, 2, 2, 10, 10, transparent, 2, 1, 1, 0, 0)

	// 11. SaveLayerBlur + draw - image filter blur shader
	CanvasSaveLayerBlur(canvas, 0, 0, 16, 16, 2, 2)
	CanvasDrawRect(canvas, 0, 0, 8, 8, red, styleFill, 0, true,
		capButt, joinMiter, 4, nil, 0, blendSrcOver, 1.0)
	CanvasRestore(canvas)

	// 12. SaveLayerAlpha + draw - alpha blend shader
	CanvasSaveLayerAlpha(canvas, 0, 0, 16, 16, 128)
	CanvasDrawRect(canvas, 0, 0, 8, 8, blue, styleFill, 0, true,
		capButt, joinMiter, 4, nil, 0, blendSrcOver, 1.0)
	CanvasRestore(canvas)

	// 13. DrawRect (stroked + dash) - dash path effect shader
	dashIntervals := []float32{2, 2}
	CanvasDrawRect(canvas, 1, 1, 14, 14, black, styleStroke, 1, true,
		capButt, joinMiter, 4, dashIntervals, 0, blendSrcOver, 1.0)

	// 14. DrawPath (curves) - path shader
	path := NewPath(FillTypeWinding)
	path.MoveTo(0, 8)
	path.QuadTo(8, 0, 16, 8)
	path.CubicTo(12, 12, 4, 12, 0, 8)
	path.Close()
	CanvasDrawPath(canvas, path, black, styleFill, 0, true,
		capButt, joinMiter, 4, nil, 0, blendSrcOver, 1.0)
	path.Destroy()

	// 15. ClipPath + draw - clipped coverage shader
	clipPath := NewPath(FillTypeWinding)
	clipPath.MoveTo(0, 0)
	clipPath.LineTo(16, 0)
	clipPath.LineTo(8, 16)
	clipPath.Close()
	CanvasSave(canvas)
	CanvasClipPath(canvas, clipPath, 0, true)
	CanvasDrawRect(canvas, 0, 0, 16, 16, red, styleFill, 0, true,
		capButt, joinMiter, 4, nil, 0, blendSrcOver, 1.0)
	CanvasRestore(canvas)
	clipPath.Destroy()

	// 16. DrawLine (round cap/join) - stroke cap variant shader
	CanvasDrawLine(canvas, 0, 0, 16, 16, black, 2, true,
		capRound, joinRound, 4, nil, 0, blendSrcOver, 1.0)

	// 17. DrawLine (square cap/miter join) - stroke join variant shader
	CanvasDrawLine(canvas, 0, 16, 16, 0, black, 2, true,
		capSquare, joinMiter, 4, nil, 0, blendSrcOver, 1.0)
}
