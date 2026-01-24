package layout

import "github.com/go-drift/drift/pkg/rendering"

// Alignment represents a position within a rectangle.
type Alignment struct {
	X float64
	Y float64
}

// WithinRect returns the offset for a child within the given rect.
func (a Alignment) WithinRect(rect rendering.Rect, childSize rendering.Size) rendering.Offset {
	x := rect.Left + (rect.Width()-childSize.Width)*(a.X+1)/2
	y := rect.Top + (rect.Height()-childSize.Height)*(a.Y+1)/2
	return rendering.Offset{X: x, Y: y}
}

// Common alignment presets.
var (
	// AlignmentTopLeft aligns to the top-left corner.
	AlignmentTopLeft = Alignment{-1, -1}
	// AlignmentTopCenter aligns to the top center.
	AlignmentTopCenter = Alignment{0, -1}
	// AlignmentTopRight aligns to the top-right corner.
	AlignmentTopRight = Alignment{1, -1}
	// AlignmentCenterLeft aligns to the center-left edge.
	AlignmentCenterLeft = Alignment{-1, 0}
	// AlignmentCenter aligns to the center.
	AlignmentCenter = Alignment{0, 0}
	// AlignmentCenterRight aligns to the center-right edge.
	AlignmentCenterRight = Alignment{1, 0}
	// AlignmentBottomLeft aligns to the bottom-left corner.
	AlignmentBottomLeft = Alignment{-1, 1}
	// AlignmentBottomCenter aligns to the bottom center.
	AlignmentBottomCenter = Alignment{0, 1}
	// AlignmentBottomRight aligns to the bottom-right corner.
	AlignmentBottomRight = Alignment{1, 1}
)
