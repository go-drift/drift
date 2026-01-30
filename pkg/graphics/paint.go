package graphics

import "fmt"

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

// String returns a human-readable representation of the paint style.
func (s PaintStyle) String() string {
	switch s {
	case PaintStyleFill:
		return "fill"
	case PaintStyleStroke:
		return "stroke"
	case PaintStyleFillAndStroke:
		return "fill_and_stroke"
	default:
		return fmt.Sprintf("PaintStyle(%d)", int(s))
	}
}

// StrokeCap describes how stroke endpoints are drawn.
type StrokeCap int

const (
	CapButt   StrokeCap = iota // Flat edge at endpoint (default)
	CapRound                   // Semicircle at endpoint
	CapSquare                  // Square extending past endpoint
)

// String returns a human-readable representation of the stroke cap.
func (c StrokeCap) String() string {
	switch c {
	case CapButt:
		return "butt"
	case CapRound:
		return "round"
	case CapSquare:
		return "square"
	default:
		return fmt.Sprintf("StrokeCap(%d)", int(c))
	}
}

// StrokeJoin describes how stroke corners are drawn.
type StrokeJoin int

const (
	JoinMiter StrokeJoin = iota // Sharp corner (default)
	JoinRound                   // Rounded corner
	JoinBevel                   // Flattened corner
)

// String returns a human-readable representation of the stroke join.
func (j StrokeJoin) String() string {
	switch j {
	case JoinMiter:
		return "miter"
	case JoinRound:
		return "round"
	case JoinBevel:
		return "bevel"
	default:
		return fmt.Sprintf("StrokeJoin(%d)", int(j))
	}
}

// BlendMode controls how source and destination colors are composited.
// Values match Skia's SkBlendMode enum exactly (required for C interop).
type BlendMode int

const (
	BlendModeClear      BlendMode = iota // clear
	BlendModeSrc                         // src
	BlendModeDst                         // dst
	BlendModeSrcOver                     // src_over
	BlendModeDstOver                     // dst_over
	BlendModeSrcIn                       // src_in
	BlendModeDstIn                       // dst_in
	BlendModeSrcOut                      // src_out
	BlendModeDstOut                      // dst_out
	BlendModeSrcATop                     // src_atop
	BlendModeDstATop                     // dst_atop
	BlendModeXor                         // xor
	BlendModePlus                        // plus
	BlendModeModulate                    // modulate
	BlendModeScreen                      // screen
	BlendModeOverlay                     // overlay
	BlendModeDarken                      // darken
	BlendModeLighten                     // lighten
	BlendModeColorDodge                  // color_dodge
	BlendModeColorBurn                   // color_burn
	BlendModeHardLight                   // hard_light
	BlendModeSoftLight                   // soft_light
	BlendModeDifference                  // difference
	BlendModeExclusion                   // exclusion
	BlendModeMultiply                    // multiply
	BlendModeHue                         // hue
	BlendModeSaturation                  // saturation
	BlendModeColor                       // color
	BlendModeLuminosity                  // luminosity
)

var _BlendMode_names = []string{
	"clear", "src", "dst", "src_over", "dst_over",
	"src_in", "dst_in", "src_out", "dst_out",
	"src_atop", "dst_atop", "xor", "plus", "modulate",
	"screen", "overlay", "darken", "lighten",
	"color_dodge", "color_burn", "hard_light", "soft_light",
	"difference", "exclusion", "multiply",
	"hue", "saturation", "color", "luminosity",
}

// String returns a human-readable representation of the blend mode.
func (b BlendMode) String() string {
	if int(b) >= 0 && int(b) < len(_BlendMode_names) {
		return _BlendMode_names[b]
	}
	return fmt.Sprintf("BlendMode(%d)", int(b))
}

// DashPattern defines a stroke dash pattern as alternating on/off lengths.
//
// The pattern repeats along the stroke. For example, Intervals of [10, 5]
// draws 10 pixels on, 5 pixels off, repeating. Intervals of [10, 5, 5, 5]
// draws 10 on, 5 off, 5 on, 5 off, repeating.
type DashPattern struct {
	Intervals []float64 // Alternating on/off lengths; must have even count >= 2, all > 0
	Phase     float64   // Starting offset into the pattern in pixels
}

// Paint describes how to draw a shape on the canvas.
//
// A zero-value Paint draws nothing (BlendModeClear with Alpha 0).
// Use DefaultPaint for a basic opaque white fill.
type Paint struct {
	Color    Color
	Gradient *Gradient // If set, overrides Color for the fill

	// GradientBounds optionally specifies the bounds to use for resolving
	// gradient Alignment coordinates. If nil, the shape's drawing bounds are used.
	// Set this when drawing to an expanded area (e.g., for overflow effects)
	// but wanting the gradient to align to the original widget bounds.
	GradientBounds *Rect

	Style       PaintStyle // Fill, stroke, or both
	StrokeWidth float64    // Width of stroke in pixels

	// Stroke styling (only applies when Style includes stroke)
	StrokeCap  StrokeCap    // How endpoints are drawn; 0 = CapButt
	StrokeJoin StrokeJoin   // How corners are drawn; 0 = JoinMiter
	MiterLimit float64      // Miter join limit before beveling; 0 defaults to 4.0
	Dash       *DashPattern // Dash pattern; nil = solid stroke

	// Compositing
	BlendMode BlendMode // Compositing mode; negative defaults to BlendModeSrcOver
	Alpha     float64   // Overall opacity 0.0-1.0; negative defaults to 1.0

	// Filters (only applied via SaveLayer, not individual draw calls)
	//
	// ColorFilter transforms colors when the layer is composited. Use with
	// SaveLayer to apply effects like tinting, grayscale, or brightness
	// adjustment to grouped content.
	ColorFilter *ColorFilter

	// ImageFilter applies pixel-based effects when the layer is composited.
	// Use with SaveLayer to apply blur, drop shadow, or other effects to
	// grouped content.
	ImageFilter *ImageFilter
}

// DefaultPaint returns a basic opaque white fill paint with standard compositing.
func DefaultPaint() Paint {
	return Paint{
		Color:       ColorWhite,
		Style:       PaintStyleFill,
		StrokeWidth: 1,
		StrokeCap:   CapButt,
		StrokeJoin:  JoinMiter,
		MiterLimit:  4.0,
		BlendMode:   BlendModeSrcOver,
		Alpha:       1.0,
	}
}
