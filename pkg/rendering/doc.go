// Package rendering provides low-level graphics primitives for drawing.
//
// This package defines the fundamental types for rendering user interfaces:
// geometry (Size, Offset, Rect), colors, gradients, paints, and the Canvas
// interface for executing drawing operations.
//
// # Geometry
//
// Basic geometric types for layout and positioning:
//
//	size := rendering.Size{Width: 100, Height: 50}
//	offset := rendering.Offset{X: 10, Y: 20}
//	rect := rendering.RectFromLTWH(0, 0, 100, 50)
//
// # Colors
//
// Colors are stored as ARGB (0xAARRGGBB):
//
//	red := rendering.RGB(255, 0, 0)           // Opaque red
//	semiTransparent := rendering.RGBA(0, 0, 0, 128)  // 50% black
//
// # Drawing
//
// The Canvas interface provides drawing operations:
//
//	canvas.DrawRect(rect, paint)
//	canvas.DrawRRect(rrect, paint)
//	canvas.DrawText(layout, offset)
//
// Paint controls how shapes are filled or stroked:
//
//	paint := rendering.DefaultPaint()
//	paint.Color = rendering.ColorBlue
//	paint.Style = rendering.PaintStyleStroke
//	paint.StrokeWidth = 2
package rendering
