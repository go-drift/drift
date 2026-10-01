package plugin

// Config is the user-facing shape of the drift.yaml `config:` block for the
// splash plugin.
type Config struct {
	Image           string     `yaml:"image"            drift:"required,asset"`
	BackgroundColor string     `yaml:"background_color" drift:"default=#FFFFFF,hex"`
	FadeDurationMs  int        `yaml:"fade_duration_ms" drift:"default=200"`
	Android12       *Android12 `yaml:"android_12,omitempty"`
}

// Android12 enables the Android 12+ SplashScreen API path. When the block
// is present, the plugin emits a values-v31/styles.xml override and the
// AndroidX core-splashscreen Gradle dependency.
type Android12 struct {
	Icon                string `yaml:"icon"                  drift:"required,asset"`
	IconBackgroundColor string `yaml:"icon_background_color" drift:"default=#FFFFFF,hex"`
}

// resolvedConfig is the shape the codegen and op emitters consume.
type resolvedConfig struct {
	Image           string
	BackgroundColor string
	FadeDurationMs  int

	HasAndroid12 bool
	Android12    android12Resolved
}

type android12Resolved struct {
	Icon                string
	IconBackgroundColor string
}

// resolve maps the decoded Config (already schema-checked and defaulted by
// the bridge: required fields present, hex colours and assets valid) onto
// the shape the emitters consume.
func resolve(cfg Config) (resolvedConfig, error) {
	out := resolvedConfig{
		Image:           cfg.Image,
		BackgroundColor: cfg.BackgroundColor,
		FadeDurationMs:  cfg.FadeDurationMs,
	}
	if cfg.Android12 != nil {
		out.HasAndroid12 = true
		out.Android12 = android12Resolved{
			Icon:                cfg.Android12.Icon,
			IconBackgroundColor: cfg.Android12.IconBackgroundColor,
		}
	}
	return out, nil
}
