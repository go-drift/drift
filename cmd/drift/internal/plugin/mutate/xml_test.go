package mutate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
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
	ops := []*protocol.OpAndroidManifestAddPermission{
		{Base: protocol.Base{Pkg: "a"}, Name: "android.permission.CAMERA"},
		{Base: protocol.Base{Pkg: "a"}, Name: "android.permission.INTERNET"}, // dedupe
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
	ops := []*protocol.OpAndroidManifestSetActivityAttr{
		{Base: protocol.Base{Pkg: "p"}, Activity: ".MainActivity", Attr: "android:theme", Value: "@style/Splash"},
	}
	if _, err := ApplyAndroidManifest(path, nil, nil, ops, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "android:theme=\"@style/Splash\"") {
		t.Errorf("expected android:theme attr, body:\n%s", body)
	}
}

func TestWriteAndroidColorsCreates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "values/plugin_colors.xml")
	ops := []*protocol.OpAndroidColorSet{
		{Base: protocol.Base{Pkg: "p"}, Name: "splash_bg", Value: "#FFFFFF"},
	}
	wrote, changed, err := WriteAndroidColors(path, ops)
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

func TestWriteAndroidStyles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "values/plugin_styles.xml")
	ops := []*protocol.OpAndroidStyleSet{
		{
			Base:   protocol.Base{Pkg: "p"},
			Name:   "SplashTheme",
			Parent: "Theme.AppCompat",
			Items:  []protocol.StyleItem{{Name: "android:windowBackground", Value: "@drawable/splash"}},
		},
	}
	_, _, err := WriteAndroidStyles(path, ops)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), `<style name="SplashTheme" parent="Theme.AppCompat">`) {
		t.Errorf("style file content wrong:\n%s", body)
	}
}

// The plugin values files are Drift-owned: each write replaces the
// previous content, and no ops removes the file, so a dropped plugin's
// entries never linger.
func TestWriteAndroidColorsRegeneratesAndRemoves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "values/plugin_colors.xml")
	set := func(name string) []*protocol.OpAndroidColorSet {
		return []*protocol.OpAndroidColorSet{{Base: protocol.Base{Pkg: "p"}, Name: name, Value: "#000000"}}
	}
	if _, _, err := WriteAndroidColors(path, set("old")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := WriteAndroidColors(path, set("new")); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if strings.Contains(string(body), "old") || !strings.Contains(string(body), "new") {
		t.Errorf("file should hold only the latest entries:\n%s", body)
	}
	if _, changed, err := WriteAndroidColors(path, nil); err != nil || !changed {
		t.Fatalf("remove: changed=%v err=%v", changed, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should be removed with no ops: %v", err)
	}
	if _, changed, err := WriteAndroidColors(path, nil); err != nil || changed {
		t.Errorf("removing a missing file: changed=%v err=%v", changed, err)
	}
}
