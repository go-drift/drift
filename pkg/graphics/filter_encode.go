package graphics

import "math"

// Filter type markers for serialization
const (
	colorFilterTypeBlend  float32 = 0
	colorFilterTypeMatrix float32 = 1

	imageFilterTypeBlur        float32 = 0
	imageFilterTypeDropShadow  float32 = 1
	imageFilterTypeColorFilter float32 = 2
)

// encodeColorFilter serializes a ColorFilter to a float32 slice for the C bridge.
// Returns nil if cf is nil.
//
// Format:
//
//	Type 0 (Blend): [0, color_as_uint_bits, blend_mode, inner_len, ...inner]
//	Type 1 (Matrix): [1, m0..m19, inner_len, ...inner]
func encodeColorFilter(cf *ColorFilter) []float32 {
	if cf == nil {
		return nil
	}

	var result []float32

	switch cf.Type {
	case ColorFilterBlend:
		result = append(result, colorFilterTypeBlend)
		result = append(result, math.Float32frombits(uint32(cf.Color)))
		// Clamp blend mode to valid Skia range [0, BlendModeLuminosity]
		blendMode := cf.BlendMode
		if blendMode < 0 || blendMode > BlendModeLuminosity {
			blendMode = BlendModeSrcOver
		}
		result = append(result, float32(blendMode))

	case ColorFilterMatrix:
		result = append(result, colorFilterTypeMatrix)
		for i := 0; i < 20; i++ {
			result = append(result, float32(cf.Matrix[i]))
		}

	default:
		return nil
	}

	// Encode inner filter for composition chain
	inner := encodeColorFilter(cf.Inner)
	result = append(result, float32(len(inner)))
	result = append(result, inner...)

	return result
}

// encodeImageFilter serializes an ImageFilter to a float32 slice for the C bridge.
// Returns nil if imf is nil.
//
// Format:
//
//	Type 0 (Blur): [0, sigma_x, sigma_y, tile_mode, input_len, ...input]
//	Type 1 (DropShadow): [1, dx, dy, sigma_x, sigma_y, color_bits, shadow_only, input_len, ...input]
//	Type 2 (ColorFilter): [2, cf_len, ...cf_encoding, input_len, ...input]
func encodeImageFilter(imf *ImageFilter) []float32 {
	if imf == nil {
		return nil
	}

	var result []float32

	switch imf.Type {
	case ImageFilterBlur:
		result = append(result, imageFilterTypeBlur)
		result = append(result, float32(imf.SigmaX))
		result = append(result, float32(imf.SigmaY))
		result = append(result, float32(imf.TileMode))

	case ImageFilterDropShadow:
		result = append(result, imageFilterTypeDropShadow)
		result = append(result, float32(imf.OffsetX))
		result = append(result, float32(imf.OffsetY))
		result = append(result, float32(imf.SigmaX))
		result = append(result, float32(imf.SigmaY))
		result = append(result, math.Float32frombits(uint32(imf.Color)))
		shadowOnly := float32(0)
		if imf.ShadowOnly {
			shadowOnly = 1
		}
		result = append(result, shadowOnly)

	case ImageFilterColorFilter:
		result = append(result, imageFilterTypeColorFilter)
		cfData := encodeColorFilter(imf.ColorFilter)
		result = append(result, float32(len(cfData)))
		result = append(result, cfData...)

	default:
		return nil
	}

	// Encode input filter for chaining
	input := encodeImageFilter(imf.Input)
	result = append(result, float32(len(input)))
	result = append(result, input...)

	return result
}
