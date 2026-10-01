package plugin

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// stubAsset writes a 4x2 PNG at projectRoot/<rel>, so a splash image is
// twice as wide as it is tall.
func stubAsset(t *testing.T, projectRoot, rel string) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	writeAsset(t, projectRoot, rel, buf.Bytes())
}

func writeAsset(t *testing.T, projectRoot, rel string, content []byte) {
	t.Helper()
	path := filepath.Join(projectRoot, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
}

// buildYAML runs the plugin the way the bridge does: drift.yaml config
// text is checked against the schema, defaulted, decoded and handed to
// Build, in a ctx rooted at a temp dir seeded with the requested assets.
func buildYAML(t *testing.T, config string, assets ...string) (*driftplugin.BuildCtx, error) {
	t.Helper()
	root := t.TempDir()
	for _, a := range assets {
		stubAsset(t, root, a)
	}
	ctx := driftplugin.NewTestCtxAt(root)
	return ctx, driftplugin.Bind("github.com/go-drift/drift/plugins/splash/plugin", Plugin).Build(ctx, []byte(config))
}

// runBuild is buildYAML for configs that must succeed. Returns the recorded
// ops for assertion.
func runBuild(t *testing.T, config string, assets ...string) []protocol.Op {
	t.Helper()
	ctx, err := buildYAML(t, config, assets...)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx.Err: %v", err)
	}
	return ctx.Ops()
}

// hasOpType returns true if the op list contains at least one op with the
// given JSON discriminator.
func hasOpType(ops []protocol.Op, typ string) bool {
	for _, op := range ops {
		if op.Type() == typ {
			return true
		}
	}
	return false
}

func TestBuild_LightOnly(t *testing.T) {
	ops := runBuild(t, `
image: assets/splash.png
background_color: "#1A2238"
`, "assets/splash.png")

	wantSome := []string{
		"ios.assets.add_image_set",
		"ios.storyboards.replace_launch_screen",
		"ios.source.add",
		"ios.plugin",
		"android.drawable.write",
		"android.resource.write_xml",
		"android.source.add",
		"android.plugin",
	}
	for _, typ := range wantSome {
		if !hasOpType(ops, typ) {
			t.Errorf("missing required op type %q in light-only build", typ)
		}
	}

	if hasOpType(ops, "android.gradle.add_dependency") {
		t.Errorf("light-only build should not add Gradle dependencies")
	}
	// UILaunchStoryboardName=LaunchScreen is in both Info.plist templates.
	if hasOpType(ops, "info_plist.set_string") {
		t.Errorf("splash should not set Info.plist keys the templates already set")
	}
	// The Android 12 controller needs core-splashscreen, so it must not
	// ship without it.
	if kotlinSource(ops, "Android12SplashController.kt") != "" {
		t.Errorf("light-only build should not ship Android12SplashController.kt")
	}
	if strings.Contains(kotlinSource(ops, "SplashConfig.kt"), "Android12SplashController") {
		t.Errorf("light-only SplashConfig.kt must not reference the Android 12 controller")
	}
}

// kotlinSource returns the decoded content of the Kotlin source op whose
// path is rel, or "".
func kotlinSource(ops []protocol.Op, rel string) string {
	for _, op := range ops {
		if src, ok := op.(*protocol.OpAddKotlinSource); ok && src.RelPath == rel {
			b, err := protocol.DecodeContent(src.Content)
			if err != nil {
				return ""
			}
			return string(b)
		}
	}
	return ""
}

func TestBuild_Android12_EmitsGradleAndController(t *testing.T) {
	ops := runBuild(t, `
image: assets/splash.png
background_color: "#1A2238"
android_12:
  icon: assets/splash_icon.png
  icon_background_color: "#FFFFFF"
`, "assets/splash.png", "assets/splash_icon.png")

	if !hasOpType(ops, "android.gradle.add_dependency") {
		t.Errorf("android_12 config should emit gradle dependency op")
	}
	if kotlinSource(ops, "Android12SplashController.kt") == "" {
		t.Errorf("android_12 config should ship Android12SplashController.kt")
	}
	if !strings.Contains(kotlinSource(ops, "SplashConfig.kt"), "Android12SplashController.install(activity)") {
		t.Errorf("android_12 SplashConfig.kt should install the controller:\n%s", kotlinSource(ops, "SplashConfig.kt"))
	}

	// Verify the gradle dep coord pins core-splashscreen 1.0.1 (no floating).
	for _, op := range ops {
		if dep, ok := op.(*protocol.OpAndroidGradleAddDependency); ok {
			if !strings.Contains(dep.Coord, "androidx.core:core-splashscreen:") {
				t.Errorf("gradle dep coord wrong: %s", dep.Coord)
			}
			if !strings.HasSuffix(dep.Coord, ":1.0.1") {
				t.Errorf("gradle dep version must be pinned to 1.0.1; got %s", dep.Coord)
			}
		}
	}
}

// Config the plugin cannot honour on every platform is not accepted:
// unknown keys fail the build rather than doing nothing.
func TestBuild_RejectsRemovedFields(t *testing.T) {
	for _, field := range []string{
		"branding: assets/b.png",
		"branding_position: bottom",
		"dark:\n  image: assets/splash.png",
	} {
		if _, err := buildYAML(t, "image: assets/splash.png\n"+field+"\n", "assets/splash.png"); err == nil {
			t.Errorf("config with %q accepted", field)
		}
	}
}

func TestBuild_RejectsBadHexColor(t *testing.T) {
	_, err := buildYAML(t, `
image: assets/splash.png
background_color: not-a-color
`, "assets/splash.png")
	if err == nil {
		t.Fatal("expected error for invalid background_color")
	}
	if !strings.Contains(err.Error(), "background_color") {
		t.Errorf("error should name the bad field: %v", err)
	}
}

func TestBuild_PluginClasses(t *testing.T) {
	ops := runBuild(t, "image: assets/splash.png\n", "assets/splash.png")

	var sawIOS, sawAndroid bool
	for _, op := range ops {
		switch v := op.(type) {
		case *protocol.OpIOSPlugin:
			if v.Class == "DriftSplashPlugin" {
				sawIOS = true
			}
		case *protocol.OpAndroidPlugin:
			if v.Class == "com.drift.plugin.splash.DriftSplashPlugin" {
				sawAndroid = true
			}
		}
	}
	if !sawIOS {
		t.Errorf("missing iOS DriftSplashPlugin plugin class")
	}
	if !sawAndroid {
		t.Errorf("missing Android DriftSplashPlugin plugin class")
	}
}

func TestBuild_Android12RequiresIcon(t *testing.T) {
	_, err := buildYAML(t, `
image: assets/splash.png
android_12:
  icon_background_color: "#FFFFFF"
`, "assets/splash.png")
	if err == nil || !strings.Contains(err.Error(), "android_12.icon: required field missing") {
		t.Fatalf("err = %v, want missing android_12.icon", err)
	}
}

func launchStoryboard(ops []protocol.Op) string {
	for _, op := range ops {
		if sb, ok := op.(*protocol.OpIOSReplaceLaunchScreen); ok {
			return sb.Content
		}
	}
	return ""
}

func resourceXML(ops []protocol.Op, relPath string) string {
	for _, op := range ops {
		if rx, ok := op.(*protocol.OpAndroidWriteResourceXML); ok && rx.RelPath == relPath {
			return rx.Content
		}
	}
	return ""
}

func iosSource(ops []protocol.Op, rel string) string {
	for _, op := range ops {
		if src, ok := op.(*protocol.OpAddIOSSource); ok && src.RelPath == rel {
			b, err := protocol.DecodeContent(src.Content)
			if err != nil {
				return ""
			}
			return string(b)
		}
	}
	return ""
}

// The launch screen, the iOS overlay and the Android launch drawable all
// show the image at image_width with the PNG's aspect ratio (4x2 here).
func TestBuild_ImageSizedByWidthAndAspect(t *testing.T) {
	ops := runBuild(t, "image: assets/splash.png\nimage_width: 150\n", "assets/splash.png")

	sb := launchStoryboard(ops)
	for _, want := range []string{
		`<constraint firstAttribute="width" constant="150"`,
		`<constraint firstAttribute="height" constant="75"`,
	} {
		if !strings.Contains(sb, want) {
			t.Errorf("storyboard missing %s:\n%s", want, sb)
		}
	}
	if cfg := iosSource(ops, "SplashConfig.swift"); !strings.Contains(cfg, "CGSize(width: 150, height: 75)") {
		t.Errorf("SplashConfig.swift size wrong:\n%s", cfg)
	}
	lb := resourceXML(ops, "drawable/launch_background.xml")
	if !strings.Contains(lb, `android:width="150dp"`) || !strings.Contains(lb, `android:height="75dp"`) {
		t.Errorf("launch_background.xml size wrong:\n%s", lb)
	}
}

func TestBuild_ImageWidthDefault(t *testing.T) {
	ops := runBuild(t, "image: assets/splash.png\n", "assets/splash.png")
	if !strings.Contains(launchStoryboard(ops), `firstAttribute="width" constant="200"`) {
		t.Errorf("default image_width should be 200")
	}
}

// Colours are alpha last in drift.yaml, alpha first in Android resources,
// and fractional components on iOS.
func TestBuild_ColoursPerPlatform(t *testing.T) {
	ops := runBuild(t, `
image: assets/splash.png
background_color: "#33669980"
android_12:
  icon: assets/splash.png
  icon_background_color: "#11223344"
`, "assets/splash.png")

	if c := resourceXML(ops, "values/drift_splash_colors.xml"); !strings.Contains(c, "#80336699") {
		t.Errorf("drift_splash_colors.xml not ARGB:\n%s", c)
	}
	if c := resourceXML(ops, "values-v31/styles.xml"); !strings.Contains(c, "#44112233") {
		t.Errorf("v31 icon background not ARGB:\n%s", c)
	}
	want := `red="0.2000" green="0.4000" blue="0.6000" alpha="0.5020"`
	if sb := launchStoryboard(ops); !strings.Contains(sb, want) {
		t.Errorf("storyboard colour missing %s:\n%s", want, sb)
	}
	if cfg := iosSource(ops, "SplashConfig.swift"); !strings.Contains(cfg, "UIColor(red: 0.2000, green: 0.4000, blue: 0.6000, alpha: 0.5020)") {
		t.Errorf("SplashConfig.swift colour wrong:\n%s", cfg)
	}
}

func TestBuild_RejectsNonPNGImage(t *testing.T) {
	root := t.TempDir()
	writeAsset(t, root, "assets/splash.png", []byte("not a png"))
	ctx := driftplugin.NewTestCtxAt(root)
	err := driftplugin.Bind("github.com/go-drift/drift/plugins/splash/plugin", Plugin).Build(ctx, []byte("image: assets/splash.png\n"))
	if err == nil || !strings.Contains(err.Error(), "must be a PNG") {
		t.Fatalf("err = %v, want PNG error", err)
	}
}

func TestBuild_RejectsNonPositiveImageWidth(t *testing.T) {
	if _, err := buildYAML(t, "image: assets/splash.png\nimage_width: 0\n", "assets/splash.png"); err == nil {
		t.Fatal("image_width 0 accepted")
	}
}
