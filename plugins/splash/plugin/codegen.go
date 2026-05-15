package plugin

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
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
	tmplColors     = mustParseTemplate("templates/plugin_colors.xml.tmpl")
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
// storyboard mirroring the runtime overlay: a background-colour view filling
// the scene, with a single centred image view referencing the `DriftSplash`
// asset shipped in Assets.xcassets.
//
// iOS resolves the asset name to the appearance-matched variant
// (light/dark) automatically, so a dark-mode override is handled by the
// image-set, not by a separate storyboard.
func generateLaunchStoryboard(cfg resolvedConfig) string {
	return renderTemplate(tmplStoryboard, struct {
		BackgroundColorAttrs string
	}{
		BackgroundColorAttrs: hexToStoryboardColor(cfg.BackgroundColor),
	})
}

// generateLayerList returns the bytes of res/drawable/launch_background.xml:
// a layer-list with a coloured background and the splash image centred on
// top. This is the drawable referenced by the scaffold's LaunchTheme;
// replacing the drawable is how the plugin owns the pre-API-31 splash
// visuals without touching the theme XML.
func generateLayerList(backgroundColorResource, imageResource string) string {
	return renderTemplate(tmplLayerList, struct {
		BackgroundColorResource string
		ImageResource           string
	}{
		BackgroundColorResource: backgroundColorResource,
		ImageResource:           imageResource,
	})
}

// generateV31Styles returns res/values-v31/styles.xml declaring a LaunchTheme
// variant that opts into the Android 12+ SplashScreen API. Android resource
// merging picks values-v31/ over values/ on API 31+, so the scaffold's
// LaunchTheme is shadowed on those devices without resource-merge conflicts.
func generateV31Styles(cfg resolvedConfig) string {
	return renderTemplate(tmplV31Styles, struct {
		IconBackgroundColor string
	}{
		IconBackgroundColor: cfg.Android12.IconBackgroundColor,
	})
}

// generateValuesColors returns res/values/plugin_colors.xml declaring the
// drift_splash_background colour. Lives in a plugin-owned values file (not
// the scaffold's colors.xml) to avoid resource-merge collisions.
func generateValuesColors(backgroundColor string) string {
	return renderTemplate(tmplColors, struct {
		BackgroundColor string
	}{
		BackgroundColor: backgroundColor,
	})
}

// generateValuesNightColors mirrors generateValuesColors for the dark
// resource bucket (res/values-night/plugin_colors.xml). When the user
// configures a `dark:` variant, the dark background colour wins on devices
// with the night uiMode.
func generateValuesNightColors(darkBackgroundColor string) string {
	return generateValuesColors(darkBackgroundColor)
}

type nativeConfigView struct {
	BackgroundColor     string
	DarkBackgroundColor string
	FadeDurationMs      int
	BrandingPosition    string
}

func nativeConfigData(cfg resolvedConfig) nativeConfigView {
	var dark string
	if cfg.HasDark {
		dark = cfg.Dark.BackgroundColor
	}
	return nativeConfigView{
		BackgroundColor:     cfg.BackgroundColor,
		DarkBackgroundColor: dark,
		FadeDurationMs:      cfg.FadeDurationMs,
		BrandingPosition:    cfg.BrandingPos.String(),
	}
}

// generateSplashConfigSwift returns the bytes of Runner/Plugins/Splash/
// SplashConfig.swift: a tiny enum holding the resolved configuration as
// static constants. The native splash needs these values before any Go code
// runs (the launch screen is the literal first surface), so config is baked
// into the binary rather than fetched over the channel at startup.
func generateSplashConfigSwift(cfg resolvedConfig) string {
	return renderTemplate(tmplSwiftCfg, nativeConfigData(cfg))
}

// generateSplashConfigKotlin returns the bytes of
// app/src/main/java/com/drift/plugin/splash/SplashConfig.kt: the Kotlin twin
// of SplashConfig.swift.
func generateSplashConfigKotlin(cfg resolvedConfig) string {
	return renderTemplate(tmplKotlinCfg, nativeConfigData(cfg))
}

// hexToStoryboardColor converts "#RRGGBB" to the Xcode storyboard color
// attribute fragment. Only the 6-digit form is exercised here; the 8-digit
// form would need alpha extraction, which the scaffold splash doesn't use.
//
// Boundary validation in resolve() (config.go) gates hex strings through
// hexColorRE, so any value reaching this function is guaranteed well-formed.
// A panic here would indicate the boundary check was bypassed and is a
// programming error worth surfacing loudly, consistent with codegen.go's
// mustParseTemplate / renderTemplate panic-on-internal-failure pattern.
func hexToStoryboardColor(hex string) string {
	if !strings.HasPrefix(hex, "#") || len(hex) < 7 {
		panic(fmt.Sprintf("splash plugin: hexToStoryboardColor got unvalidated input %q; "+
			"boundary check in resolve() should have rejected this", hex))
	}
	r := hexByte(hex[1:3])
	g := hexByte(hex[3:5])
	b := hexByte(hex[5:7])
	return fmt.Sprintf(`red="%.4f" green="%.4f" blue="%.4f" alpha="1" colorSpace="custom" customColorSpace="sRGB"`,
		float64(r)/255.0, float64(g)/255.0, float64(b)/255.0)
}

func hexByte(s string) int {
	var v int
	fmt.Sscanf(s, "%x", &v)
	return v
}
