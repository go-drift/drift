package layout

import "github.com/go-drift/drift/pkg/graphics"

// Constraints specify the min/max dimensions a child can occupy.
type Constraints struct {
	MinWidth  float64
	MaxWidth  float64
	MinHeight float64
	MaxHeight float64
}

// Tight returns constraints that force an exact size.
func Tight(size graphics.Size) Constraints {
	return Constraints{
		MinWidth:  size.Width,
		MaxWidth:  size.Width,
		MinHeight: size.Height,
		MaxHeight: size.Height,
	}
}

// Loose returns constraints with zero minimum.
func Loose(size graphics.Size) Constraints {
	return Constraints{
		MinWidth:  0,
		MaxWidth:  size.Width,
		MinHeight: 0,
		MaxHeight: size.Height,
	}
}

// IsTight returns true if both width and height are tight.
func (c Constraints) IsTight() bool {
	return c.HasTightWidth() && c.HasTightHeight()
}

// HasTightWidth returns true if width is fixed.
func (c Constraints) HasTightWidth() bool {
	return c.MinWidth == c.MaxWidth
}

// HasTightHeight returns true if height is fixed.
func (c Constraints) HasTightHeight() bool {
	return c.MinHeight == c.MaxHeight
}

// Constrain clamps a size to fit within the constraints.
func (c Constraints) Constrain(size graphics.Size) graphics.Size {
	return graphics.Size{
		Width:  min(max(size.Width, c.MinWidth), c.MaxWidth),
		Height: min(max(size.Height, c.MinHeight), c.MaxHeight),
	}
}

// Deflate reduces constraints by the provided padding.
func (c Constraints) Deflate(insets EdgeInsets) Constraints {
	horizontal := insets.Horizontal()
	vertical := insets.Vertical()
	return Constraints{
		MinWidth:  max(c.MinWidth-horizontal, 0),
		MaxWidth:  max(c.MaxWidth-horizontal, 0),
		MinHeight: max(c.MinHeight-vertical, 0),
		MaxHeight: max(c.MaxHeight-vertical, 0),
	}
}
