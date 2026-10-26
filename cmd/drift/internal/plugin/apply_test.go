package plugin

import (
	b64 "encoding/base64"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-drift/drift/cmd/drift/internal/plugin/mutate"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// commitPlan runs plan against a fresh changeset and commits it when plan
// succeeds, as a build does.
func commitPlan[R any](t *testing.T, plan func(cs *mutate.Changeset) (R, error)) (R, error) {
	t.Helper()
	cs := mutate.NewChangeset()
	r, err := plan(cs)
	if err == nil {
		if cerr := cs.Commit(); cerr != nil {
			t.Fatalf("Commit: %v", cerr)
		}
	}
	return r, err
}

// apply runs Apply and commits its plan.
func apply(t *testing.T, ops []protocol.Op, dir, platform string, prev *Lock) ([]string, error) {
	t.Helper()
	return commitPlan(t, func(cs *mutate.Changeset) ([]string, error) { return Apply(cs, ops, dir, platform, prev) })
}

// ensureRunnerSupport runs EnsureRunnerSupport and commits its plan.
func ensureRunnerSupport(t *testing.T, dir, platform string) ([]string, error) {
	t.Helper()
	return commitPlan(t, func(cs *mutate.Changeset) ([]string, error) { return EnsureRunnerSupport(cs, dir, platform) })
}

// writeRegistrant runs WriteRegistrant and commits its plan.
func writeRegistrant(t *testing.T, dir, platform string, ops []protocol.Op) ([]string, error) {
	t.Helper()
	return commitPlan(t, func(cs *mutate.Changeset) ([]string, error) { return WriteRegistrant(cs, dir, platform, ops) })
}

func TestApplyWritesKotlinSources(t *testing.T) {
	dir := t.TempDir()
	body := "package com.foo.camera\nclass Stub {}\n"
	ops := []protocol.Op{
		&protocol.OpAddKotlinSource{
			Base:    protocol.Base{Pkg: "p"},
			Package: "com.foo.camera",
			RelPath: "Stub.kt",
			Content: base64(body),
		},
	}
	changed, err := apply(t, ops, dir, "android", nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := filepath.Join(dir, "app/src/main/java/com/foo/camera/Stub.kt")
	if len(changed) != 1 || changed[0] != want {
		t.Fatalf("changed = %v, want one path %q", changed, want)
	}
	got, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(got) != body {
		t.Errorf("body roundtrip: got %q want %q", got, body)
	}
}

func TestApplySkipsOtherPlatformOps(t *testing.T) {
	dir := t.TempDir()
	ops := []protocol.Op{
		&protocol.OpAndroidManifestAddPermission{Base: protocol.Base{Pkg: "p"}, Name: "android.permission.CAMERA"},
	}
	// Targeting iOS but op is android: should be skipped without error.
	if _, err := apply(t, ops, dir, "ios", nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func base64(s string) string {
	return b64.StdEncoding.EncodeToString([]byte(s))
}

// Every op type the CLI can decode has an opKind; without one, Apply
// panics on the first build that uses it.
func TestEveryOpTypeHasAKind(t *testing.T) {
	for _, typ := range protocol.OpTypes() {
		op, err := protocol.NewOp(typ)
		if err != nil {
			t.Fatalf("NewOp(%q): %v", typ, err)
		}
		if _, ok := opKinds[reflect.TypeOf(op)]; !ok {
			t.Errorf("op type %q has no opKind", typ)
		}
	}
}

func TestApplyIOSBundleResourcesLandInPluginResources(t *testing.T) {
	dir := t.TempDir()
	ops := []protocol.Op{
		&protocol.OpIOSAddBundleResource{Base: protocol.Base{Pkg: "fb"}, Path: "GoogleService-Info.plist", Content: base64("plist")},
	}
	if _, err := apply(t, ops, dir, "ios", nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "Runner", "PluginResources", "GoogleService-Info.plist"))
	if err != nil || string(got) != "plist" {
		t.Fatalf("bundle resource not written under Runner/PluginResources: %q, %v", got, err)
	}
}

// On xtool, bundle resources and image sets both become main-bundle
// resources listed in xtool.yml; nothing goes through SwiftPM resources or
// an asset catalog.
func TestApplyXtoolBundleResourcesAndImageSets(t *testing.T) {
	dir := t.TempDir()
	yml := filepath.Join(dir, "xtool.yml")
	if err := os.WriteFile(yml, []byte("bundleID: com.example.app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ops := []protocol.Op{
		&protocol.OpIOSAddBundleResource{Base: protocol.Base{Pkg: "fb"}, Path: "GoogleService-Info.plist", Content: base64("plist")},
		&protocol.OpIOSAssetsAddImageSet{Base: protocol.Base{Pkg: "splash"}, Name: "DriftSplash", Image: base64("png")},
	}
	if _, err := apply(t, ops, dir, "xtool", nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for name, want := range map[string]string{"GoogleService-Info.plist": "plist", "DriftSplash.png": "png"} {
		got, err := os.ReadFile(filepath.Join(dir, "PluginResources", name))
		if err != nil || string(got) != want {
			t.Errorf("%s: got %q, %v", name, got, err)
		}
	}
	body, _ := os.ReadFile(yml)
	for _, want := range []string{"PluginResources/DriftSplash.png", "PluginResources/GoogleService-Info.plist"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("xtool.yml missing %q:\n%s", want, body)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "Sources", "Runner", "Resources", "Assets.xcassets")); !os.IsNotExist(err) {
		t.Errorf("xtool build must not get an asset catalog")
	}

	// Removing the plugins prunes their files and the xtool.yml entries.
	if _, err := apply(t, nil, dir, "xtool", nil); err != nil {
		t.Fatalf("Apply(nil): %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "PluginResources")); !os.IsNotExist(err) {
		t.Errorf("PluginResources should be removed when no plugin provides resources")
	}
	body, _ = os.ReadFile(yml)
	if strings.Contains(string(body), "resources") {
		t.Errorf("xtool.yml resources should be cleared:\n%s", body)
	}
}

func TestApplyAndroidAppModuleFile(t *testing.T) {
	dir := t.TempDir()
	ops := []protocol.Op{
		&protocol.OpAndroidAddAppModuleFile{Base: protocol.Base{Pkg: "fb"}, Name: "google-services.json", Content: base64("{}")},
	}
	if _, err := apply(t, ops, dir, "android", nil); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "app", "google-services.json")); err != nil || string(got) != "{}" {
		t.Fatalf("google-services.json not written next to app/build.gradle: %q, %v", got, err)
	}
}

// Entitlement ops edit Runner.entitlements at the project root on both iOS
// paths, and Info.plist ops leave it alone.
func TestApplyEntitlementsAndInfoPlistAreSeparateFiles(t *testing.T) {
	const empty = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict/></plist>
`
	for platform, infoPath := range map[string]string{
		"ios":   "Runner/Info.plist",
		"xtool": "Sources/Runner/Resources/Info.plist",
	} {
		t.Run(platform, func(t *testing.T) {
			dir := t.TempDir()
			writeTree(t, dir, map[string]string{infoPath: empty, "Runner.entitlements": empty, "xtool.yml": "bundleID: com.example.app\n"})
			ops := []protocol.Op{
				&protocol.OpPlistSetString{
					Base:       protocol.Base{Pkg: "fb"},
					PlistEntry: protocol.PlistEntry{File: protocol.PlistEntitlements, Key: "aps-environment"},
					Value:      "development",
				},
				&protocol.OpPlistSetBool{
					Base:       protocol.Base{Pkg: "fb"},
					PlistEntry: protocol.PlistEntry{File: protocol.PlistInfo, Key: "FirebaseAppDelegateProxyEnabled"},
					Value:      false,
				},
			}
			if _, err := apply(t, ops, dir, platform, nil); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			entitlements := readFile(t, filepath.Join(dir, "Runner.entitlements"))
			info := readFile(t, filepath.Join(dir, infoPath))
			if !strings.Contains(entitlements, "aps-environment") || strings.Contains(entitlements, "FirebaseAppDelegateProxyEnabled") {
				t.Errorf("Runner.entitlements:\n%s", entitlements)
			}
			if !strings.Contains(info, "FirebaseAppDelegateProxyEnabled") || strings.Contains(info, "aps-environment") {
				t.Errorf("Info.plist:\n%s", info)
			}
		})
	}
}

// Apply only plans: an iOS build whose last step fails (a plugin class
// with no Swift sources) leaves the plist edit, asset catalog and bundle
// resource earlier ops planned off disk.
func TestApplyWritesNothingWhenALaterStepFails(t *testing.T) {
	dir := t.TempDir()
	const empty = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict/></plist>
`
	tree := map[string]string{"Runner/Info.plist": empty, "Runner.entitlements": empty}
	writeTree(t, dir, tree)
	base := protocol.Base{Pkg: "github.com/acme/maps"}
	ops := []protocol.Op{
		&protocol.OpPlistSetString{Base: base, PlistEntry: protocol.PlistEntry{File: protocol.PlistInfo, Key: "MapsKey"}, Value: "k"},
		&protocol.OpIOSAssetsAddImageSet{Base: base, Name: "pin", Image: base64("png")},
		&protocol.OpIOSAddBundleResource{Base: base, Path: "maps.json", Content: base64("{}")},
		&protocol.OpIOSPlugin{Base: base, Class: "MapsPlugin"},
	}
	cs := mutate.NewChangeset()
	if _, err := Apply(cs, ops, dir, "ios", nil); err == nil || !strings.Contains(err.Error(), "ships no Swift sources") {
		t.Fatalf("err = %v, want the missing-sources error", err)
	}
	var got []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			got = append(got, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(tree) {
		t.Errorf("files after the failed Apply = %v, want only %v", got, tree)
	}
	for rel, want := range tree {
		if body := readFile(t, filepath.Join(dir, rel)); body != want {
			t.Errorf("%s changed:\n%s", rel, body)
		}
	}
}
