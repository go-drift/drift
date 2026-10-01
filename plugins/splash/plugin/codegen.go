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

// Each generated artifact lives as a .tmpl under templates/ so the file
// extension reflects the target language and editors syntax-highlight
// accordingly. The Go-side codegen here is a thin Execute over the
// embedded templates with the resolved-config view object.

var (
	tmplStoryboard = mustParseTemplate("templates/LaunchScreen.storyboard.tmpl")
	tmplLayerList  = mustParseTemplate("templates/launch_background.xml.tmpl")
	tmplV31Styles  = mustParseTemplate("templates/values-v31-styles.xml.tmpl")
	tmplColors     = mustParseTemplate("templates/splash_colors.xml.tmpl")
	tmplSwiftCfg   = mustParseTemplate("templates/SplashConfig.swift.tmpl")
	tmplKotlinCfg  = mustParseTemplate("templates/SplashConfig.kt.tmpl")
)

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

// generateLayerList returns res/drawable/launch_background.xml: the
// background colour with the splash image centred on top at ImageSize. It is
// the drawable the scaffold's LaunchTheme references, so replacing it owns
// the pre-API-31 splash without touching the theme XML.
func generateLayerList(cfg resolvedConfig) string {
	return renderTemplate(tmplLayerList, struct {
		Width, Height string
	}{
		Width:  formatLength(cfg.ImageSize.Width),
		Height: formatLength(cfg.ImageSize.Height),
	})
}

// generateV31Styles returns res/values-v31/styles.xml declaring a LaunchTheme
// variant that opts into the Android 12+ SplashScreen API. Android resource
// merging picks values-v31/ over values/ on API 31+, so the scaffold's
// LaunchTheme is shadowed on those devices without resource-merge conflicts.
func generateV31Styles(a android12Resolved) string {
	return renderTemplate(tmplV31Styles, struct {
		IconBackgroundColor string
	}{
		IconBackgroundColor: a.IconBackgroundColor.AndroidHex(),
	})
}

// generateValuesColors returns res/values/drift_splash_colors.xml declaring the
// drift_splash_background colour. Lives in a splash-owned values file (not
// the scaffold's colors.xml or Drift's plugin_colors.xml) to avoid clashes.
func generateValuesColors(c driftplugin.Color) string {
	return renderTemplate(tmplColors, struct {
		BackgroundColor string
	}{
		BackgroundColor: c.AndroidHex(),
	})
}

// generateSplashConfigSwift returns SplashConfig.swift: the resolved
// configuration as static constants. The native splash needs these values
// before any Go code runs (the launch screen is the literal first surface),
// so config is baked into the binary rather than fetched over the channel.
func generateSplashConfigSwift(cfg resolvedConfig) string {
	c := cfg.BackgroundColor
	return renderTemplate(tmplSwiftCfg, struct {
		Red, Green, Blue, Alpha string
		Width, Height           string
		FadeDurationMs          int
		MaxDurationMs           int
	}{
		Red: component(c.R), Green: component(c.G), Blue: component(c.B), Alpha: component(c.A),
		Width:          formatLength(cfg.ImageSize.Width),
		Height:         formatLength(cfg.ImageSize.Height),
		FadeDurationMs: cfg.FadeDurationMs,
		MaxDurationMs:  cfg.MaxDurationMs,
	})
}

// generateSplashConfigKotlin returns SplashConfig.kt: the Kotlin twin of
// SplashConfig.swift.
func generateSplashConfigKotlin(cfg resolvedConfig) string {
	return renderTemplate(tmplKotlinCfg, struct {
		FadeDurationMs int
		Android12      bool
	}{
		FadeDurationMs: cfg.FadeDurationMs,
		Android12:      cfg.Android12 != nil,
	})
}

// storyboardColor returns the Interface Builder colour attributes for c.
func storyboardColor(c driftplugin.Color) string {
	return fmt.Sprintf(`red="%s" green="%s" blue="%s" alpha="%s" colorSpace="custom" customColorSpace="sRGB"`,
		component(c.R), component(c.G), component(c.B), component(c.A))
}

// component formats an 8-bit colour channel as a 0-1 fraction.
func component(v uint8) string {
	return strconv.FormatFloat(float64(v)/255, 'f', 4, 64)
}

// formatLength formats a length in points / dp for XML and Swift, with at
// most two decimals.
func formatLength(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', -1, 64)
}
