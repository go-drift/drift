//go:build android || darwin || ios
// +build android darwin ios

package validation

import (
	"math"
	"testing"

	"github.com/go-drift/drift/pkg/rendering"
)

func TestContrastRatio_BlackOnWhite(t *testing.T) {
	ratio := ContrastRatio(rendering.ColorBlack, rendering.ColorWhite)

	// Black on white should be 21:1
	if math.Abs(ratio-21.0) > 0.1 {
		t.Errorf("Expected ratio ~21, got %.2f", ratio)
	}
}

func TestContrastRatio_WhiteOnBlack(t *testing.T) {
	ratio := ContrastRatio(rendering.ColorWhite, rendering.ColorBlack)

	// Should be the same regardless of order
	if math.Abs(ratio-21.0) > 0.1 {
		t.Errorf("Expected ratio ~21, got %.2f", ratio)
	}
}

func TestContrastRatio_SameColor(t *testing.T) {
	ratio := ContrastRatio(rendering.ColorWhite, rendering.ColorWhite)

	// Same color should be 1:1
	if math.Abs(ratio-1.0) > 0.1 {
		t.Errorf("Expected ratio ~1, got %.2f", ratio)
	}
}

func TestMeetsWCAGAA_NormalText(t *testing.T) {
	// 4.5:1 required for normal text
	if !MeetsWCAGAA(4.5, false) {
		t.Error("4.5:1 should meet AA for normal text")
	}

	if MeetsWCAGAA(4.4, false) {
		t.Error("4.4:1 should not meet AA for normal text")
	}
}

func TestMeetsWCAGAA_LargeText(t *testing.T) {
	// 3:1 required for large text
	if !MeetsWCAGAA(3.0, true) {
		t.Error("3:1 should meet AA for large text")
	}

	if MeetsWCAGAA(2.9, true) {
		t.Error("2.9:1 should not meet AA for large text")
	}
}

func TestMeetsWCAGAAA_NormalText(t *testing.T) {
	// 7:1 required for normal text
	if !MeetsWCAGAAA(7.0, false) {
		t.Error("7:1 should meet AAA for normal text")
	}

	if MeetsWCAGAAA(6.9, false) {
		t.Error("6.9:1 should not meet AAA for normal text")
	}
}

func TestMeetsWCAGAAA_LargeText(t *testing.T) {
	// 4.5:1 required for large text
	if !MeetsWCAGAAA(4.5, true) {
		t.Error("4.5:1 should meet AAA for large text")
	}

	if MeetsWCAGAAA(4.4, true) {
		t.Error("4.4:1 should not meet AAA for large text")
	}
}

func TestCheckContrast(t *testing.T) {
	// Black on white should pass both AA and AAA
	result := CheckContrast(rendering.ColorBlack, rendering.ColorWhite, false)

	if math.Abs(result.Ratio-21.0) > 0.1 {
		t.Errorf("Expected ratio ~21, got %.2f", result.Ratio)
	}

	if !result.MeetsAA {
		t.Error("Black on white should meet AA")
	}

	if !result.MeetsAAA {
		t.Error("Black on white should meet AAA")
	}
}

func TestCheckContrast_LowContrast(t *testing.T) {
	// Light gray on white
	lightGray := rendering.Color(0xFFCCCCCC)
	result := CheckContrast(lightGray, rendering.ColorWhite, false)

	if result.MeetsAA {
		t.Error("Light gray on white should not meet AA")
	}

	if result.MeetsAAA {
		t.Error("Light gray on white should not meet AAA")
	}
}

func TestSuggestForegroundColor_DarkBackground(t *testing.T) {
	darkBg := rendering.Color(0xFF333333)
	suggested := SuggestForegroundColor(darkBg, 4.5)

	// Should suggest white for dark backgrounds
	if suggested != rendering.ColorWhite {
		t.Errorf("Expected white for dark background, got %v", suggested)
	}
}

func TestSuggestForegroundColor_LightBackground(t *testing.T) {
	lightBg := rendering.Color(0xFFEEEEEE)
	suggested := SuggestForegroundColor(lightBg, 4.5)

	// Should suggest black for light backgrounds
	if suggested != rendering.ColorBlack {
		t.Errorf("Expected black for light background, got %v", suggested)
	}
}

func TestIsLargeText(t *testing.T) {
	tests := []struct {
		sizePx   float64
		isBold   bool
		expected bool
	}{
		{24, false, true},   // 18pt normal
		{23, false, false},  // Under 18pt normal
		{18.67, true, true}, // 14pt bold
		{18, true, false},   // Under 14pt bold
		{24, true, true},    // Over threshold, bold
	}

	for _, test := range tests {
		result := IsLargeText(test.sizePx, test.isBold)
		if result != test.expected {
			t.Errorf("IsLargeText(%.2f, %v): expected %v, got %v",
				test.sizePx, test.isBold, test.expected, result)
		}
	}
}

func TestWCAGLevel_String(t *testing.T) {
	if WCAGLevelA.String() != "A" {
		t.Error("WCAGLevelA should be 'A'")
	}
	if WCAGLevelAA.String() != "AA" {
		t.Error("WCAGLevelAA should be 'AA'")
	}
	if WCAGLevelAAA.String() != "AAA" {
		t.Error("WCAGLevelAAA should be 'AAA'")
	}
}

func TestMeetsWCAG_LevelA(t *testing.T) {
	// Level A has no contrast requirements
	if !MeetsWCAG(1.0, WCAGLevelA, TextSizeNormal) {
		t.Error("Any ratio should meet Level A")
	}
}

func TestRelativeLuminance(t *testing.T) {
	// White should have luminance of 1.0
	whiteLum := relativeLuminance(rendering.ColorWhite)
	if math.Abs(whiteLum-1.0) > 0.01 {
		t.Errorf("White luminance: expected ~1.0, got %.4f", whiteLum)
	}

	// Black should have luminance of 0.0
	blackLum := relativeLuminance(rendering.ColorBlack)
	if math.Abs(blackLum-0.0) > 0.01 {
		t.Errorf("Black luminance: expected ~0.0, got %.4f", blackLum)
	}
}
