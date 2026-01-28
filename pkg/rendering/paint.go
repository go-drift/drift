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

// StrokeCap describes how stroke endpoints are drawn.
type StrokeCap int

const (
	CapButt   StrokeCap = iota // Flat edge at endpoint (default)
	CapRound                   // Semicircle at endpoint
	CapSquare                  // Square extending past endpoint
)

// StrokeJoin describes how stroke corners are drawn.
type StrokeJoin int

const (
	JoinMiter StrokeJoin = iota // Sharp corner (default)
	JoinRound                   // Rounded corner
	JoinBevel                   // Flattened corner
)

// BlendMode controls how source and destination colors are composited.
// Values match Skia's SkBlendMode enum exactly (required for C interop).
type BlendMode int

const (
	BlendModeClear    BlendMode = iota // Clears destination to transparent
	BlendModeSrc                       // Replaces destination with source
	BlendModeDst                       // Keeps destination, ignores source
	BlendModeSrcOver                   // Source over destination (default alpha compositing)
	BlendModeDstOver                   // Destination over source
	BlendModeSrcIn                     // Source where destination is opaque
	BlendModeDstIn                     // Destination where source is opaque
	BlendModeSrcOut                    // Source where destination is transparent
	BlendModeDstOut                    // Destination where source is transparent
	BlendModeSrcATop                   // Source atop destination
	BlendModeDstATop                   // Destination atop source
	BlendModeXor                       // Source or destination, but not both
	BlendModePlus                      // Adds source and destination (clamped)
	BlendModeModulate                  // Multiplies source and destination
	BlendModeScreen                    // Inverse multiply, brightens
	BlendModeOverlay                   // Multiply or screen depending on destination
	BlendModeDarken                    // Keeps darker of source and destination
	BlendModeLighten                   // Keeps lighter of source and destination
	BlendModeColorDodge                // Brightens destination based on source
	BlendModeColorBurn                 // Darkens destination based on source
	BlendModeHardLight                 // Multiply or screen depending on source
	BlendModeSoftLight                 // Soft version of hard light
	BlendModeDifference                // Absolute difference of colors
	BlendModeExclusion                 // Lower contrast difference
	BlendModeMultiply                  // Multiplies colors, always darkens
	BlendModeHue                       // Hue of source, saturation/luminosity of destination
	BlendModeSaturation                // Saturation of source, hue/luminosity of destination
	BlendModeColor                     // Hue/saturation of source, luminosity of destination
	BlendModeLuminosity                // Luminosity of source, hue/saturation of destination
)

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
	Color       Color
	Gradient    *Gradient  // If set, overrides Color for the fill
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
