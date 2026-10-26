package plugin

import (
	"bytes"
	"embed"
	"fmt"
	"math"
	"strconv"
	"text/template"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

//go:embed templates
var templatesFS embed.FS

// The launch storyboard lives as a .tmpl under templates/ so editors
// highlight it as XML. The codegen here is a thin Execute over it with the
// resolved-config view object.
var tmplStoryboard = mustParseTemplate("templates/LaunchScreen.storyboard.tmpl")

func mustParseTemplate(path string) *template.Template {
	data, err := templatesFS.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("splash plugin: embed missing %s: %v", path, err))
	}
	t, err := template.New(path).Parse(string(data))
	if err != nil {
		panic(fmt.Sprintf("splash plugin: parse %s: %v", path, err))
	}
	return t
}

func renderTemplate(t *template.Template, data any) string {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		// Templates are checked at startup; runtime Execute failures are
		// surfaced as panics so build-time codegen errors are loud.
		panic(fmt.Sprintf("splash plugin: render %s: %v", t.Name(), err))
	}
	return buf.String()
}

// generateLaunchStoryboard returns the bytes of a minimal LaunchScreen
// storyboard laid out exactly like the runtime overlay
// (ios/SplashOverlayView.swift): the background colour filling the scene
// and the `DriftSplash` image set centred at ImageSize.
func generateLaunchStoryboard(cfg resolvedConfig) string {
	return renderTemplate(tmplStoryboard, struct {
		BackgroundColorAttrs string
		Width, Height        string
	}{
		BackgroundColorAttrs: storyboardColor(cfg.BackgroundColor),
		Width:                formatLength(cfg.ImageSize.Width),
		Height:               formatLength(cfg.ImageSize.Height),
	})
}

// nativeConfig is what the native halves read as host.config
// (plugin/ios/SplashConfig.swift, plugin/android/SplashConfig.kt): the
// resolved config, rounded as the launch storyboard writes it, so the
// overlay that takes over from the launch screen matches it exactly.
type nativeConfig struct {
	// BackgroundColor is red, green, blue and alpha, each 0 to 1 (iOS).
	BackgroundColor [4]float64 `json:"backgroundColor"`
	// ImageWidth and ImageHeight are the image's size in points (iOS).
	ImageWidth  float64 `json:"imageWidth"`
	ImageHeight float64 `json:"imageHeight"`
	// The durations, in milliseconds, both platforms use.
	FadeDurationMs int `json:"fadeDurationMs"`
	MaxDurationMs  int `json:"maxDurationMs"`
	CeilingMs      int `json:"ceilingMs"`
}

func newNativeConfig(cfg resolvedConfig) nativeConfig {
	c := cfg.BackgroundColor
	return nativeConfig{
		BackgroundColor: [4]float64{fraction(c.R), fraction(c.G), fraction(c.B), fraction(c.A)},
		ImageWidth:      roundLength(cfg.ImageSize.Width),
		ImageHeight:     roundLength(cfg.ImageSize.Height),
		FadeDurationMs:  cfg.FadeDurationMs,
		MaxDurationMs:   cfg.MaxDurationMs,
		CeilingMs:       cfg.CeilingMs,
	}
}

// storyboardColor returns the Interface Builder colour attributes for c.
func storyboardColor(c driftplugin.Color) string {
	return fmt.Sprintf(`red="%s" green="%s" blue="%s" alpha="%s" colorSpace="custom" customColorSpace="sRGB"`,
		component(c.R), component(c.G), component(c.B), component(c.A))
}

// fraction is an 8-bit colour channel as a 0-1 fraction, to four
// decimals.
func fraction(v uint8) float64 {
	return math.Round(float64(v)/255*10000) / 10000
}

// component formats an 8-bit colour channel as a 0-1 fraction for XML.
func component(v uint8) string {
	return strconv.FormatFloat(fraction(v), 'f', 4, 64)
}

// roundLength rounds a length in points / dp to two decimals.
func roundLength(v float64) float64 {
	return math.Round(v*100) / 100
}

// formatLength formats a length in points / dp for XML, with at most two
// decimals.
func formatLength(v float64) string {
	return strconv.FormatFloat(roundLength(v), 'f', -1, 64)
}
