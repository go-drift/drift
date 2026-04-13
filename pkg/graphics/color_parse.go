package graphics

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseHexColor parses #RRGGBB, RRGGBB, #AARRGGBB, or AARRGGBB into a Color.
func ParseHexColor(s string) (Color, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(s), "#")
	switch len(trimmed) {
	case 6:
		value, err := strconv.ParseUint(trimmed, 16, 32)
		if err != nil {
			return 0, fmt.Errorf("parse hex color %q: %w", s, err)
		}
		return Color(0xFF000000 | uint32(value)), nil
	case 8:
		value, err := strconv.ParseUint(trimmed, 16, 32)
		if err != nil {
			return 0, fmt.Errorf("parse hex color %q: %w", s, err)
		}
		return Color(value), nil
	default:
		return 0, fmt.Errorf("parse hex color %q: expected 6 or 8 hex digits", s)
	}
}
