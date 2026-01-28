//go:build android || darwin || ios
// +build android darwin ios

// Package validation provides accessibility validation and linting tools.
package validation

import (
	"math"

	"github.com/go-drift/drift/pkg/graphics"
)

// ContrastRatio calculates the contrast ratio between two colors according to WCAG 2.1.
// Returns a value between 1 and 21, where higher values indicate more contrast.
// A ratio of 4.5:1 is required for normal text (AA), 7:1 for enhanced (AAA).
// A ratio of 3:1 is required for large text (AA), 4.5:1 for enhanced (AAA).
func ContrastRatio(fg, bg graphics.Color) float64 {
	l1 := relativeLuminance(fg)
	l2 := relativeLuminance(bg)

	// Ensure l1 is the lighter color
	if l1 < l2 {
		l1, l2 = l2, l1
	}

	return (l1 + 0.05) / (l2 + 0.05)
}

// relativeLuminance calculates the relative luminance of a color.
// See: https://www.w3.org/WAI/GL/wiki/Relative_luminance
func relativeLuminance(c graphics.Color) float64 {
	r := linearize(float64(c.R()) / 255)
	g := linearize(float64(c.G()) / 255)
	b := linearize(float64(c.B()) / 255)

	return 0.2126*r + 0.7152*g + 0.0722*b
}

// linearize converts sRGB component to linear RGB.
func linearize(v float64) float64 {
	if v <= 0.03928 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// WCAGLevel represents a WCAG conformance level.
type WCAGLevel int

const (
	// WCAGLevelA is the minimum level of conformance.
	WCAGLevelA WCAGLevel = iota

	// WCAGLevelAA is the recommended level of conformance.
	WCAGLevelAA

	// WCAGLevelAAA is the highest level of conformance.
	WCAGLevelAAA
)

// String returns the WCAG level name.
func (l WCAGLevel) String() string {
	switch l {
	case WCAGLevelA:
		return "A"
	case WCAGLevelAA:
		return "AA"
	case WCAGLevelAAA:
		return "AAA"
	default:
		return "Unknown"
	}
}

// TextSize indicates whether text is considered "large" for WCAG purposes.
type TextSize int

const (
	// TextSizeNormal is regular text (under 18pt or 14pt bold).
	TextSizeNormal TextSize = iota

	// TextSizeLarge is large text (18pt+ or 14pt+ bold).
	TextSizeLarge
)

// MeetsWCAG checks if a contrast ratio meets the specified WCAG level for the given text size.
func MeetsWCAG(ratio float64, level WCAGLevel, textSize TextSize) bool {
	switch level {
	case WCAGLevelA:
		// Level A has no specific contrast requirements
		return true
	case WCAGLevelAA:
		if textSize == TextSizeLarge {
			return ratio >= 3.0
		}
		return ratio >= 4.5
	case WCAGLevelAAA:
		if textSize == TextSizeLarge {
			return ratio >= 4.5
		}
		return ratio >= 7.0
	default:
		return false
	}
}

// MeetsWCAGAA checks if a contrast ratio meets WCAG AA requirements.
// Pass largeText=true for text that is 18pt+ or 14pt+ bold.
func MeetsWCAGAA(ratio float64, largeText bool) bool {
	textSize := TextSizeNormal
	if largeText {
		textSize = TextSizeLarge
	}
	return MeetsWCAG(ratio, WCAGLevelAA, textSize)
}

// MeetsWCAGAAA checks if a contrast ratio meets WCAG AAA requirements.
// Pass largeText=true for text that is 18pt+ or 14pt+ bold.
func MeetsWCAGAAA(ratio float64, largeText bool) bool {
	textSize := TextSizeNormal
	if largeText {
		textSize = TextSizeLarge
	}
	return MeetsWCAG(ratio, WCAGLevelAAA, textSize)
}

// ContrastResult contains the result of a contrast check.
type ContrastResult struct {
	// Ratio is the calculated contrast ratio.
	Ratio float64

	// MeetsAA indicates whether the ratio meets WCAG AA.
	MeetsAA bool

	// MeetsAAA indicates whether the ratio meets WCAG AAA.
	MeetsAAA bool
}

// CheckContrast checks the contrast ratio between two colors and returns detailed results.
func CheckContrast(fg, bg graphics.Color, largeText bool) ContrastResult {
	ratio := ContrastRatio(fg, bg)
	return ContrastResult{
		Ratio:    ratio,
		MeetsAA:  MeetsWCAGAA(ratio, largeText),
		MeetsAAA: MeetsWCAGAAA(ratio, largeText),
	}
}

// SuggestForegroundColor suggests a foreground color that meets the target contrast
// with the given background color.
func SuggestForegroundColor(bg graphics.Color, targetRatio float64) graphics.Color {
	bgLum := relativeLuminance(bg)

	// Try black
	blackRatio := ContrastRatio(graphics.ColorBlack, bg)
	if blackRatio >= targetRatio {
		return graphics.ColorBlack
	}

	// Try white
	whiteRatio := ContrastRatio(graphics.ColorWhite, bg)
	if whiteRatio >= targetRatio {
		return graphics.ColorWhite
	}

	// If background is dark, use white; if light, use black
	if bgLum < 0.5 {
		return graphics.ColorWhite
	}
	return graphics.ColorBlack
}

// IsLargeText determines if text at the given font size and weight is considered "large"
// for WCAG contrast requirements.
// Large text is 18pt (24px) or larger, or 14pt (18.67px) bold or larger.
func IsLargeText(fontSizePx float64, isBold bool) bool {
	if isBold {
		return fontSizePx >= 18.67 // 14pt
	}
	return fontSizePx >= 24 // 18pt
}
