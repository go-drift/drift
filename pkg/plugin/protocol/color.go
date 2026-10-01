package protocol

import (
	"fmt"
	"strconv"
)

// Color is a colour in Drift's hex format: "#RRGGBB" or "#RRGGBBAA", alpha
// last. It is the format the `hex` config tag accepts and the
// android.color.set op carries. Platform formats are derived from it, never
// written by hand: Android resources put alpha first.
type Color struct{ R, G, B, A uint8 }

// ParseColor parses Drift's hex format. Six digits mean opaque.
func ParseColor(s string) (Color, error) {
	if !hexColorRe.MatchString(s) {
		return Color{}, fmt.Errorf("%q is not a hex colour (want #RRGGBB or #RRGGBBAA)", s)
	}
	v, err := strconv.ParseUint(s[1:], 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("%q is not a hex colour: %w", s, err)
	}
	if len(s) == len("#RRGGBB") {
		v = v<<8 | 0xFF
	}
	return Color{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, nil
}

// String returns the colour in Drift's format: "#RRGGBB" when opaque,
// otherwise "#RRGGBBAA".
func (c Color) String() string {
	if c.A == 0xFF {
		return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02X%02X%02X%02X", c.R, c.G, c.B, c.A)
}

// AndroidHex returns the colour as an Android resource value: "#RRGGBB"
// when opaque, otherwise "#AARRGGBB".
func (c Color) AndroidHex() string {
	if c.A == 0xFF {
		return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02X%02X%02X%02X", c.A, c.R, c.G, c.B)
}
