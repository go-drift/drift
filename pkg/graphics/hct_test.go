package graphics

import (
	"math"
	"testing"
)

func TestHueChroma_KnownColors(t *testing.T) {
	tests := []struct {
		name       string
		color      Color
		wantHue    float64
		wantChroma float64
		hueTol     float64
		chromaTol  float64
	}{
		{"red", ColorRed, 27.4, 113.4, 1.0, 1.0},
		{"green", ColorGreen, 142.1, 108.4, 1.0, 1.0},
		{"blue", ColorBlue, 282.8, 87.2, 1.0, 1.0},
		{"white", ColorWhite, 209, 3, 10, 1.0}, // slightly chromatic in CAM16
		{"black", ColorBlack, 0, 0, 360, 0.5},  // hue undefined for achromatic
		{"material purple", Color(0xFF6750A4), 299, 48, 2.0, 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hue, chroma := HueChroma(tt.color)
			if math.Abs(hue-tt.wantHue) > tt.hueTol && tt.hueTol < 360 {
				t.Errorf("hue = %.1f, want %.1f (±%.1f)", hue, tt.wantHue, tt.hueTol)
			}
			if math.Abs(chroma-tt.wantChroma) > tt.chromaTol && tt.chromaTol < 200 {
				t.Errorf("chroma = %.1f, want %.1f (±%.1f)", chroma, tt.wantChroma, tt.chromaTol)
			}
		})
	}
}

func TestColorFromHCT_EdgeCases(t *testing.T) {
	black := ColorFromHCT(0, 0, 0)
	if black != ColorBlack {
		t.Errorf("tone=0 should be black, got %#08x", uint32(black))
	}

	white := ColorFromHCT(0, 0, 100)
	if white != ColorWhite {
		t.Errorf("tone=100 should be white, got %#08x", uint32(white))
	}

	// All results must be opaque.
	for tone := 0.0; tone <= 100; tone += 10 {
		c := ColorFromHCT(180, 50, tone)
		if c.A() != 0xFF {
			t.Errorf("ColorFromHCT(180, 50, %.0f) alpha = %d, want 255", tone, c.A())
		}
	}
}

func TestRoundTrip(t *testing.T) {
	colors := []Color{
		ColorRed, ColorGreen, ColorBlue,
		Color(0xFF6750A4), // Material purple
		Color(0xFF009688), // Teal
		Color(0xFFFF5722), // Deep orange
		Color(0xFF808080), // Gray
	}
	for _, c := range colors {
		hue, chroma := HueChroma(c)
		tone := lstarFromArgb(c)
		reconstructed := ColorFromHCT(hue, chroma, tone)

		// Allow ±1 per channel due to rounding in delinearization.
		dr := int(c.R()) - int(reconstructed.R())
		dg := int(c.G()) - int(reconstructed.G())
		db := int(c.B()) - int(reconstructed.B())
		if abs(dr) > 1 || abs(dg) > 1 || abs(db) > 1 {
			t.Errorf("round-trip %#08x: got %#08x (dr=%d dg=%d db=%d)",
				uint32(c), uint32(reconstructed), dr, dg, db)
		}
	}
}

func TestColorFromHCT_HighChromaFallback(t *testing.T) {
	// Request chroma far beyond gamut to exercise the bisectToLimit path.
	c := ColorFromHCT(120, 200, 50)
	if c.A() != 0xFF {
		t.Fatalf("expected opaque, got alpha %d", c.A())
	}
	// Result should have the correct hue and tone, with chroma reduced.
	hue, _ := HueChroma(c)
	tone := lstarFromArgb(c)
	if math.Abs(hue-120) > 2.0 {
		t.Errorf("hue = %.1f, want ~120", hue)
	}
	if math.Abs(tone-50) > 1.0 {
		t.Errorf("tone = %.1f, want ~50", tone)
	}
}

func TestColorFromHCT_ClampsTone(t *testing.T) {
	// Tones outside [0,100] should clamp, not panic.
	below := ColorFromHCT(0, 0, -10)
	above := ColorFromHCT(0, 0, 110)
	if below != ColorBlack {
		t.Errorf("tone=-10 should clamp to black, got %#08x", uint32(below))
	}
	if above != ColorWhite {
		t.Errorf("tone=110 should clamp to white, got %#08x", uint32(above))
	}
}
