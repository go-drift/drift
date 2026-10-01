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
	// ImageWidth is the image's on-screen width in points on iOS; the height
	// follows the PNG's aspect ratio. Android sizes its splash icon itself.
	ImageWidth      int    `yaml:"image_width"      drift:"default=200"`
	BackgroundColor string `yaml:"background_color" drift:"default=#FFFFFF,hex"`
	FadeDurationMs  int    `yaml:"fade_duration_ms" drift:"default=200"`
	// MaxDurationMs is a safety net for a Preserve never matched by Remove:
	// after this long from launch, Preserve no longer holds the splash. It
	// still waits for the app's first frame, however long App.OnInit runs.
	MaxDurationMs int      `yaml:"max_duration_ms" drift:"default=10000"`
	Android       *Android `yaml:"android,omitempty"`
}

// Android adjusts the Android splash, which is the platform's splash screen
// (Android 12+): an icon centred on background_color, at a size the system
// picks, masked to a circle. Keep the logo inside the centre two thirds of
// the icon image.
type Android struct {
	// Icon replaces image on Android, e.g. a padded variant.
	Icon string `yaml:"icon" drift:"asset"`
	// IconBackgroundColor fills the circle behind the icon. Unset: none.
	IconBackgroundColor string `yaml:"icon_background_color" drift:"hex"`
}

// resolvedConfig is Config parsed for the emitters: assets read, colours
// parsed, the image's on-screen size computed.
type resolvedConfig struct {
	Image           []byte
	ImageSize       size // points / dp
	BackgroundColor driftplugin.Color
	FadeDurationMs  int
	MaxDurationMs   int
	AndroidIcon     []byte
	// AndroidIconBackground is nil when no icon background is configured.
	AndroidIconBackground *driftplugin.Color
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
	out.AndroidIcon = img
	if a := cfg.Android; a != nil {
		if a.Icon != "" {
			if out.AndroidIcon, err = ctx.ResolveAsset(a.Icon); err != nil {
				return resolvedConfig{}, fmt.Errorf("read android.icon %q: %w", a.Icon, err)
			}
			if _, err := png.DecodeConfig(bytes.NewReader(out.AndroidIcon)); err != nil {
				return resolvedConfig{}, fmt.Errorf("android.icon %q must be a PNG: %w", a.Icon, err)
			}
		}
		if a.IconBackgroundColor != "" {
			c, err := driftplugin.ParseColor(a.IconBackgroundColor)
			if err != nil {
				return resolvedConfig{}, fmt.Errorf("android.icon_background_color: %w", err)
			}
			out.AndroidIconBackground = &c
		}
	}
	return out, nil
}
