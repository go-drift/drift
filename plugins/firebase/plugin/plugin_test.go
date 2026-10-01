package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

const fullConfig = `
ios:
  config_file: firebase/GoogleService-Info.plist
android:
  config_file: firebase/google-services.json
`

// build runs the plugin the way the bridge does (schema check, decode,
// Build) for platform, in a project holding the testdata config files,
// optionally edited by replace (old, new pairs) first.
func build(t *testing.T, platform, config string, replace ...string) ([]protocol.Op, error) {
	t.Helper()
	root := t.TempDir()
	for _, name := range []string{"GoogleService-Info.plist", "google-services.json"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		for i := 0; i+1 < len(replace); i += 2 {
			content = strings.ReplaceAll(content, replace[i], replace[i+1])
		}
		if err := os.MkdirAll(filepath.Join(root, "firebase"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "firebase", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := driftplugin.NewTestCtxFor(root, platform)
	if err := driftplugin.Bind("github.com/go-drift/drift/plugins/firebase/plugin", Plugin).Build(ctx, []byte(config)); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx.Err: %v", err)
	}
	return ctx.Ops(), nil
}

func mustBuild(t *testing.T, platform, config string) []protocol.Op {
	t.Helper()
	ops, err := build(t, platform, config)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return ops
}

func find[T protocol.Op](ops []protocol.Op, match func(T) bool) (T, bool) {
	for _, op := range ops {
		if v, ok := op.(T); ok && match(v) {
			return v, true
		}
	}
	var zero T
	return zero, false
}

func TestBuildIOS(t *testing.T) {
	ops := mustBuild(t, "ios", fullConfig)
	for _, op := range ops {
		if op.Platform() != "ios" {
			t.Errorf("iOS build emitted %s", op.Type())
		}
	}
	spm, ok := find(ops, func(*protocol.OpIOSAddPackageDependency) bool { return true })
	if !ok || spm.URL != firebaseIOSSDK || spm.Requirement.Kind != protocol.SPMExact ||
		strings.Join(spm.Products, ",") != "FirebaseCore,FirebaseMessaging" {
		t.Errorf("SwiftPM dependency = %+v", spm)
	}
	res, ok := find(ops, func(o *protocol.OpIOSAddBundleResource) bool { return o.Path == "GoogleService-Info.plist" })
	if content, _ := protocol.DecodeContent(res.Content); !ok || !strings.Contains(string(content), "com.example.app") {
		t.Error("GoogleService-Info.plist not shipped verbatim in the bundle")
	}
	if _, ok := find(ops, func(o *protocol.OpPlistSetBool) bool {
		return o.File == protocol.PlistInfo && o.Key == "FirebaseAppDelegateProxyEnabled" && !o.Value
	}); !ok {
		t.Error("swizzling not disabled")
	}
	if _, ok := find(ops, func(o *protocol.OpPlistAppendArrayItem) bool {
		return o.File == protocol.PlistInfo && o.Key == "UIBackgroundModes" && o.Value == "remote-notification"
	}); !ok {
		t.Error("remote-notification background mode missing")
	}
	if _, ok := find(ops, func(o *protocol.OpPlistSetString) bool {
		return o.File == protocol.PlistEntitlements && o.Key == "aps-environment"
	}); !ok {
		t.Error("aps-environment entitlement missing")
	}
	if _, ok := find(ops, func(o *protocol.OpIOSPlugin) bool { return o.Class == "DriftFirebasePlugin" }); !ok {
		t.Error("plugin class not registered")
	}
	if _, ok := find(ops, func(o *protocol.OpAddIOSSource) bool { return o.RelPath == "DriftFirebasePlugin.swift" }); !ok {
		t.Error("Swift source missing")
	}
}

func TestBuildAndroid(t *testing.T) {
	ops := mustBuild(t, "android", fullConfig)
	for _, op := range ops {
		if op.Platform() != "android" {
			t.Errorf("Android build emitted %s", op.Type())
		}
	}
	if _, ok := find(ops, func(o *protocol.OpAndroidGradleAddDependency) bool { return o.Coord == firebaseMessagingCoord }); !ok {
		t.Error("firebase-messaging dependency missing")
	}
	if _, ok := find(ops, func(o *protocol.OpAndroidGradleApplyPlugin) bool {
		return o.ID == googleServicesPlugin && o.Version == googleServicesVersion
	}); !ok {
		t.Error("google-services Gradle plugin missing")
	}
	if _, ok := find(ops, func(o *protocol.OpAndroidAddAppModuleFile) bool { return o.Name == "google-services.json" }); !ok {
		t.Error("google-services.json missing")
	}
	service, ok := find(ops, func(*protocol.OpAndroidManifestAddService) bool { return true })
	if !ok || service.ServiceName() != androidPackage+".DriftFirebaseMessagingService" ||
		!strings.Contains(service.XML, "com.google.firebase.MESSAGING_EVENT") {
		t.Errorf("messaging service = %+v", service)
	}
	if _, ok := find(ops, func(o *protocol.OpAndroidPlugin) bool { return o.Class == androidPackage+".DriftFirebasePlugin" }); !ok {
		t.Error("plugin class not registered")
	}
	for _, src := range []string{"DriftFirebasePlugin.kt", "DriftFirebaseMessagingService.kt"} {
		if _, ok := find(ops, func(o *protocol.OpAddKotlinSource) bool { return o.RelPath == src }); !ok {
			t.Errorf("Kotlin source %s missing", src)
		}
	}
}

// A platform builds from its own section only.
func TestBuildNeedsOnlyThePlatformsSection(t *testing.T) {
	if _, err := build(t, "android", "android:\n  config_file: firebase/google-services.json\n"); err != nil {
		t.Errorf("Android with only android: %v", err)
	}
	_, err := build(t, "ios", "android:\n  config_file: firebase/google-services.json\n")
	if err == nil || !strings.Contains(err.Error(), "ios.config_file") {
		t.Errorf("iOS without ios: err = %v, want one naming ios.config_file", err)
	}
}

func TestBuildRejectsXtool(t *testing.T) {
	_, err := build(t, "xtool", fullConfig)
	if err == nil || !strings.Contains(err.Error(), "xtool") {
		t.Errorf("err = %v, want xtool unsupported", err)
	}
}

// Config files for another app fail the build, naming both ids.
func TestBuildRejectsConfigForAnotherApp(t *testing.T) {
	for _, platform := range []string{"ios", "android"} {
		_, err := build(t, platform, fullConfig, driftplugin.TestAppID, "com.example.other")
		if err == nil || !strings.Contains(err.Error(), "com.example.other") || !strings.Contains(err.Error(), driftplugin.TestAppID) {
			t.Errorf("%s: err = %v, want a mismatch naming both ids", platform, err)
		}
	}
}

func TestBuildRejectsMalformedConfigFiles(t *testing.T) {
	if _, err := build(t, "ios", fullConfig, "GCM_SENDER_ID", "OTHER_KEY"); err == nil {
		t.Error("iOS: plist without GCM_SENDER_ID accepted")
	}
	if _, err := build(t, "android", fullConfig, `"project_info"`, `"other"`); err == nil {
		t.Error("Android: json without project_info accepted")
	}
}
