package theme

import (
	"testing"

	"github.com/go-drift/drift/pkg/graphics"
)

func TestColorSchemeFromSeed_LightAndDark(t *testing.T) {
	light, err := ColorSchemeFromSeed(ColorSchemeSeedOptions{
		Seed:       graphics.Color(0xFF6750A4),
		Brightness: BrightnessLight,
	})
	if err != nil {
		t.Fatalf("ColorSchemeFromSeed() light error = %v", err)
	}
	dark, err := ColorSchemeFromSeed(ColorSchemeSeedOptions{
		Seed:       graphics.Color(0xFF6750A4),
		Brightness: BrightnessDark,
	})
	if err != nil {
		t.Fatalf("ColorSchemeFromSeed() dark error = %v", err)
	}

	if light.Brightness != BrightnessLight {
		t.Fatalf("light Brightness = %v, want %v", light.Brightness, BrightnessLight)
	}
	if dark.Brightness != BrightnessDark {
		t.Fatalf("dark Brightness = %v, want %v", dark.Brightness, BrightnessDark)
	}
	if light.Primary == dark.Primary {
		t.Fatal("expected light and dark primary colors to differ")
	}
	if light.Surface == 0 || dark.Surface == 0 {
		t.Fatal("expected generated surface colors")
	}
}

func TestColorSchemeFromSeed_RejectsInvalidContrast(t *testing.T) {
	_, err := ColorSchemeFromSeed(ColorSchemeSeedOptions{
		Seed:     graphics.Color(0xFF6750A4),
		Contrast: 1.1,
	})
	if err == nil {
		t.Fatal("expected invalid contrast error")
	}
}

func TestColorSchemeFromSeed_IgnoresSeedAlpha(t *testing.T) {
	opaque, err := ColorSchemeFromSeed(ColorSchemeSeedOptions{Seed: graphics.Color(0xFF6750A4)})
	if err != nil {
		t.Fatalf("opaque seed error = %v", err)
	}
	translucent, err := ColorSchemeFromSeed(ColorSchemeSeedOptions{Seed: graphics.Color(0x806750A4)})
	if err != nil {
		t.Fatalf("translucent seed error = %v", err)
	}
	if opaque != translucent {
		t.Fatal("expected seed alpha to be ignored")
	}
}

func TestColorSchemeFromSeed_VariantsDiffer(t *testing.T) {
	def, err := ColorSchemeFromSeed(ColorSchemeSeedOptions{Seed: graphics.Color(0xFF009688)})
	if err != nil {
		t.Fatalf("default variant error = %v", err)
	}
	vibrant, err := ColorSchemeFromSeed(ColorSchemeSeedOptions{
		Seed:    graphics.Color(0xFF009688),
		Variant: ColorSchemeSeedVariantVibrant,
	})
	if err != nil {
		t.Fatalf("vibrant variant error = %v", err)
	}
	neutral, err := ColorSchemeFromSeed(ColorSchemeSeedOptions{
		Seed:    graphics.Color(0xFF009688),
		Variant: ColorSchemeSeedVariantNeutral,
	})
	if err != nil {
		t.Fatalf("neutral variant error = %v", err)
	}

	if def.Primary == vibrant.Primary && def.Secondary == vibrant.Secondary {
		t.Fatal("expected vibrant variant to change generated palette")
	}
	if def.Primary == neutral.Primary && def.SurfaceVariant == neutral.SurfaceVariant {
		t.Fatal("expected neutral variant to change generated palette")
	}
}

func TestColorSchemeFromSeed_KeyPairsMeetContrast(t *testing.T) {
	cs := mustSeedScheme(t)
	tests := []struct {
		name string
		bg   graphics.Color
		fg   graphics.Color
	}{
		{name: "primary", bg: cs.Primary, fg: cs.OnPrimary},
		{name: "primary container", bg: cs.PrimaryContainer, fg: cs.OnPrimaryContainer},
		{name: "surface", bg: cs.Surface, fg: cs.OnSurface},
		{name: "error", bg: cs.Error, fg: cs.OnError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := graphics.ContrastRatio(tt.bg, tt.fg); got < 4.5 {
				t.Fatalf("contrast ratio = %.2f, want >= 4.5", got)
			}
		})
	}
}

func mustSeedScheme(t *testing.T) ColorScheme {
	t.Helper()
	cs, err := ColorSchemeFromSeed(ColorSchemeSeedOptions{Seed: graphics.Color(0xFF6750A4)})
	if err != nil {
		t.Fatalf("ColorSchemeFromSeed() error = %v", err)
	}
	return cs
}
