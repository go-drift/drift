package rendering

// PaintStyle describes how shapes are filled or stroked.
type PaintStyle int

const (
	// PaintStyleFill fills the shape interior.
	PaintStyleFill PaintStyle = iota

	// PaintStyleStroke draws only the outline.
	PaintStyleStroke

	// PaintStyleFillAndStroke fills and then strokes the outline.
	PaintStyleFillAndStroke
)

// Paint describes how to draw a shape on the canvas.
type Paint struct {
	Color       Color
	Gradient    *Gradient
	Style       PaintStyle
	StrokeWidth float64
}

// DefaultPaint returns a basic fill paint.
func DefaultPaint() Paint {
	return Paint{
		Color:       ColorWhite,
		Style:       PaintStyleFill,
		StrokeWidth: 1,
	}
}
