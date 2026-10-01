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

func TestBuild_EmitsBothPlatforms(t *testing.T) {
	ops := runBuild(t, `
image: assets/splash.png
background_color: "#1A2238"
`, "assets/splash.png")

	for _, typ := range []string{
		"ios.assets.add_image_set",
		"ios.storyboards.replace_launch_screen",
		"ios.source.add",
		"ios.plugin",
		"android.drawable.write",
		"android.color.set",
		"android.style.set",
		"android.manifest.set_activity_attr",
		"android.source.add",
		"android.plugin",
	} {
		if !hasOpType(ops, typ) {
			t.Errorf("missing op type %q", typ)
		}
	}
	// The platform splash needs no library and no resource files of its own.
	for _, typ := range []string{"android.gradle.add_dependency", "android.resource.write_xml"} {
		if hasOpType(ops, typ) {
			t.Errorf("unexpected op type %q", typ)
		}
	}
	// UILaunchStoryboardName=LaunchScreen is in both Info.plist templates.
	if hasOpType(ops, "ios.plist.set_string") {
		t.Errorf("splash should not set Info.plist keys the templates already set")
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

// Config the plugin cannot honour on every platform is not accepted:
// unknown keys fail the build rather than doing nothing.
func TestBuild_RejectsRemovedFields(t *testing.T) {
	for _, field := range []string{
		"branding: assets/b.png",
		"branding_position: bottom",
		"dark:\n  image: assets/splash.png",
		"android_12:\n  icon: assets/splash.png",
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

func launchStoryboard(ops []protocol.Op) string {
	for _, op := range ops {
		if sb, ok := op.(*protocol.OpIOSReplaceLaunchScreen); ok {
			return sb.Content
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

// The launch screen and the iOS overlay show the image at image_width with
// the PNG's aspect ratio (4x2 here).
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
}

func TestBuild_ImageWidthDefault(t *testing.T) {
	ops := runBuild(t, "image: assets/splash.png\n", "assets/splash.png")
	if !strings.Contains(launchStoryboard(ops), `firstAttribute="width" constant="200"`) {
		t.Errorf("default image_width should be 200")
	}
}

// Colours stay in Drift's format for android.color.set (Drift converts) and
// become fractional components on iOS.
func TestBuild_ColoursPerPlatform(t *testing.T) {
	ops := runBuild(t, `
image: assets/splash.png
background_color: "#33669980"
android:
  icon_background_color: "#11223344"
`, "assets/splash.png")

	colors := androidColors(ops)
	if colors["drift_splash_background"] != "#33669980" || colors["drift_splash_icon_background"] != "#11223344" {
		t.Errorf("android colours = %v", colors)
	}
	want := `red="0.2000" green="0.4000" blue="0.6000" alpha="0.5020"`
	if sb := launchStoryboard(ops); !strings.Contains(sb, want) {
		t.Errorf("storyboard colour missing %s:\n%s", want, sb)
	}
	if cfg := iosSource(ops, "SplashConfig.swift"); !strings.Contains(cfg, "UIColor(red: 0.2000, green: 0.4000, blue: 0.6000, alpha: 0.5020)") {
		t.Errorf("SplashConfig.swift colour wrong:\n%s", cfg)
	}
}

func androidColors(ops []protocol.Op) map[string]string {
	m := map[string]string{}
	for _, op := range ops {
		if c, ok := op.(*protocol.OpAndroidColorSet); ok {
			m[c.Name] = c.Value
		}
	}
	return m
}

func splashStyle(t *testing.T, ops []protocol.Op) map[string]string {
	t.Helper()
	for _, op := range ops {
		if st, ok := op.(*protocol.OpAndroidStyleSet); ok && st.Name == "Drift.Splash" {
			if st.Parent != "LaunchTheme" {
				t.Errorf("Drift.Splash parent = %q, want LaunchTheme", st.Parent)
			}
			m := map[string]string{}
			for _, it := range st.Items {
				m[it.Name] = it.Value
			}
			return m
		}
	}
	t.Fatal("no Drift.Splash style")
	return nil
}

// Android's splash is the platform one, styled through MainActivity's
// launch theme. The icon defaults to image; no icon background unless set.
func TestBuild_AndroidPlatformSplash(t *testing.T) {
	ops := runBuild(t, "image: assets/splash.png\n", "assets/splash.png")

	style := splashStyle(t, ops)
	if style["android:windowSplashScreenBackground"] != "@color/drift_splash_background" ||
		style["android:windowSplashScreenAnimatedIcon"] != "@drawable/drift_splash_icon" {
		t.Errorf("Drift.Splash items = %v", style)
	}
	if _, ok := style["android:windowSplashScreenIconBackgroundColor"]; ok {
		t.Errorf("icon background set without android.icon_background_color")
	}
	var themed bool
	for _, op := range ops {
		if a, ok := op.(*protocol.OpAndroidManifestSetActivityAttr); ok &&
			a.Activity == ".MainActivity" && a.Attr == "android:theme" && a.Value == "@style/Drift.Splash" {
			themed = true
		}
	}
	if !themed {
		t.Error("MainActivity theme not set to @style/Drift.Splash")
	}
	var image4x2 bytes.Buffer
	if err := png.Encode(&image4x2, image.NewRGBA(image.Rect(0, 0, 4, 2))); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(drawable(t, ops, "drift_splash_icon"), image4x2.Bytes()) {
		t.Error("drift_splash_icon should default to image")
	}
}

func drawable(t *testing.T, ops []protocol.Op, name string) []byte {
	t.Helper()
	for _, op := range ops {
		if d, ok := op.(*protocol.OpAndroidWriteDrawable); ok && d.Name == name {
			b, err := protocol.DecodeContent(d.Content)
			if err != nil {
				t.Fatalf("decode %s: %v", name, err)
			}
			return b
		}
	}
	t.Fatalf("no drawable %s", name)
	return nil
}

func TestBuild_AndroidIconOverride(t *testing.T) {
	root := t.TempDir()
	stubAsset(t, root, "assets/splash.png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 3, 3))); err != nil {
		t.Fatal(err)
	}
	writeAsset(t, root, "assets/icon.png", buf.Bytes())
	ctx := driftplugin.NewTestCtxAt(root)
	err := driftplugin.Bind("github.com/go-drift/drift/plugins/splash/plugin", Plugin).Build(ctx,
		[]byte("image: assets/splash.png\nandroid:\n  icon: assets/icon.png\n  icon_background_color: \"#FFFFFF\"\n"))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if !bytes.Equal(drawable(t, ctx.Ops(), "drift_splash_icon"), buf.Bytes()) {
		t.Error("android.icon not used for drift_splash_icon")
	}
	if splashStyle(t, ctx.Ops())["android:windowSplashScreenIconBackgroundColor"] != "@color/drift_splash_icon_background" {
		t.Error("icon background not wired")
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

func TestBuild_MaxDuration(t *testing.T) {
	ops := runBuild(t, "image: assets/splash.png\n", "assets/splash.png")
	if cfg := iosSource(ops, "SplashConfig.swift"); !strings.Contains(cfg, "maxDurationMs = 10000") {
		t.Errorf("default max_duration_ms should be 10000:\n%s", cfg)
	}
	if _, err := buildYAML(t, "image: assets/splash.png\nmax_duration_ms: 0\n", "assets/splash.png"); err == nil {
		t.Error("max_duration_ms 0 accepted")
	}
}
