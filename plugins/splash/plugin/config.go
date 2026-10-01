package plugin

import (
	"bytes"
	"fmt"
	"image/png"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// Config is the user-facing shape of the drift.yaml `config:` block for the
// splash plugin.
type Config struct {
	// Image is a PNG shown centred on the background.
	Image string `yaml:"image" drift:"required,asset"`
	// ImageWidth is the image's on-screen width in points (iOS) and dp
	// (Android); the height follows the PNG's aspect ratio.
	ImageWidth      int    `yaml:"image_width"      drift:"default=200"`
	BackgroundColor string `yaml:"background_color" drift:"default=#FFFFFF,hex"`
	FadeDurationMs  int    `yaml:"fade_duration_ms" drift:"default=200"`
	// MaxDurationMs is a safety net for a Preserve never matched by Remove:
	// after this long from launch, Preserve no longer holds the splash. It
	// still waits for the app's first frame, however long App.OnInit runs.
	MaxDurationMs int        `yaml:"max_duration_ms" drift:"default=10000"`
	Android12     *Android12 `yaml:"android_12,omitempty"`
}

// Android12 enables the Android 12+ SplashScreen API path. When the block
// is present, the plugin emits a values-v31/styles.xml override and the
// AndroidX core-splashscreen Gradle dependency.
type Android12 struct {
	Icon                string `yaml:"icon"                  drift:"required,asset"`
	IconBackgroundColor string `yaml:"icon_background_color" drift:"default=#FFFFFF,hex"`
}

// resolvedConfig is Config parsed for the emitters: assets read, colours
// parsed, the image's on-screen size computed.
type resolvedConfig struct {
	Image           []byte
	ImageSize       size // points / dp
	BackgroundColor driftplugin.Color
	FadeDurationMs  int
	MaxDurationMs   int
	Android12       *android12Resolved // nil when not configured
}

type android12Resolved struct {
	Icon                []byte
	IconBackgroundColor driftplugin.Color
}

type size struct{ Width, Height float64 }

// resolve reads and parses cfg (already schema-checked and defaulted by the
// bridge: required fields present, hex colours and asset paths valid).
func resolve(ctx *driftplugin.BuildCtx, cfg Config) (resolvedConfig, error) {
	if cfg.ImageWidth <= 0 {
		return resolvedConfig{}, fmt.Errorf("image_width must be positive, got %d", cfg.ImageWidth)
	}
	if cfg.FadeDurationMs < 0 {
		return resolvedConfig{}, fmt.Errorf("fade_duration_ms must not be negative, got %d", cfg.FadeDurationMs)
	}
	if cfg.MaxDurationMs <= 0 {
		return resolvedConfig{}, fmt.Errorf("max_duration_ms must be positive, got %d", cfg.MaxDurationMs)
	}
	img, err := ctx.ResolveAsset(cfg.Image)
	if err != nil {
		return resolvedConfig{}, fmt.Errorf("read image %q: %w", cfg.Image, err)
	}
	px, err := png.DecodeConfig(bytes.NewReader(img))
	if err != nil {
		return resolvedConfig{}, fmt.Errorf("image %q must be a PNG: %w", cfg.Image, err)
	}
	if px.Width == 0 || px.Height == 0 {
		return resolvedConfig{}, fmt.Errorf("image %q is empty", cfg.Image)
	}
	bg, err := driftplugin.ParseColor(cfg.BackgroundColor)
	if err != nil {
		return resolvedConfig{}, fmt.Errorf("background_color: %w", err)
	}
	width := float64(cfg.ImageWidth)
	out := resolvedConfig{
		Image:           img,
		ImageSize:       size{Width: width, Height: width * float64(px.Height) / float64(px.Width)},
		BackgroundColor: bg,
		FadeDurationMs:  cfg.FadeDurationMs,
		MaxDurationMs:   cfg.MaxDurationMs,
	}
	if a := cfg.Android12; a != nil {
		icon, err := ctx.ResolveAsset(a.Icon)
		if err != nil {
			return resolvedConfig{}, fmt.Errorf("read android_12.icon %q: %w", a.Icon, err)
		}
		iconBg, err := driftplugin.ParseColor(a.IconBackgroundColor)
		if err != nil {
			return resolvedConfig{}, fmt.Errorf("android_12.icon_background_color: %w", err)
		}
		out.Android12 = &android12Resolved{Icon: icon, IconBackgroundColor: iconBg}
	}
	return out, nil
}
