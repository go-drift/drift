package mutate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

const baseManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <!-- User-added comment, must be preserved -->
    <uses-permission android:name="android.permission.INTERNET" />

    <application android:label="App">
        <activity android:name=".MainActivity"
                  android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
`

func writeManifest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "AndroidManifest.xml")
	if err := os.WriteFile(p, []byte(baseManifest), 0o644); err != nil {
		t.Fatalf("seed manifest: %v", err)
	}
	return p
}

func TestApplyAndroidManifestAddPermission(t *testing.T) {
	path := writeManifest(t)
	ops := []*driftplugin.OpAndroidManifestAddPermission{
		{Base: driftplugin.Base{Pkg: "a"}, Name: "android.permission.CAMERA"},
		{Base: driftplugin.Base{Pkg: "a"}, Name: "android.permission.INTERNET"}, // dedupe
	}
	changed, err := ApplyAndroidManifest(path, ops, nil, nil, nil)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !changed {
		t.Errorf("expected changed=true")
	}
	body, _ := os.ReadFile(path)
	if strings.Count(string(body), "android.permission.INTERNET") != 1 {
		t.Errorf("expected single INTERNET permission, body:\n%s", body)
	}
	if !strings.Contains(string(body), "android.permission.CAMERA") {
		t.Errorf("expected CAMERA permission, body:\n%s", body)
	}
	if !strings.Contains(string(body), "User-added comment") {
		t.Errorf("comment should be preserved")
	}
}

func TestApplyAndroidManifestSetActivityAttr(t *testing.T) {
	path := writeManifest(t)
	ops := []*driftplugin.OpAndroidManifestSetActivityAttr{
		{Base: driftplugin.Base{Pkg: "p"}, Activity: ".MainActivity", Attr: "android:theme", Value: "@style/Splash"},
	}
	if _, err := ApplyAndroidManifest(path, nil, nil, ops, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "android:theme=\"@style/Splash\"") {
		t.Errorf("expected android:theme attr, body:\n%s", body)
	}
}

func TestApplyAndroidColorsCreates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "values/plugin_colors.xml")
	ops := []*driftplugin.OpAndroidColorSet{
		{Base: driftplugin.Base{Pkg: "p"}, Name: "splash_bg", Value: "#FFFFFF"},
	}
	wrote, changed, err := ApplyAndroidColors(path, ops)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !changed || wrote != path {
		t.Errorf("expected changed=true and path: %s changed=%v", wrote, changed)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), `<color name="splash_bg">#FFFFFF</color>`) {
		t.Errorf("colors file content wrong:\n%s", body)
	}
}

func TestApplyAndroidStylesReplacesItems(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "values/plugin_styles.xml")
	ops := []*driftplugin.OpAndroidStyleSet{
		{
			Base:   driftplugin.Base{Pkg: "p"},
			Name:   "SplashTheme",
			Parent: "Theme.AppCompat",
			Items:  []driftplugin.StyleItem{{Name: "android:windowBackground", Value: "@drawable/splash"}},
		},
	}
	_, _, err := ApplyAndroidStyles(path, ops)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), `<style name="SplashTheme" parent="Theme.AppCompat">`) {
		t.Errorf("style file content wrong:\n%s", body)
	}
}
