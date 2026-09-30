package plugin

import "fmt"

// Config is the user-facing shape of the drift.yaml `config:` block for the
// splash plugin.
type Config struct {
	Image            string       `yaml:"image"             drift:"required,asset"`
	BackgroundColor  string       `yaml:"background_color"  drift:"default=#FFFFFF,hex"`
	FadeDurationMs   int          `yaml:"fade_duration_ms"  drift:"default=200"`
	Branding         string       `yaml:"branding"`
	BrandingPosition string       `yaml:"branding_position" drift:"default=bottom"`
	Android12        *Android12   `yaml:"android_12,omitempty"`
	Dark             *DarkVariant `yaml:"dark,omitempty"`
}

// Android12 enables the Android 12+ SplashScreen API path. When the block
// is present, the plugin emits a values-v31/styles.xml override and the
// AndroidX core-splashscreen Gradle dependency.
type Android12 struct {
	Icon                string `yaml:"icon"                  drift:"required,asset"`
	IconBackgroundColor string `yaml:"icon_background_color" drift:"default=#FFFFFF,hex"`
	Branding            string `yaml:"branding"`
}

// DarkVariant overrides the splash assets when the device is in dark mode.
// All fields mirror the top-level Config shape.
type DarkVariant struct {
	Image           string     `yaml:"image"            drift:"required,asset"`
	BackgroundColor string     `yaml:"background_color" drift:"default=#000000,hex"`
	Branding        string     `yaml:"branding"`
	Android12       *Android12 `yaml:"android_12,omitempty"`
}

// resolvedConfig is the shape the codegen and op emitters consume.
type resolvedConfig struct {
	Image           string
	BackgroundColor string
	FadeDurationMs  int
	Branding        string
	BrandingPos     brandingPosition

	HasDark bool
	Dark    darkResolved

	HasAndroid12 bool
	Android12    android12Resolved
}

type darkResolved struct {
	Image           string
	BackgroundColor string
	Branding        string
	HasAndroid12    bool
	Android12       android12Resolved
}

type android12Resolved struct {
	Icon                string
	IconBackgroundColor string
	Branding            string
}

type brandingPosition int

const (
	brandingBottom brandingPosition = iota
	brandingBottomLeft
	brandingBottomRight
)

func (p brandingPosition) String() string {
	switch p {
	case brandingBottomLeft:
		return "bottom_left"
	case brandingBottomRight:
		return "bottom_right"
	default:
		return "bottom"
	}
}

// resolve maps the decoded Config (already schema-checked and defaulted by
// the bridge: required fields present, hex colours and assets valid) onto
// the shape the emitters consume.
func resolve(cfg Config) (resolvedConfig, error) {
	out := resolvedConfig{
		Image:           cfg.Image,
		BackgroundColor: cfg.BackgroundColor,
		FadeDurationMs:  cfg.FadeDurationMs,
		Branding:        cfg.Branding,
	}
	pos, err := parseBrandingPosition(cfg.BrandingPosition)
	if err != nil {
		return out, err
	}
	out.BrandingPos = pos

	if cfg.Dark != nil {
		out.HasDark = true
		out.Dark = darkResolved{
			Image:           cfg.Dark.Image,
			BackgroundColor: cfg.Dark.BackgroundColor,
			Branding:        cfg.Dark.Branding,
		}
		if cfg.Dark.Android12 != nil {
			out.Dark.HasAndroid12 = true
			out.Dark.Android12 = resolveAndroid12(*cfg.Dark.Android12)
		}
	}

	if cfg.Android12 != nil {
		out.HasAndroid12 = true
		out.Android12 = resolveAndroid12(*cfg.Android12)
	}

	return out, nil
}

func resolveAndroid12(a Android12) android12Resolved {
	return android12Resolved{
		Icon:                a.Icon,
		IconBackgroundColor: a.IconBackgroundColor,
		Branding:            a.Branding,
	}
}

func parseBrandingPosition(s string) (brandingPosition, error) {
	switch s {
	case "bottom":
		return brandingBottom, nil
	case "bottom_left":
		return brandingBottomLeft, nil
	case "bottom_right":
		return brandingBottomRight, nil
	default:
		return brandingBottom, fmt.Errorf("branding_position %q is not valid (use bottom | bottom_left | bottom_right)", s)
	}
}
