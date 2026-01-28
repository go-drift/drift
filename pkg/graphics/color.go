package graphics

// maxByte is the maximum value of a byte, used for color normalization.
const maxByte = 255.0

// Color is stored as ARGB (0xAARRGGBB).
type Color uint32

// RGBA constructs a Color from red, green, blue, alpha bytes.
func RGBA(r, g, b, a uint8) Color {
	return Color(uint32(a)<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b))
}

// RGB constructs an opaque Color from red, green, blue bytes.
func RGB(r, g, b uint8) Color {
	return RGBA(r, g, b, 0xFF)
}

// RGBAF returns normalized color components (0.0 to 1.0).
func (c Color) RGBAF() (r, g, b, a float64) {
	return float64(uint8(c>>16)) / maxByte,
		float64(uint8(c>>8)) / maxByte,
		float64(uint8(c)) / maxByte,
		float64(uint8(c>>24)) / maxByte
}

// WithAlpha returns a copy of the color with the given alpha (0-255).
func (c Color) WithAlpha(a uint8) Color {
	return Color(uint32(a)<<24 | uint32(c)&0x00FFFFFF)
}

// Common colors.
var (
	ColorTransparent = Color(0x00000000)
	ColorBlack       = Color(0xFF000000)
	ColorWhite       = Color(0xFFFFFFFF)
	ColorRed         = Color(0xFFFF0000)
	ColorGreen       = Color(0xFF00FF00)
	ColorBlue        = Color(0xFF0000FF)
)
