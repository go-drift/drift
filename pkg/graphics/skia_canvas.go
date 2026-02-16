//go:build android || darwin || ios

package graphics

import (
	"image"
	"image/draw"
	"math"
	"unsafe"

	"github.com/go-drift/drift/pkg/skia"
)

// paintParams extracts extended paint parameters with defaults applied.
func paintParams(paint Paint) (cap, join int32, miter float32, dash []float32, dashPhase float32, blend int32, alpha float32) {
	cap = int32(paint.StrokeCap)
	join = int32(paint.StrokeJoin)
	miter = float32(paint.MiterLimit)
	if miter == 0 {
		miter = 4.0
	}
	// Validate dash pattern: must have even count >= 2 with finite positive intervals
	if paint.Dash != nil && len(paint.Dash.Intervals) >= 2 && len(paint.Dash.Intervals)%2 == 0 {
		valid := true
		for _, v := range paint.Dash.Intervals {
			if !(v > 0) { // false for NaN, zero, and negative
				valid = false
				break
			}
		}
		if valid {
			dash = make([]float32, len(paint.Dash.Intervals))
			for i, v := range paint.Dash.Intervals {
				dash[i] = float32(v)
			}
			dashPhase = float32(paint.Dash.Phase)
		}
	}
	// Clamp BlendMode to valid range, invalid values default to SrcOver
	blend = int32(paint.BlendMode)
	if blend < 0 || blend > int32(BlendModeLuminosity) {
		blend = int32(BlendModeSrcOver)
	}
	// Clamp Alpha to [0,1]; invalid values (negative, >1, NaN) default to 1.0
	alpha = float32(paint.Alpha)
	if !(alpha >= 0 && alpha <= 1) {
		alpha = 1.0
	}
	return
}

// buildSkiaPath converts a graphics.Path to a skia.Path.
// Returns nil if path is nil or empty. Caller must call Destroy() on non-nil result.
func buildSkiaPath(path *Path) *skia.Path {
	if path == nil || path.IsEmpty() {
		return nil
	}
	fillType := skia.FillTypeWinding
	if path.FillRule == FillRuleEvenOdd {
		fillType = skia.FillTypeEvenOdd
	}
	skPath := skia.NewPath(fillType)
	for _, cmd := range path.Commands {
		switch cmd.Op {
		case PathOpMoveTo:
			skPath.MoveTo(float32(cmd.Args[0]), float32(cmd.Args[1]))
		case PathOpLineTo:
			skPath.LineTo(float32(cmd.Args[0]), float32(cmd.Args[1]))
		case PathOpQuadTo:
			skPath.QuadTo(float32(cmd.Args[0]), float32(cmd.Args[1]), float32(cmd.Args[2]), float32(cmd.Args[3]))
		case PathOpCubicTo:
			skPath.CubicTo(float32(cmd.Args[0]), float32(cmd.Args[1]), float32(cmd.Args[2]), float32(cmd.Args[3]), float32(cmd.Args[4]), float32(cmd.Args[5]))
		case PathOpClose:
			skPath.Close()
		}
	}
	return skPath
}

// SkiaCanvas implements Canvas using the Skia backend.
type SkiaCanvas struct {
	canvas unsafe.Pointer
	size   Size
}

// NewSkiaCanvas wraps a Skia canvas pointer as a Canvas.
func NewSkiaCanvas(canvas unsafe.Pointer, size Size) *SkiaCanvas {
	return &SkiaCanvas{
		canvas: canvas,
		size:   size,
	}
}

func (c *SkiaCanvas) Save() {
	skia.CanvasSave(c.canvas)
}

func (c *SkiaCanvas) SaveLayerAlpha(bounds Rect, alpha float64) {
	// Clamp alpha to 0.0-1.0 to handle tween overshoot, then convert to 0-255
	if alpha < 0 {
		alpha = 0
	} else if alpha > 1 {
		alpha = 1
	}
	alpha8 := uint8(alpha * 255)
	skia.CanvasSaveLayerAlpha(c.canvas, float32(bounds.Left), float32(bounds.Top), float32(bounds.Right), float32(bounds.Bottom), alpha8)
}

func (c *SkiaCanvas) SaveLayer(bounds Rect, paint *Paint) {
	if paint == nil {
		skia.CanvasSave(c.canvas)
		return
	}
	// Extract blend mode with default
	blend := int32(paint.BlendMode)
	if blend < 0 || blend > int32(BlendModeLuminosity) {
		blend = int32(BlendModeSrcOver)
	}
	// Extract alpha with default
	alpha := float32(paint.Alpha)
	if !(alpha >= 0 && alpha <= 1) {
		alpha = 1.0
	}

	// Check if filters are present
	hasFilters := paint.ColorFilter != nil || paint.ImageFilter != nil
	if !hasFilters {
		skia.CanvasSaveLayer(
			c.canvas,
			float32(bounds.Left), float32(bounds.Top), float32(bounds.Right), float32(bounds.Bottom),
			blend, alpha,
		)
		return
	}

	// Encode filters for C bridge
	cfData := encodeColorFilter(paint.ColorFilter)
	ifData := encodeImageFilter(paint.ImageFilter)

	skia.CanvasSaveLayerFiltered(
		c.canvas,
		float32(bounds.Left), float32(bounds.Top), float32(bounds.Right), float32(bounds.Bottom),
		blend, alpha,
		cfData, ifData,
	)
}

func (c *SkiaCanvas) Restore() {
	skia.CanvasRestore(c.canvas)
}

func (c *SkiaCanvas) Translate(dx, dy float64) {
	skia.CanvasTranslate(c.canvas, float32(dx), float32(dy))
}

func (c *SkiaCanvas) Scale(sx, sy float64) {
	skia.CanvasScale(c.canvas, float32(sx), float32(sy))
}

func (c *SkiaCanvas) Rotate(radians float64) {
	skia.CanvasRotate(c.canvas, float32(radians))
}

func (c *SkiaCanvas) ClipRect(rect Rect) {
	skia.CanvasClipRect(c.canvas, float32(rect.Left), float32(rect.Top), float32(rect.Right), float32(rect.Bottom))
}

func (c *SkiaCanvas) ClipRRect(rrect RRect) {
	skia.CanvasClipRRect(
		c.canvas,
		float32(rrect.Rect.Left),
		float32(rrect.Rect.Top),
		float32(rrect.Rect.Right),
		float32(rrect.Rect.Bottom),
		float32(rrect.TopLeft.X),
		float32(rrect.TopLeft.Y),
		float32(rrect.TopRight.X),
		float32(rrect.TopRight.Y),
		float32(rrect.BottomRight.X),
		float32(rrect.BottomRight.Y),
		float32(rrect.BottomLeft.X),
		float32(rrect.BottomLeft.Y),
	)
}

func (c *SkiaCanvas) ClipPath(path *Path, op ClipOp, antialias bool) {
	skPath := buildSkiaPath(path)
	if skPath == nil {
		// Empty or nil path: create an empty Skia path and let Skia handle it.
		// Intersect with empty = empty clip; Difference with empty = unchanged.
		fillType := skia.FillTypeWinding
		if path != nil && path.FillRule == FillRuleEvenOdd {
			fillType = skia.FillTypeEvenOdd
		}
		skPath = skia.NewPath(fillType)
	}
	defer skPath.Destroy()
	skia.CanvasClipPath(c.canvas, skPath, int32(op), antialias)
}

func (c *SkiaCanvas) Clear(color Color) {
	skia.CanvasClear(c.canvas, uint32(color))
}

func (c *SkiaCanvas) DrawRect(rect Rect, paint Paint) {
	cap, join, miter, dash, dashPhase, blend, alpha := paintParams(paint)
	// Use GradientBounds if set, otherwise use the shape bounds
	gradientBounds := rect
	if paint.GradientBounds != nil {
		gradientBounds = *paint.GradientBounds
	}
	if payload, ok := buildGradientPayload(paint.Gradient, gradientBounds); ok {
		skia.CanvasDrawRectGradient(
			c.canvas,
			float32(rect.Left), float32(rect.Top), float32(rect.Right), float32(rect.Bottom),
			uint32(paint.Color), int32(paint.Style), float32(paint.StrokeWidth), true,
			cap, join, miter, dash, dashPhase, blend, alpha,
			payload.gradientType,
			float32(payload.start.X), float32(payload.start.Y),
			float32(payload.end.X), float32(payload.end.Y),
			float32(payload.center.X), float32(payload.center.Y), float32(payload.radius),
			payload.colors, payload.positions,
		)
		return
	}
	skia.CanvasDrawRect(
		c.canvas,
		float32(rect.Left), float32(rect.Top), float32(rect.Right), float32(rect.Bottom),
		uint32(paint.Color), int32(paint.Style), float32(paint.StrokeWidth), true,
		cap, join, miter, dash, dashPhase, blend, alpha,
	)
}

func (c *SkiaCanvas) DrawRRect(rrect RRect, paint Paint) {
	cap, join, miter, dash, dashPhase, blend, alpha := paintParams(paint)
	// Use GradientBounds if set, otherwise use the shape bounds
	gradientBounds := rrect.Rect
	if paint.GradientBounds != nil {
		gradientBounds = *paint.GradientBounds
	}
	if payload, ok := buildGradientPayload(paint.Gradient, gradientBounds); ok {
		skia.CanvasDrawRRectGradient(
			c.canvas,
			float32(rrect.Rect.Left), float32(rrect.Rect.Top),
			float32(rrect.Rect.Right), float32(rrect.Rect.Bottom),
			float32(rrect.TopLeft.X), float32(rrect.TopLeft.Y),
			float32(rrect.TopRight.X), float32(rrect.TopRight.Y),
			float32(rrect.BottomRight.X), float32(rrect.BottomRight.Y),
			float32(rrect.BottomLeft.X), float32(rrect.BottomLeft.Y),
			uint32(paint.Color), int32(paint.Style), float32(paint.StrokeWidth), true,
			cap, join, miter, dash, dashPhase, blend, alpha,
			payload.gradientType,
			float32(payload.start.X), float32(payload.start.Y),
			float32(payload.end.X), float32(payload.end.Y),
			float32(payload.center.X), float32(payload.center.Y), float32(payload.radius),
			payload.colors, payload.positions,
		)
		return
	}
	skia.CanvasDrawRRect(
		c.canvas,
		float32(rrect.Rect.Left), float32(rrect.Rect.Top),
		float32(rrect.Rect.Right), float32(rrect.Rect.Bottom),
		float32(rrect.TopLeft.X), float32(rrect.TopLeft.Y),
		float32(rrect.TopRight.X), float32(rrect.TopRight.Y),
		float32(rrect.BottomRight.X), float32(rrect.BottomRight.Y),
		float32(rrect.BottomLeft.X), float32(rrect.BottomLeft.Y),
		uint32(paint.Color), int32(paint.Style), float32(paint.StrokeWidth), true,
		cap, join, miter, dash, dashPhase, blend, alpha,
	)
}

func (c *SkiaCanvas) DrawCircle(center Offset, radius float64, paint Paint) {
	cap, join, miter, dash, dashPhase, blend, alpha := paintParams(paint)
	// Compute bounding rect for the circle
	bounds := RectFromLTWH(center.X-radius, center.Y-radius, radius*2, radius*2)
	// Use GradientBounds if set, otherwise use the shape bounds
	gradientBounds := bounds
	if paint.GradientBounds != nil {
		gradientBounds = *paint.GradientBounds
	}
	if payload, ok := buildGradientPayload(paint.Gradient, gradientBounds); ok {
		skia.CanvasDrawCircleGradient(
			c.canvas,
			float32(center.X), float32(center.Y), float32(radius),
			uint32(paint.Color), int32(paint.Style), float32(paint.StrokeWidth), true,
			cap, join, miter, dash, dashPhase, blend, alpha,
			payload.gradientType,
			float32(payload.start.X), float32(payload.start.Y),
			float32(payload.end.X), float32(payload.end.Y),
			float32(payload.center.X), float32(payload.center.Y), float32(payload.radius),
			payload.colors, payload.positions,
		)
		return
	}
	skia.CanvasDrawCircle(
		c.canvas,
		float32(center.X), float32(center.Y), float32(radius),
		uint32(paint.Color), int32(paint.Style), float32(paint.StrokeWidth), true,
		cap, join, miter, dash, dashPhase, blend, alpha,
	)
}

func (c *SkiaCanvas) DrawLine(start, end Offset, paint Paint) {
	cap, join, miter, dash, dashPhase, blend, alpha := paintParams(paint)
	// Compute bounding rect for the line
	bounds := Rect{
		Left:   math.Min(start.X, end.X),
		Top:    math.Min(start.Y, end.Y),
		Right:  math.Max(start.X, end.X),
		Bottom: math.Max(start.Y, end.Y),
	}
	// Use GradientBounds if set, otherwise use the shape bounds
	gradientBounds := bounds
	if paint.GradientBounds != nil {
		gradientBounds = *paint.GradientBounds
	}
	if payload, ok := buildGradientPayload(paint.Gradient, gradientBounds); ok {
		skia.CanvasDrawLineGradient(
			c.canvas,
			float32(start.X), float32(start.Y), float32(end.X), float32(end.Y),
			uint32(paint.Color), float32(paint.StrokeWidth), true,
			cap, join, miter, dash, dashPhase, blend, alpha,
			payload.gradientType,
			float32(payload.start.X), float32(payload.start.Y),
			float32(payload.end.X), float32(payload.end.Y),
			float32(payload.center.X), float32(payload.center.Y), float32(payload.radius),
			payload.colors, payload.positions,
		)
		return
	}
	skia.CanvasDrawLine(
		c.canvas,
		float32(start.X), float32(start.Y), float32(end.X), float32(end.Y),
		uint32(paint.Color), float32(paint.StrokeWidth), true,
		cap, join, miter, dash, dashPhase, blend, alpha,
	)
}

func (c *SkiaCanvas) DrawText(layout *TextLayout, position Offset) {
	if layout == nil {
		return
	}
	if layout.paragraph != nil {
		skia.CanvasSave(c.canvas)
		if position.X != 0 || position.Y != 0 {
			skia.CanvasTranslate(c.canvas, float32(position.X), float32(position.Y))
		}
		layout.paragraph.Paint(c.canvas, 0, 0)
		skia.CanvasRestore(c.canvas)
		return
	}
	fontSize := layout.Style.FontSize
	if fontSize <= 0 {
		fontSize = 16
	}
	fontWeight := int(layout.Style.FontWeight)
	if fontWeight < 100 {
		fontWeight = int(FontWeightNormal)
	}
	lineHeight := layout.LineHeight
	if lineHeight == 0 {
		lineHeight = layout.Ascent + layout.Descent
	}
	// Use text bounds at drawing position for gradient resolution
	textBounds := RectFromLTWH(position.X, position.Y, layout.Size.Width, layout.Size.Height)
	payload, hasGradient := buildGradientPayload(layout.Style.Gradient, textBounds)
	var startX, startY, endX, endY, centerX, centerY float32
	var gradientRadius float32
	if hasGradient {
		startX = float32(payload.start.X)
		startY = float32(payload.start.Y)
		endX = float32(payload.end.X)
		endY = float32(payload.end.Y)
		centerX = float32(payload.center.X)
		centerY = float32(payload.center.Y)
		gradientRadius = float32(payload.radius)
	}
	shadow := layout.Style.Shadow
	for i, line := range layout.Lines {
		if line.Text == "" {
			continue
		}
		baseline := position.Y + layout.Ascent + float64(i)*lineHeight

		// Draw shadow first if present
		if shadow != nil {
			skia.CanvasDrawTextShadow(
				c.canvas,
				line.Text,
				layout.Style.FontFamily,
				float32(position.X+shadow.Offset.X),
				float32(baseline+shadow.Offset.Y),
				float32(fontSize),
				uint32(shadow.Color),
				float32(shadow.Sigma()),
				fontWeight,
				fontStyleBridgeValue(layout.Style.FontStyle),
			)
		}

		// Draw foreground text
		if hasGradient {
			skia.CanvasDrawTextGradient(
				c.canvas,
				line.Text,
				layout.Style.FontFamily,
				float32(position.X),
				float32(baseline),
				float32(fontSize),
				uint32(layout.Style.Color),
				fontWeight,
				fontStyleBridgeValue(layout.Style.FontStyle),
				payload.gradientType,
				startX,
				startY,
				endX,
				endY,
				centerX,
				centerY,
				gradientRadius,
				payload.colors,
				payload.positions,
			)
			continue
		}
		skia.CanvasDrawText(
			c.canvas,
			line.Text,
			layout.Style.FontFamily,
			float32(position.X),
			float32(baseline),
			float32(fontSize),
			uint32(layout.Style.Color),
			fontWeight,
			fontStyleBridgeValue(layout.Style.FontStyle),
		)
	}
}

func (c *SkiaCanvas) DrawImage(img image.Image, position Offset) {
	if img == nil {
		return
	}
	rgba := toRGBA(img)
	if rgba == nil {
		return
	}
	bounds := rgba.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return
	}
	skia.CanvasDrawImageRGBA(
		c.canvas,
		rgba.Pix,
		width,
		height,
		rgba.Stride,
		float32(position.X),
		float32(position.Y),
	)
}

func (c *SkiaCanvas) DrawImageRect(img image.Image, srcRect, dstRect Rect, quality FilterQuality, cacheKey uintptr) {
	if img == nil {
		return
	}
	rgba := toRGBA(img)
	if rgba == nil {
		return
	}
	bounds := rgba.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return
	}

	skia.CanvasDrawImageRect(
		c.canvas, rgba.Pix, w, h, rgba.Stride,
		float32(srcRect.Left), float32(srcRect.Top), float32(srcRect.Right), float32(srcRect.Bottom),
		float32(dstRect.Left), float32(dstRect.Top), float32(dstRect.Right), float32(dstRect.Bottom),
		int(quality), cacheKey,
	)
}

func (c *SkiaCanvas) DrawPath(path *Path, paint Paint) {
	skPath := buildSkiaPath(path)
	if skPath == nil {
		return
	}
	defer skPath.Destroy()

	cap, join, miter, dash, dashPhase, blend, alpha := paintParams(paint)
	bounds := path.Bounds()
	// Expand bounds by half stroke width for stroke/fill-and-stroke styles
	// so gradient alignment matches the rendered stroke extent
	if paint.Style == PaintStyleStroke || paint.Style == PaintStyleFillAndStroke {
		halfStroke := paint.StrokeWidth / 2
		bounds.Left -= halfStroke
		bounds.Top -= halfStroke
		bounds.Right += halfStroke
		bounds.Bottom += halfStroke
	}
	// Use GradientBounds if set, otherwise use the shape bounds
	gradientBounds := bounds
	if paint.GradientBounds != nil {
		gradientBounds = *paint.GradientBounds
	}
	if payload, ok := buildGradientPayload(paint.Gradient, gradientBounds); ok {
		skia.CanvasDrawPathGradient(
			c.canvas, skPath,
			uint32(paint.Color), int32(paint.Style), float32(paint.StrokeWidth), true,
			cap, join, miter, dash, dashPhase, blend, alpha,
			payload.gradientType,
			float32(payload.start.X), float32(payload.start.Y),
			float32(payload.end.X), float32(payload.end.Y),
			float32(payload.center.X), float32(payload.center.Y), float32(payload.radius),
			payload.colors, payload.positions,
		)
		return
	}

	skia.CanvasDrawPath(
		c.canvas, skPath,
		uint32(paint.Color), int32(paint.Style), float32(paint.StrokeWidth), true,
		cap, join, miter, dash, dashPhase, blend, alpha,
	)
}

func (c *SkiaCanvas) DrawRectShadow(rect Rect, shadow BoxShadow) {
	skia.CanvasDrawRectShadow(
		c.canvas,
		float32(rect.Left),
		float32(rect.Top),
		float32(rect.Right),
		float32(rect.Bottom),
		uint32(shadow.Color),
		float32(shadow.Sigma()),
		float32(shadow.Offset.X),
		float32(shadow.Offset.Y),
		float32(shadow.Spread),
		int32(shadow.BlurStyle),
	)
}

func (c *SkiaCanvas) DrawRRectShadow(rrect RRect, shadow BoxShadow) {
	skia.CanvasDrawRRectShadow(
		c.canvas,
		float32(rrect.Rect.Left),
		float32(rrect.Rect.Top),
		float32(rrect.Rect.Right),
		float32(rrect.Rect.Bottom),
		float32(rrect.TopLeft.X),
		float32(rrect.TopLeft.Y),
		float32(rrect.TopRight.X),
		float32(rrect.TopRight.Y),
		float32(rrect.BottomRight.X),
		float32(rrect.BottomRight.Y),
		float32(rrect.BottomLeft.X),
		float32(rrect.BottomLeft.Y),
		uint32(shadow.Color),
		float32(shadow.Sigma()),
		float32(shadow.Offset.X),
		float32(shadow.Offset.Y),
		float32(shadow.Spread),
		int32(shadow.BlurStyle),
	)
}

func (c *SkiaCanvas) SaveLayerBlur(bounds Rect, sigmaX, sigmaY float64) {
	skia.CanvasSaveLayerBlur(
		c.canvas,
		float32(bounds.Left),
		float32(bounds.Top),
		float32(bounds.Right),
		float32(bounds.Bottom),
		float32(sigmaX),
		float32(sigmaY),
	)
}

func (c *SkiaCanvas) DrawSVG(svgPtr unsafe.Pointer, bounds Rect) {
	c.DrawSVGTinted(svgPtr, bounds, 0)
}

func (c *SkiaCanvas) DrawSVGTinted(svgPtr unsafe.Pointer, bounds Rect, tintColor Color) {
	if svgPtr == nil {
		return
	}
	w, h := bounds.Width(), bounds.Height()
	if w <= 0 || h <= 0 {
		return
	}
	skia.CanvasSave(c.canvas)
	skia.CanvasClipRect(c.canvas, float32(bounds.Left), float32(bounds.Top), float32(bounds.Right), float32(bounds.Bottom))
	if bounds.Left != 0 || bounds.Top != 0 {
		skia.CanvasTranslate(c.canvas, float32(bounds.Left), float32(bounds.Top))
	}
	skia.SVGDOMRenderTinted(svgPtr, c.canvas, float32(w), float32(h), uint32(tintColor))
	skia.CanvasRestore(c.canvas)
}

func (c *SkiaCanvas) DrawLottie(animPtr unsafe.Pointer, bounds Rect, t float64) {
	if animPtr == nil {
		return
	}
	w, h := bounds.Width(), bounds.Height()
	if w <= 0 || h <= 0 {
		return
	}
	skia.CanvasSave(c.canvas)
	skia.CanvasClipRect(c.canvas, float32(bounds.Left), float32(bounds.Top), float32(bounds.Right), float32(bounds.Bottom))
	if bounds.Left != 0 || bounds.Top != 0 {
		skia.CanvasTranslate(c.canvas, float32(bounds.Left), float32(bounds.Top))
	}
	skia.SkottieSeekAndRender(animPtr, c.canvas, float32(t), float32(w), float32(h))
	skia.CanvasRestore(c.canvas)
}

func (c *SkiaCanvas) EmbedPlatformView(viewID int64, size Size) {
	// No-op: platform view geometry is resolved by GeometryCanvas in StepFrame
}

func (c *SkiaCanvas) Size() Size {
	return c.size
}

func toRGBA(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba
	}
	bounds := src.Bounds()
	if bounds.Empty() {
		return nil
	}
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, src, bounds.Min, draw.Src)
	return rgba
}
