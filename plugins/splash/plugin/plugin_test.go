package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// stubAsset writes a 1x1 PNG-shaped fake at projectRoot/<rel> so
// ctx.ResolveAsset returns bytes during Build. Contents don't need to be a
// valid PNG; the Build phase just bundles whatever ResolveAsset returns.
func stubAsset(t *testing.T, projectRoot, rel string) {
	t.Helper()
	path := filepath.Join(projectRoot, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("fake-png-bytes"), 0o644); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
}

// runBuild constructs a NewTestCtx, points its projectRoot at a temp dir
// seeded with the requested assets, and invokes splash{}.Build. Returns
// the recorded ops for assertion.
func runBuild(t *testing.T, cfg Config, assets ...string) []driftplugin.Op {
	t.Helper()
	root := t.TempDir()
	for _, a := range assets {
		stubAsset(t, root, a)
	}
	ctx := driftplugin.NewTestCtxAt(root)
	if err := (splash{}).Build(ctx, cfg); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx.Err: %v", err)
	}
	return ctx.Ops()
}

// hasOpType returns true if the op list contains at least one op with the
// given JSON discriminator.
func hasOpType(ops []driftplugin.Op, typ string) bool {
	for _, op := range ops {
		if op.Type() == typ {
			return true
		}
	}
	return false
}

func TestBuild_LightOnly(t *testing.T) {
	ops := runBuild(t, Config{
		Image:           "assets/splash.png",
		BackgroundColor: "#1A2238",
	}, "assets/splash.png")

	wantSome := []string{
		"ios.assets.add_image_set",
		"ios.storyboards.replace_launch_screen",
		"info_plist.set_string",
		"ios.source.add",
		"ios.registrant",
		"android.drawable.write",
		"android.resource.write_xml",
		"android.source.add",
		"android.registrant",
	}
	for _, typ := range wantSome {
		if !hasOpType(ops, typ) {
			t.Errorf("missing required op type %q in light-only build", typ)
		}
	}

	wantNone := []string{
		"android.gradle.add_dependency",
		"android.pre_activity_registrant",
	}
	for _, typ := range wantNone {
		if hasOpType(ops, typ) {
			t.Errorf("light-only build should not emit %q", typ)
		}
	}
}

func TestBuild_Android12_EmitsGradleAndPreActivity(t *testing.T) {
	ops := runBuild(t, Config{
		Image:           "assets/splash.png",
		BackgroundColor: "#1A2238",
		Android12: &Android12{
			Icon:                "assets/splash_icon.png",
			IconBackgroundColor: "#FFFFFF",
		},
	}, "assets/splash.png", "assets/splash_icon.png")

	if !hasOpType(ops, "android.gradle.add_dependency") {
		t.Errorf("android_12 config should emit gradle dependency op")
	}
	if !hasOpType(ops, "android.pre_activity_registrant") {
		t.Errorf("android_12 config should emit pre-activity registrant op")
	}

	// Verify the gradle dep coord pins core-splashscreen 1.0.1 (no floating).
	for _, op := range ops {
		if dep, ok := op.(*driftplugin.OpAndroidGradleAddDependency); ok {
			if !strings.Contains(dep.Coord, "androidx.core:core-splashscreen:") {
				t.Errorf("gradle dep coord wrong: %s", dep.Coord)
			}
			if !strings.HasSuffix(dep.Coord, ":1.0.1") {
				t.Errorf("gradle dep version must be pinned to 1.0.1; got %s", dep.Coord)
			}
		}
	}
}

func TestBuild_DarkVariant_AddsNightBucketResources(t *testing.T) {
	ops := runBuild(t, Config{
		Image:           "assets/splash.png",
		BackgroundColor: "#FFFFFF",
		Dark: &DarkVariant{
			Image:           "assets/splash_dark.png",
			BackgroundColor: "#000000",
		},
	}, "assets/splash.png", "assets/splash_dark.png")

	var sawNightDrawable, sawNightColors bool
	for _, op := range ops {
		if rx, ok := op.(*driftplugin.OpAndroidWriteResourceXML); ok {
			if strings.Contains(rx.RelPath, "drawable-night/") {
				sawNightDrawable = true
			}
			if strings.Contains(rx.RelPath, "values-night/") {
				sawNightColors = true
			}
		}
	}
	if !sawNightDrawable {
		t.Errorf("dark variant should emit drawable-night/ resource XML")
	}
	if !sawNightColors {
		t.Errorf("dark variant should emit values-night/ resource XML")
	}
}

func TestBuild_RejectsBadHexColor(t *testing.T) {
	root := t.TempDir()
	stubAsset(t, root, "assets/splash.png")
	ctx := driftplugin.NewTestCtxAt(root)
	err := (splash{}).Build(ctx, Config{
		Image:           "assets/splash.png",
		BackgroundColor: "not-a-color",
	})
	if err == nil {
		t.Fatal("expected error for invalid background_color")
	}
	if !strings.Contains(err.Error(), "background_color") {
		t.Errorf("error should name the bad field: %v", err)
	}
}

func TestBuild_RegistrantSymbol(t *testing.T) {
	ops := runBuild(t, Config{
		Image: "assets/splash.png",
	}, "assets/splash.png")

	var sawIOS, sawAndroid bool
	for _, op := range ops {
		switch v := op.(type) {
		case *driftplugin.OpRegistrantIOS:
			if v.Symbol == "DriftSplashPlugin.register" {
				sawIOS = true
			}
		case *driftplugin.OpRegistrantAndroid:
			if v.Symbol == "com.drift.plugin.splash.DriftSplashPlugin.register" {
				sawAndroid = true
			}
		}
	}
	if !sawIOS {
		t.Errorf("missing iOS DriftSplashPlugin.register registrant")
	}
	if !sawAndroid {
		t.Errorf("missing Android DriftSplashPlugin.register registrant")
	}
}
