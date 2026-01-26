//go:build android || darwin || ios

package rendering

import (
	"image"
	"image/draw"
	"unsafe"

	"github.com/go-drift/drift/pkg/skia"
)

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

func (c *SkiaCanvas) Clear(color Color) {
	skia.CanvasClear(c.canvas, uint32(color))
}

func (c *SkiaCanvas) DrawRect(rect Rect, paint Paint) {
	if payload, ok := buildGradientPayload(paint.Gradient); ok {
		skia.CanvasDrawRectGradient(
			c.canvas,
			float32(rect.Left),
			float32(rect.Top),
			float32(rect.Right),
			float32(rect.Bottom),
			uint32(paint.Color),
			int32(paint.Style),
			float32(paint.StrokeWidth),
			true,
			payload.gradientType,
			float32(payload.start.X),
			float32(payload.start.Y),
			float32(payload.end.X),
			float32(payload.end.Y),
			float32(payload.center.X),
			float32(payload.center.Y),
			float32(payload.radius),
			payload.colors,
			payload.positions,
		)
		return
	}
	skia.CanvasDrawRect(
		c.canvas,
		float32(rect.Left),
		float32(rect.Top),
		float32(rect.Right),
		float32(rect.Bottom),
		uint32(paint.Color),
		int32(paint.Style),
		float32(paint.StrokeWidth),
		true,
	)
}

func (c *SkiaCanvas) DrawRRect(rrect RRect, paint Paint) {
	if payload, ok := buildGradientPayload(paint.Gradient); ok {
		skia.CanvasDrawRRectGradient(
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
			uint32(paint.Color),
			int32(paint.Style),
			float32(paint.StrokeWidth),
			true,
			payload.gradientType,
			float32(payload.start.X),
			float32(payload.start.Y),
			float32(payload.end.X),
			float32(payload.end.Y),
			float32(payload.center.X),
			float32(payload.center.Y),
			float32(payload.radius),
			payload.colors,
			payload.positions,
		)
		return
	}
	skia.CanvasDrawRRect(
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
		uint32(paint.Color),
		int32(paint.Style),
		float32(paint.StrokeWidth),
		true,
	)
}

func (c *SkiaCanvas) DrawCircle(center Offset, radius float64, paint Paint) {
	if payload, ok := buildGradientPayload(paint.Gradient); ok {
		skia.CanvasDrawCircleGradient(
			c.canvas,
			float32(center.X),
			float32(center.Y),
			float32(radius),
			uint32(paint.Color),
			int32(paint.Style),
			float32(paint.StrokeWidth),
			true,
			payload.gradientType,
			float32(payload.start.X),
			float32(payload.start.Y),
			float32(payload.end.X),
			float32(payload.end.Y),
			float32(payload.center.X),
			float32(payload.center.Y),
			float32(payload.radius),
			payload.colors,
			payload.positions,
		)
		return
	}
	skia.CanvasDrawCircle(
		c.canvas,
		float32(center.X),
		float32(center.Y),
		float32(radius),
		uint32(paint.Color),
		int32(paint.Style),
		float32(paint.StrokeWidth),
		true,
	)
}

func (c *SkiaCanvas) DrawLine(start, end Offset, paint Paint) {
	if payload, ok := buildGradientPayload(paint.Gradient); ok {
		skia.CanvasDrawLineGradient(
			c.canvas,
			float32(start.X),
			float32(start.Y),
			float32(end.X),
			float32(end.Y),
			uint32(paint.Color),
			float32(paint.StrokeWidth),
			true,
			payload.gradientType,
			float32(payload.start.X),
			float32(payload.start.Y),
			float32(payload.end.X),
			float32(payload.end.Y),
			float32(payload.center.X),
			float32(payload.center.Y),
			float32(payload.radius),
			payload.colors,
			payload.positions,
		)
		return
	}
	skia.CanvasDrawLine(
		c.canvas,
		float32(start.X),
		float32(start.Y),
		float32(end.X),
		float32(end.Y),
		uint32(paint.Color),
		float32(paint.StrokeWidth),
		true,
	)
}

func (c *SkiaCanvas) DrawText(layout *TextLayout, position Offset) {
	if layout == nil {
		return
	}
	fontSize := layout.Style.FontSize
	if fontSize <= 0 {
		fontSize = 16
	}
	lineHeight := layout.LineHeight
	if lineHeight == 0 {
		lineHeight = layout.Ascent + layout.Descent
	}
	payload, hasGradient := buildGradientPayload(layout.Style.Gradient)
	var startX, startY, endX, endY, centerX, centerY float32
	var gradientRadius float32
	if hasGradient {
		startX = float32(payload.start.X + position.X)
		startY = float32(payload.start.Y + position.Y)
		endX = float32(payload.end.X + position.X)
		endY = float32(payload.end.Y + position.Y)
		centerX = float32(payload.center.X + position.X)
		centerY = float32(payload.center.Y + position.Y)
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
				int(layout.Style.FontWeight),
				int(layout.Style.FontStyle),
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
				int(layout.Style.FontWeight),
				int(layout.Style.FontStyle),
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
			int(layout.Style.FontWeight),
			int(layout.Style.FontStyle),
		)
	}
}

func (c *SkiaCanvas) DrawImage(image image.Image, position Offset) {
	if image == nil {
		return
	}
	rgba := toRGBA(image)
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

func (c *SkiaCanvas) DrawPath(path *Path, paint Paint) {
	if path == nil || path.IsEmpty() {
		return
	}
	fillType := skia.FillTypeWinding
	if path.FillRule == FillRuleEvenOdd {
		fillType = skia.FillTypeEvenOdd
	}
	skPath := skia.NewPath(fillType)
	defer skPath.Destroy()

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

	if payload, ok := buildGradientPayload(paint.Gradient); ok {
		skia.CanvasDrawPathGradient(
			c.canvas,
			skPath,
			uint32(paint.Color),
			int32(paint.Style),
			float32(paint.StrokeWidth),
			true,
			payload.gradientType,
			float32(payload.start.X),
			float32(payload.start.Y),
			float32(payload.end.X),
			float32(payload.end.Y),
			float32(payload.center.X),
			float32(payload.center.Y),
			float32(payload.radius),
			payload.colors,
			payload.positions,
		)
		return
	}

	skia.CanvasDrawPath(
		c.canvas,
		skPath,
		uint32(paint.Color),
		int32(paint.Style),
		float32(paint.StrokeWidth),
		true,
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

type gradientPayload struct {
	gradientType int32
	colors       []uint32
	positions    []float32
	start        Offset
	end          Offset
	center       Offset
	radius       float64
}

func buildGradientPayload(gradient *Gradient) (gradientPayload, bool) {
	if gradient == nil || !gradient.IsValid() {
		return gradientPayload{}, false
	}
	stops := gradient.Stops()
	colors := make([]uint32, len(stops))
	positions := make([]float32, len(stops))
	for i, stop := range stops {
		colors[i] = uint32(stop.Color)
		positions[i] = float32(stop.Position)
	}
	payload := gradientPayload{
		gradientType: int32(gradient.Type),
		colors:       colors,
		positions:    positions,
	}
	switch gradient.Type {
	case GradientTypeLinear:
		payload.start = gradient.Linear.Start
		payload.end = gradient.Linear.End
	case GradientTypeRadial:
		payload.center = gradient.Radial.Center
		payload.radius = gradient.Radial.Radius
	default:
		return gradientPayload{}, false
	}
	return payload, true
}
