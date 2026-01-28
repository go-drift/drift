package graphics

// ColorFilterType specifies the algorithm used by a ColorFilter.
type ColorFilterType int

const (
	// ColorFilterBlend blends a constant color with the input using a blend mode.
	// Requires Color and BlendMode fields to be set.
	ColorFilterBlend ColorFilterType = iota

	// ColorFilterMatrix applies a 5x4 color transformation matrix.
	// Requires the Matrix field to be set.
	ColorFilterMatrix
)

// ColorFilter transforms colors during graphics.
//
// ColorFilters modify pixel colors as they pass through the rendering pipeline.
// They can be used for effects like tinting, grayscale conversion, brightness
// adjustment, and color correction.
//
// Filters can be chained using the Compose method. When composed, the inner
// filter is applied first, then the outer filter processes the result.
//
// To apply a ColorFilter, set it on a Paint and pass that Paint to SaveLayer.
// The filter is applied when the layer is composited back to the parent.
//
// Filter chains must be acyclic. Creating cycles (e.g., setting Inner to
// point back to the same filter) causes infinite recursion. Use the Compose
// method to safely build chains.
type ColorFilter struct {
	// Type specifies the filter algorithm.
	Type ColorFilterType

	// Color is the constant color for ColorFilterBlend.
	Color Color

	// BlendMode controls how Color is blended for ColorFilterBlend.
	BlendMode BlendMode

	// Matrix is a 5x4 color transformation matrix for ColorFilterMatrix.
	// The matrix is stored in row-major order as [R, G, B, A, translate] for
	// each output channel:
	//
	//   R' = Matrix[0]*R + Matrix[1]*G + Matrix[2]*B + Matrix[3]*A + Matrix[4]
	//   G' = Matrix[5]*R + Matrix[6]*G + Matrix[7]*B + Matrix[8]*A + Matrix[9]
	//   B' = Matrix[10]*R + Matrix[11]*G + Matrix[12]*B + Matrix[13]*A + Matrix[14]
	//   A' = Matrix[15]*R + Matrix[16]*G + Matrix[17]*B + Matrix[18]*A + Matrix[19]
	//
	// Input values are in the range [0, 255]. Translation values (indices 4, 9,
	// 14, 19) are added after multiplication.
	Matrix [20]float64

	// Inner is an optional filter to apply before this one.
	// Used for filter composition chains.
	Inner *ColorFilter
}

// ImageFilterType specifies the algorithm used by an ImageFilter.
type ImageFilterType int

const (
	// ImageFilterBlur applies a Gaussian blur effect.
	// Requires SigmaX and SigmaY fields to be set.
	ImageFilterBlur ImageFilterType = iota

	// ImageFilterDropShadow renders a shadow behind the content.
	// Requires OffsetX, OffsetY, SigmaX, SigmaY, and Color fields.
	ImageFilterDropShadow

	// ImageFilterColorFilter applies a ColorFilter as an image filter.
	// Requires the ColorFilter field to be set.
	ImageFilterColorFilter
)

// TileMode specifies how an image filter handles pixels outside its bounds.
type TileMode int

const (
	// TileModeClamp extends edge pixels outward.
	TileModeClamp TileMode = iota

	// TileModeRepeat tiles the image.
	TileModeRepeat

	// TileModeMirror tiles with alternating mirrored copies.
	TileModeMirror

	// TileModeDecal renders transparent black outside the bounds.
	// This is the default for blur filters.
	TileModeDecal
)

// ImageFilter applies pixel-based effects to rendered content.
//
// ImageFilters operate on the rendered pixels rather than individual colors.
// They enable effects like blur, drop shadows, and other post-processing.
//
// Filters can be chained using the Compose method. When composed, the input
// filter is applied first, then this filter processes the result.
//
// To apply an ImageFilter, set it on a Paint and pass that Paint to SaveLayer.
// The filter processes all drawing operations within the layer when it is
// composited back to the parent.
//
// Filter chains must be acyclic. Creating cycles causes infinite recursion.
// Use the Compose method to safely build chains.
type ImageFilter struct {
	// Type specifies the filter algorithm.
	Type ImageFilterType

	// SigmaX is the horizontal blur radius for ImageFilterBlur and
	// ImageFilterDropShadow. Larger values produce more blur.
	SigmaX float64

	// SigmaY is the vertical blur radius for ImageFilterBlur and
	// ImageFilterDropShadow. Larger values produce more blur.
	SigmaY float64

	// TileMode controls edge handling for ImageFilterBlur.
	TileMode TileMode

	// OffsetX is the horizontal shadow offset for ImageFilterDropShadow.
	// Positive values move the shadow right.
	OffsetX float64

	// OffsetY is the vertical shadow offset for ImageFilterDropShadow.
	// Positive values move the shadow down.
	OffsetY float64

	// Color is the shadow color for ImageFilterDropShadow.
	Color Color

	// ShadowOnly, when true, renders only the shadow without the original
	// content. Used by ImageFilterDropShadow.
	ShadowOnly bool

	// ColorFilter is the filter to apply for ImageFilterColorFilter.
	ColorFilter *ColorFilter

	// Input is an optional filter to apply before this one.
	// Used for filter composition chains.
	Input *ImageFilter
}

// ColorFilterTint creates a color filter that blends a constant color
// with the input using the specified blend mode.
//
// Common blend modes for tinting:
//   - BlendModeSrcIn: replaces color in opaque areas, useful for icons
//   - BlendModeSrcATop: tints while preserving original transparency
//   - BlendModeMultiply: multiplies colors, darkening the result
func ColorFilterTint(color Color, mode BlendMode) ColorFilter {
	return ColorFilter{
		Type:      ColorFilterBlend,
		Color:     color,
		BlendMode: mode,
	}
}

// ColorFilterGrayscale creates a color filter that converts colors to grayscale.
//
// Uses the ITU-R BT.709 luminance formula:
// gray = 0.2126*R + 0.7152*G + 0.0722*B
func ColorFilterGrayscale() ColorFilter {
	return ColorFilter{
		Type: ColorFilterMatrix,
		Matrix: [20]float64{
			0.2126, 0.7152, 0.0722, 0, 0,
			0.2126, 0.7152, 0.0722, 0, 0,
			0.2126, 0.7152, 0.0722, 0, 0,
			0, 0, 0, 1, 0,
		},
	}
}

// ColorFilterDisabled creates a color filter suitable for disabled UI states.
//
// Combines grayscale conversion with 38% opacity, matching Material Design
// guidelines for disabled components.
func ColorFilterDisabled() ColorFilter {
	return ColorFilter{
		Type: ColorFilterMatrix,
		Matrix: [20]float64{
			0.2126, 0.7152, 0.0722, 0, 0,
			0.2126, 0.7152, 0.0722, 0, 0,
			0.2126, 0.7152, 0.0722, 0, 0,
			0, 0, 0, 0.38, 0,
		},
	}
}

// ColorFilterSepia creates a color filter that applies a warm sepia tone.
func ColorFilterSepia() ColorFilter {
	return ColorFilter{
		Type: ColorFilterMatrix,
		Matrix: [20]float64{
			0.393, 0.769, 0.189, 0, 0,
			0.349, 0.686, 0.168, 0, 0,
			0.272, 0.534, 0.131, 0, 0,
			0, 0, 0, 1, 0,
		},
	}
}

// ColorFilterInvert creates a color filter that inverts RGB values.
// Alpha is preserved.
func ColorFilterInvert() ColorFilter {
	return ColorFilter{
		Type: ColorFilterMatrix,
		Matrix: [20]float64{
			-1, 0, 0, 0, 255,
			0, -1, 0, 0, 255,
			0, 0, -1, 0, 255,
			0, 0, 0, 1, 0,
		},
	}
}

// ColorFilterBrightness creates a color filter that scales RGB values.
//
// A factor of 1.0 leaves colors unchanged. Values greater than 1.0 brighten
// the image, while values less than 1.0 darken it. Alpha is preserved.
func ColorFilterBrightness(factor float64) ColorFilter {
	return ColorFilter{
		Type: ColorFilterMatrix,
		Matrix: [20]float64{
			factor, 0, 0, 0, 0,
			0, factor, 0, 0, 0,
			0, 0, factor, 0, 0,
			0, 0, 0, 1, 0,
		},
	}
}

// ColorFilterSaturate creates a color filter that adjusts color saturation.
//
// A factor of 1.0 leaves colors unchanged. A factor of 0.0 produces grayscale.
// Values greater than 1.0 increase saturation. Uses the SVG saturate matrix
// algorithm based on ITU-R BT.709 luminance coefficients.
func ColorFilterSaturate(factor float64) ColorFilter {
	inv := 1 - factor
	r := 0.2126 * inv
	g := 0.7152 * inv
	b := 0.0722 * inv
	return ColorFilter{
		Type: ColorFilterMatrix,
		Matrix: [20]float64{
			r + factor, g, b, 0, 0,
			r, g + factor, b, 0, 0,
			r, g, b + factor, 0, 0,
			0, 0, 0, 1, 0,
		},
	}
}

// Compose returns a new ColorFilter that applies inner first, then this filter.
//
// The returned filter is independent of the inputs; modifying inner after
// calling Compose does not affect the composed filter.
func (cf ColorFilter) Compose(inner ColorFilter) ColorFilter {
	result := cf
	innerCopy := inner
	result.Inner = &innerCopy
	return result
}

// NewBlurFilter creates a Gaussian blur image filter.
//
// SigmaX and sigmaY specify the blur radius in pixels for each axis.
// Larger values produce stronger blur. The filter uses TileModeDecal by
// default, treating pixels outside the bounds as transparent.
func NewBlurFilter(sigmaX, sigmaY float64) ImageFilter {
	return ImageFilter{
		Type:     ImageFilterBlur,
		SigmaX:   sigmaX,
		SigmaY:   sigmaY,
		TileMode: TileModeDecal,
	}
}

// NewBlurFilterUniform creates a Gaussian blur with equal horizontal and
// vertical blur radius.
func NewBlurFilterUniform(sigma float64) ImageFilter {
	return NewBlurFilter(sigma, sigma)
}

// NewDropShadowFilter creates a drop shadow effect that renders both the
// shadow and the original content.
//
// The shadow is offset by (dx, dy) pixels from the original content and
// blurred by sigma pixels. Positive dx moves the shadow right; positive dy
// moves it down.
func NewDropShadowFilter(dx, dy, sigma float64, color Color) ImageFilter {
	return ImageFilter{
		Type:       ImageFilterDropShadow,
		OffsetX:    dx,
		OffsetY:    dy,
		SigmaX:     sigma,
		SigmaY:     sigma,
		Color:      color,
		ShadowOnly: false,
	}
}

// NewDropShadowOnlyFilter creates a drop shadow effect that renders only
// the shadow, without the original content.
//
// This is useful for creating shadow layers that are composited separately
// from the content.
func NewDropShadowOnlyFilter(dx, dy, sigma float64, color Color) ImageFilter {
	return ImageFilter{
		Type:       ImageFilterDropShadow,
		OffsetX:    dx,
		OffsetY:    dy,
		SigmaX:     sigma,
		SigmaY:     sigma,
		Color:      color,
		ShadowOnly: true,
	}
}

// NewImageFilterFromColorFilter wraps a ColorFilter as an ImageFilter.
//
// This allows ColorFilters to be composed with other ImageFilters in a
// filter chain.
func NewImageFilterFromColorFilter(cf ColorFilter) ImageFilter {
	cfCopy := cf
	return ImageFilter{
		Type:        ImageFilterColorFilter,
		ColorFilter: &cfCopy,
	}
}

// WithTileMode returns a copy of the filter with the specified tile mode.
//
// Tile mode only affects ImageFilterBlur. Other filter types ignore it.
func (imf ImageFilter) WithTileMode(mode TileMode) ImageFilter {
	result := imf
	result.TileMode = mode
	return result
}

// Compose returns a new ImageFilter that applies input first, then this filter.
//
// The returned filter is independent of the inputs; modifying input after
// calling Compose does not affect the composed filter.
func (imf ImageFilter) Compose(input ImageFilter) ImageFilter {
	result := imf
	inputCopy := input
	result.Input = &inputCopy
	return result
}

// clone returns a deep copy of the ColorFilter, including the Inner chain.
func (cf *ColorFilter) clone() *ColorFilter {
	if cf == nil {
		return nil
	}
	c := *cf
	c.Inner = cf.Inner.clone()
	return &c
}

// clone returns a deep copy of the ImageFilter, including Input chain and nested ColorFilter.
func (imf *ImageFilter) clone() *ImageFilter {
	if imf == nil {
		return nil
	}
	c := *imf
	c.Input = imf.Input.clone()
	c.ColorFilter = imf.ColorFilter.clone()
	return &c
}
