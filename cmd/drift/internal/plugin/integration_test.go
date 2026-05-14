package plugin

import (
	b64 "encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// TestPipelineEndToEndAndroid exercises the CLI-side pipeline that runs
// after the bridge returns ops: DecodeOps → Validate → Apply →
// EnsureRunnerSupport → WriteRegistrant. The bridge subprocess itself is
// covered by pkg/plugin/runtime_test.go; this test fills the integration
// gap on the CLI side by feeding hand-crafted ops through the same code
// path Workspace.runPluginPipeline drives.
func TestPipelineEndToEndAndroid(t *testing.T) {
	dir := t.TempDir()
	seedAndroidScaffold(t, dir)

	kotlinBody := "package com.example.splash\nclass SplashPlugin\n"
	ops := []driftplugin.Op{
		&driftplugin.OpAndroidManifestAddPermission{
			Base: driftplugin.Base{Pkg: "github.com/example/splash", Ident: "splash"},
			Name: "android.permission.POST_NOTIFICATIONS",
		},
		&driftplugin.OpAndroidManifestSetActivityAttr{
			Base:     driftplugin.Base{Pkg: "github.com/example/splash", Ident: "splash"},
			Activity: ".MainActivity",
			Attr:     "android:theme",
			Value:    "@style/Drift.Splash",
		},
		&driftplugin.OpAndroidColorSet{
			Base:  driftplugin.Base{Pkg: "github.com/example/splash", Ident: "splash"},
			Name:  "drift_splash_background",
			Value: "#1A2238",
		},
		&driftplugin.OpAndroidStyleSet{
			Base:   driftplugin.Base{Pkg: "github.com/example/splash", Ident: "splash"},
			Name:   "Drift.Splash",
			Parent: "Theme.AppCompat.NoActionBar",
			Items:  []driftplugin.StyleItem{{Name: "android:windowBackground", Value: "@color/drift_splash_background"}},
		},
		&driftplugin.OpAddKotlinSource{
			Base:    driftplugin.Base{Pkg: "github.com/example/splash", Ident: "splash"},
			Package: "com.example.splash",
			RelPath: "SplashPlugin.kt",
			Content: b64.StdEncoding.EncodeToString([]byte(kotlinBody)),
		},
		// Two plugins emitting the same permission must collapse without
		// surfacing a conflict.
		&driftplugin.OpAndroidManifestAddPermission{
			Base: driftplugin.Base{Pkg: "github.com/example/other", Ident: "other"},
			Name: "android.permission.POST_NOTIFICATIONS",
		},
		&driftplugin.OpRegistrantAndroid{
			Base:   driftplugin.Base{Pkg: "github.com/example/splash", Ident: "splash"},
			Symbol: "com.example.splash.SplashPlugin.register",
		},
	}

	normalized, err := Validate(ops)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := countOpType(normalized, "android.manifest.add_permission"); got != 1 {
		t.Errorf("duplicate permission should collapse to 1, got %d", got)
	}

	if _, err := Apply(normalized, dir, "android"); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := EnsureRunnerSupport(dir, "android"); err != nil {
		t.Fatalf("EnsureRunnerSupport: %v", err)
	}
	if _, err := WriteRegistrant(dir, "android", normalized); err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}

	manifest := readFile(t, filepath.Join(dir, "app/src/main/AndroidManifest.xml"))
	if !strings.Contains(manifest, "android.permission.POST_NOTIFICATIONS") {
		t.Errorf("manifest missing POST_NOTIFICATIONS permission:\n%s", manifest)
	}
	if !strings.Contains(manifest, `android:theme="@style/Drift.Splash"`) {
		t.Errorf("manifest missing theme override:\n%s", manifest)
	}

	colors := readFile(t, filepath.Join(dir, "app/src/main/res/values/plugin_colors.xml"))
	if !strings.Contains(colors, "#1A2238") {
		t.Errorf("colors.xml missing splash colour:\n%s", colors)
	}

	styles := readFile(t, filepath.Join(dir, "app/src/main/res/values/plugin_styles.xml"))
	if !strings.Contains(styles, `parent="Theme.AppCompat.NoActionBar"`) {
		t.Errorf("styles.xml missing parent attr:\n%s", styles)
	}

	kotlinPath := filepath.Join(dir, "app/src/main/java/com/example/splash/SplashPlugin.kt")
	if got := readFile(t, kotlinPath); got != kotlinBody {
		t.Errorf("kotlin source mismatch:\nwant: %q\ngot:  %q", kotlinBody, got)
	}

	host := readFile(t, filepath.Join(dir, "app/src/main/java/com/drift/runner/DriftPluginHost.kt"))
	if !strings.Contains(host, "interface DriftPluginHost") {
		t.Errorf("DriftPluginHost.kt missing interface decl")
	}

	registrant := readFile(t, filepath.Join(dir, "app/src/main/java/com/drift/runner/DriftPluginRegistrant.kt"))
	if !strings.Contains(registrant, "com.example.splash.SplashPlugin.register(host)") {
		t.Errorf("registrant missing call:\n%s", registrant)
	}
}

// TestPipelineRejectsIncompatibleExclusiveOps confirms the validate stage
// surfaces ConflictError before any file is touched.
func TestPipelineRejectsIncompatibleExclusiveOps(t *testing.T) {
	ops := []driftplugin.Op{
		&driftplugin.OpIOSReplaceLaunchScreen{
			Base:    driftplugin.Base{Pkg: "a", Ident: "a"},
			Content: "<one/>",
		},
		&driftplugin.OpIOSReplaceLaunchScreen{
			Base:    driftplugin.Base{Pkg: "b", Ident: "b"},
			Content: "<two/>",
		},
	}
	_, err := Validate(ops)
	if err == nil {
		t.Fatal("expected conflict error from divergent exclusive ops")
	}
	if !strings.Contains(err.Error(), "plugin conflict") {
		t.Errorf("error should mention plugin conflict; got %v", err)
	}
}

// seedAndroidScaffold writes the minimal subset of the Android scaffold the
// mutators expect: AndroidManifest.xml with a <manifest>/<application>/
// <activity> chain, plus the res/values directory.
func seedAndroidScaffold(t *testing.T, root string) {
	t.Helper()
	manifest := `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application android:label="App">
        <activity android:name=".MainActivity" android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`
	path := filepath.Join(root, "app/src/main/AndroidManifest.xml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "app/src/main/res/values"), 0o755); err != nil {
		t.Fatalf("mkdir res/values: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func countOpType(ops []driftplugin.Op, typ string) int {
	n := 0
	for _, op := range ops {
		if op.Type() == typ {
			n++
		}
	}
	return n
}
