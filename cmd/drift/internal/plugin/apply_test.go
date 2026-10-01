package plugin

import (
	b64 "encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

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
	changed, err := Apply(ops, dir, "android")
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
	if _, err := Apply(ops, dir, "ios"); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func base64(s string) string {
	return b64.StdEncoding.EncodeToString([]byte(s))
}

// Every op type pkg/plugin can decode must be known to Apply; otherwise a
// plugin's op is silently dropped with only a runtime error report.
func TestApplyKnowsEveryOpType(t *testing.T) {
	for _, typ := range protocol.OpTypes() {
		op, err := protocol.NewOp(typ)
		if err != nil {
			t.Fatalf("NewOp(%q): %v", typ, err)
		}
		if !bundleOp(&opBag{}, op) {
			t.Errorf("Apply does not handle op type %q", typ)
		}
	}
}

func TestApplyIOSBundleResourcesLandInPluginResources(t *testing.T) {
	dir := t.TempDir()
	ops := []protocol.Op{
		&protocol.OpIOSAddBundleResource{Base: protocol.Base{Pkg: "fb"}, Path: "GoogleService-Info.plist", Content: base64("plist")},
	}
	if _, err := Apply(ops, dir, "ios"); err != nil {
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
	if _, err := Apply(ops, dir, "xtool"); err != nil {
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
	if _, err := Apply(nil, dir, "xtool"); err != nil {
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
	if _, err := Apply(ops, dir, "android"); err != nil {
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
			if _, err := Apply(ops, dir, platform); err != nil {
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
