package theme

import (
	"fmt"
	"math"

	"github.com/go-drift/drift/pkg/graphics"
)

// ColorSchemeSeedVariant adjusts how strongly the generated scheme follows the seed.
type ColorSchemeSeedVariant int

const (
	// ColorSchemeSeedVariantDefault generates a balanced Material 3 palette.
	ColorSchemeSeedVariantDefault ColorSchemeSeedVariant = iota
	// ColorSchemeSeedVariantVibrant keeps more of the seed colorfulness.
	ColorSchemeSeedVariantVibrant
	// ColorSchemeSeedVariantNeutral generates a quieter, less colorful palette.
	ColorSchemeSeedVariantNeutral
)

// ColorSchemeSeedOptions controls seed-based ColorScheme generation.
type ColorSchemeSeedOptions struct {
	// Seed is the source color for the generated palette. Alpha is ignored.
	Seed graphics.Color
	// Brightness selects the light or dark scheme.
	Brightness Brightness
	// Contrast adjusts foreground/background separation. Valid range: [-1, 1].
	Contrast float64
	// Variant adjusts palette character. Zero value uses the default variant.
	Variant ColorSchemeSeedVariant
}

type tonalPalette struct {
	hue    float64
	chroma float64
}

// ColorSchemeFromSeed generates a Material 3-style color scheme from a single seed color.
func ColorSchemeFromSeed(opts ColorSchemeSeedOptions) (ColorScheme, error) {
	if opts.Contrast < -1 || opts.Contrast > 1 {
		return ColorScheme{}, fmt.Errorf("contrast %.2f out of range [-1, 1]", opts.Contrast)
	}

	seed := opts.Seed.WithAlpha(1)
	hue, chroma := graphics.HueChroma(seed)
	palettes := makeSeedPalettes(hue, chroma, opts.Variant)
	targetContrast := 4.5 + opts.Contrast*2.5
	if targetContrast < 3 {
		targetContrast = 3
	}

	if opts.Brightness == BrightnessDark {
		return darkSeedColorScheme(palettes, targetContrast), nil
	}
	return lightSeedColorScheme(palettes, targetContrast), nil
}

func lightSeedColorScheme(p seedPalettes, targetContrast float64) ColorScheme {
	primary := p.primary.tone(40)
	primaryContainer := p.primary.tone(90)
	secondary := p.secondary.tone(40)
	secondaryContainer := p.secondary.tone(90)
	tertiary := p.tertiary.tone(40)
	tertiaryContainer := p.tertiary.tone(90)
	errorColor := p.error.tone(40)
	errorContainer := p.error.tone(90)
	surface := p.neutral.tone(98)
	surfaceVariant := p.neutralVariant.tone(90)
	background := p.neutral.tone(98)
	inverseSurface := p.neutral.tone(20)

	return ColorScheme{
		Primary:            primary,
		OnPrimary:          ensureReadable(primary, p.primary, 100, targetContrast),
		PrimaryContainer:   primaryContainer,
		OnPrimaryContainer: ensureReadable(primaryContainer, p.primary, 10, targetContrast),

		Secondary:            secondary,
		OnSecondary:          ensureReadable(secondary, p.secondary, 100, targetContrast),
		SecondaryContainer:   secondaryContainer,
		OnSecondaryContainer: ensureReadable(secondaryContainer, p.secondary, 10, targetContrast),

		Tertiary:            tertiary,
		OnTertiary:          ensureReadable(tertiary, p.tertiary, 100, targetContrast),
		TertiaryContainer:   tertiaryContainer,
		OnTertiaryContainer: ensureReadable(tertiaryContainer, p.tertiary, 10, targetContrast),

		Surface:                 surface,
		OnSurface:               ensureReadable(surface, p.neutral, 10, targetContrast),
		SurfaceVariant:          surfaceVariant,
		OnSurfaceVariant:        ensureReadable(surfaceVariant, p.neutralVariant, 30, targetContrast),
		SurfaceDim:              p.neutral.tone(87),
		SurfaceBright:           p.neutral.tone(98),
		SurfaceContainerLowest:  p.neutral.tone(100),
		SurfaceContainerLow:     p.neutral.tone(96),
		SurfaceContainer:        p.neutral.tone(94),
		SurfaceContainerHigh:    p.neutral.tone(92),
		SurfaceContainerHighest: p.neutral.tone(90),

		Background:   background,
		OnBackground: ensureReadable(background, p.neutral, 10, targetContrast),

		Error:            errorColor,
		OnError:          ensureReadable(errorColor, p.error, 100, targetContrast),
		ErrorContainer:   errorContainer,
		OnErrorContainer: ensureReadable(errorContainer, p.error, 10, targetContrast),

		Outline:        p.neutralVariant.tone(50 + 5*max(contrastFactor(targetContrast), 0)),
		OutlineVariant: p.neutralVariant.tone(80),

		Shadow: graphics.ColorBlack,
		Scrim:  graphics.ColorBlack,

		InverseSurface:   inverseSurface,
		OnInverseSurface: ensureReadable(inverseSurface, p.neutral, 95, targetContrast),
		InversePrimary:   p.primary.tone(80),

		SurfaceTint: primary,
		Brightness:  BrightnessLight,
	}
}

func darkSeedColorScheme(p seedPalettes, targetContrast float64) ColorScheme {
	primary := p.primary.tone(80)
	primaryContainer := p.primary.tone(30)
	secondary := p.secondary.tone(80)
	secondaryContainer := p.secondary.tone(30)
	tertiary := p.tertiary.tone(80)
	tertiaryContainer := p.tertiary.tone(30)
	errorColor := p.error.tone(80)
	errorContainer := p.error.tone(30)
	surface := p.neutral.tone(6)
	surfaceVariant := p.neutralVariant.tone(30)
	background := p.neutral.tone(6)
	inverseSurface := p.neutral.tone(90)

	return ColorScheme{
		Primary:            primary,
		OnPrimary:          ensureReadable(primary, p.primary, 20, targetContrast),
		PrimaryContainer:   primaryContainer,
		OnPrimaryContainer: ensureReadable(primaryContainer, p.primary, 90, targetContrast),

		Secondary:            secondary,
		OnSecondary:          ensureReadable(secondary, p.secondary, 20, targetContrast),
		SecondaryContainer:   secondaryContainer,
		OnSecondaryContainer: ensureReadable(secondaryContainer, p.secondary, 90, targetContrast),

		Tertiary:            tertiary,
		OnTertiary:          ensureReadable(tertiary, p.tertiary, 20, targetContrast),
		TertiaryContainer:   tertiaryContainer,
		OnTertiaryContainer: ensureReadable(tertiaryContainer, p.tertiary, 90, targetContrast),

		Surface:                 surface,
		OnSurface:               ensureReadable(surface, p.neutral, 90, targetContrast),
		SurfaceVariant:          surfaceVariant,
		OnSurfaceVariant:        ensureReadable(surfaceVariant, p.neutralVariant, 80, targetContrast),
		SurfaceDim:              p.neutral.tone(6),
		SurfaceBright:           p.neutral.tone(24),
		SurfaceContainerLowest:  p.neutral.tone(4),
		SurfaceContainerLow:     p.neutral.tone(10),
		SurfaceContainer:        p.neutral.tone(12),
		SurfaceContainerHigh:    p.neutral.tone(17),
		SurfaceContainerHighest: p.neutral.tone(22),

		Background:   background,
		OnBackground: ensureReadable(background, p.neutral, 90, targetContrast),

		Error:            errorColor,
		OnError:          ensureReadable(errorColor, p.error, 20, targetContrast),
		ErrorContainer:   errorContainer,
		OnErrorContainer: ensureReadable(errorContainer, p.error, 90, targetContrast),

		Outline:        p.neutralVariant.tone(60 - 5*max(contrastFactor(targetContrast), 0)),
		OutlineVariant: p.neutralVariant.tone(30),

		Shadow: graphics.ColorBlack,
		Scrim:  graphics.ColorBlack,

		InverseSurface:   inverseSurface,
		OnInverseSurface: ensureReadable(inverseSurface, p.neutral, 20, targetContrast),
		InversePrimary:   p.primary.tone(40),

		SurfaceTint: primary,
		Brightness:  BrightnessDark,
	}
}

type seedPalettes struct {
	primary        tonalPalette
	secondary      tonalPalette
	tertiary       tonalPalette
	neutral        tonalPalette
	neutralVariant tonalPalette
	error          tonalPalette
}

func makeSeedPalettes(hue, chroma float64, variant ColorSchemeSeedVariant) seedPalettes {

	primaryChroma := max(chroma, 36)
	secondaryChroma := min(max(chroma*0.33, 16), 24)
	tertiaryChroma := min(max(chroma*0.5, 20), 32)
	neutralChroma := min(max(chroma*0.12, 4), 10)
	neutralVariantChroma := min(max(chroma*0.18, 8), 14)

	switch variant {
	case ColorSchemeSeedVariantVibrant:
		primaryChroma = max(chroma, 48)
		secondaryChroma = min(max(chroma*0.4, 20), 28)
		tertiaryChroma = min(max(chroma*0.65, 24), 40)
		neutralChroma = min(max(chroma*0.16, 6), 12)
		neutralVariantChroma = min(max(chroma*0.22, 10), 18)
	case ColorSchemeSeedVariantNeutral:
		primaryChroma = min(max(chroma*0.5, 18), 28)
		secondaryChroma = min(max(chroma*0.2, 10), 16)
		tertiaryChroma = min(max(chroma*0.28, 12), 18)
		neutralChroma = min(max(chroma*0.08, 3), 6)
		neutralVariantChroma = min(max(chroma*0.12, 5), 10)
	}

	return seedPalettes{
		primary:        tonalPalette{hue: hue, chroma: primaryChroma},
		secondary:      tonalPalette{hue: hue, chroma: secondaryChroma},
		tertiary:       tonalPalette{hue: math.Mod(hue+60, 360), chroma: tertiaryChroma},
		neutral:        tonalPalette{hue: hue, chroma: neutralChroma},
		neutralVariant: tonalPalette{hue: hue, chroma: neutralVariantChroma},
		error:          tonalPalette{hue: 25, chroma: 84},
	}
}

func (p tonalPalette) tone(tone float64) graphics.Color {
	return graphics.ColorFromHCT(p.hue, p.chroma, tone)
}

// ensureReadable finds a tone on the palette that meets the target contrast ratio
// against the background. It starts from preferredTone and walks toward 0 or 100
// (whichever increases contrast) using binary search.
func ensureReadable(background graphics.Color, p tonalPalette, preferredTone, target float64) graphics.Color {
	preferred := p.tone(preferredTone)
	if graphics.ContrastRatio(background, preferred) >= target {
		return preferred
	}

	bgLum := background.RelativeLuminance()

	// Determine search direction: darken (toward 0) for light backgrounds,
	// lighten (toward 100) for dark backgrounds.
	var lo, hi float64
	if bgLum > 0.5 {
		lo, hi = 0, preferredTone
	} else {
		lo, hi = preferredTone, 100
	}

	// Binary search for the nearest tone that satisfies the contrast target.
	for range 20 {
		mid := (lo + hi) / 2
		if graphics.ContrastRatio(background, p.tone(mid)) >= target {
			if bgLum > 0.5 {
				lo = mid // move toward preferred (lighter)
			} else {
				hi = mid // move toward preferred (darker)
			}
		} else {
			if bgLum > 0.5 {
				hi = mid // need darker
			} else {
				lo = mid // need lighter
			}
		}
	}

	// Pick the endpoint closest to preferred that still passes.
	candidate := lo
	if bgLum <= 0.5 {
		candidate = hi
	}
	return p.tone(candidate)
}

func contrastFactor(target float64) float64 {
	return min(max((target-4.5)/2.5, -1), 1)
}
