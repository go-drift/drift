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

var (
	AlignmentTopLeft      = Alignment{-1, -1}
	AlignmentTopCenter    = Alignment{0, -1}
	AlignmentTopRight     = Alignment{1, -1}
	AlignmentCenterLeft   = Alignment{-1, 0}
	AlignmentCenter       = Alignment{0, 0}
	AlignmentCenterRight  = Alignment{1, 0}
	AlignmentBottomLeft   = Alignment{-1, 1}
	AlignmentBottomCenter = Alignment{0, 1}
	AlignmentBottomRight  = Alignment{1, 1}
)
