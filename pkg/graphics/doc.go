// Package graphics provides low-level graphics primitives for drawing.
//
// This package defines the fundamental types for rendering user interfaces:
// geometry (Size, Offset, Rect), colors, gradients, paints, and the Canvas
// interface for executing drawing operations.
//
// # Geometry
//
// Basic geometric types for layout and positioning:
//
//	size := graphics.Size{Width: 100, Height: 50}
//	offset := graphics.Offset{X: 10, Y: 20}
//	rect := graphics.RectFromLTWH(0, 0, 100, 50)
//
// # Colors
//
// Colors are stored as ARGB (0xAARRGGBB):
//
//	red := graphics.RGB(255, 0, 0)           // Opaque red
//	semiTransparent := graphics.RGBA(0, 0, 0, 128)  // 50% black
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
//	paint := graphics.DefaultPaint()
//	paint.Color = graphics.ColorBlue
//	paint.Style = graphics.PaintStyleStroke
//	paint.StrokeWidth = 2
package graphics
