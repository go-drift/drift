package rendering

import (
	"image"
	"unsafe"
)

// Canvas records or renders drawing commands.
type Canvas interface {
	// Save pushes the current transform and clip state.
	Save()

	// SaveLayerAlpha saves a new layer with the given opacity (0.0 to 1.0).
	// All drawing until the matching Restore() call will be composited with this opacity.
	SaveLayerAlpha(bounds Rect, alpha float64)

	// Restore pops the most recent transform and clip state.
	Restore()

	// Translate moves the origin by the given offset.
	Translate(dx, dy float64)

	// Scale scales the coordinate system by the given factors.
	Scale(sx, sy float64)

	// Rotate rotates the coordinate system by radians.
	Rotate(radians float64)

	// ClipRect restricts future drawing to the given rectangle.
	ClipRect(rect Rect)

	// ClipRRect restricts future drawing to the given rounded rectangle.
	ClipRRect(rrect RRect)

	// Clear fills the entire canvas with the given color.
	Clear(color Color)

	// DrawRect draws a rectangle with the provided paint.
	DrawRect(rect Rect, paint Paint)

	// DrawRRect draws a rounded rectangle with the provided paint.
	DrawRRect(rrect RRect, paint Paint)

	// DrawCircle draws a circle with the provided paint.
	DrawCircle(center Offset, radius float64, paint Paint)

	// DrawLine draws a line segment with the provided paint.
	DrawLine(start, end Offset, paint Paint)

	// DrawText draws a pre-shaped text layout at the given position.
	DrawText(layout *TextLayout, position Offset)

	// DrawImage draws an image with its top-left corner at the given position.
	DrawImage(image image.Image, position Offset)

	// DrawPath draws a path with the provided paint.
	DrawPath(path *Path, paint Paint)

	// DrawRectShadow draws a shadow behind a rectangle.
	DrawRectShadow(rect Rect, shadow BoxShadow)

	// DrawRRectShadow draws a shadow behind a rounded rectangle.
	DrawRRectShadow(rrect RRect, shadow BoxShadow)

	// SaveLayerBlur saves a layer with a backdrop blur effect.
	// Content drawn before this call will be blurred within the bounds.
	// Call Restore() to apply the blur and pop the layer.
	SaveLayerBlur(bounds Rect, sigmaX, sigmaY float64)

	// DrawSVG renders an SVG DOM within the given bounds.
	// svgPtr must be the C handle from SVGDOM.Ptr(), not a Go pointer.
	// The SVG is positioned at bounds.Left/Top and sized to bounds width/height.
	// No-op if bounds has zero or negative dimensions.
	DrawSVG(svgPtr unsafe.Pointer, bounds Rect)

	// DrawSVGTinted renders an SVG DOM within the given bounds with an optional tint color.
	// The tint color replaces all SVG colors while preserving alpha (SrcIn blend mode).
	// If tintColor is 0 (ColorTransparent), renders without tinting.
	// Note: Tinting affects ALL SVG content including gradients and embedded images.
	DrawSVGTinted(svgPtr unsafe.Pointer, bounds Rect, tintColor Color)

	// Size returns the size of the canvas in pixels.
	Size() Size
}
