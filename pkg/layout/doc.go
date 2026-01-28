// Package layout provides layout value types and the render system.
//
// This package defines the types used for positioning and sizing widgets:
// Constraints, EdgeInsets, Alignment, and the render object system.
//
// # Layout Types
//
// Constraints specify the min/max dimensions a child can occupy:
//
//	constraints := layout.Tight(graphics.Size{Width: 100, Height: 50})
//	constraints := layout.Loose(graphics.Size{Width: 200, Height: 200})
//
// EdgeInsets represents padding/margin on four sides:
//
//	padding := layout.EdgeInsetsAll(16)
//	padding := layout.EdgeInsetsSymmetric(horizontal: 8, vertical: 16)
//
// Alignment represents a position within a rectangle:
//
//	alignment := layout.AlignmentCenter
//	alignment := layout.AlignmentTopLeft
//
// # Render Objects
//
// RenderObject handles layout, painting, and hit testing. The render tree
// is responsible for the actual visual representation of widgets.
//
// RenderBox is a RenderObject with box layout, the most common type.
//
// PipelineOwner tracks render objects that need layout or paint updates.
package layout
