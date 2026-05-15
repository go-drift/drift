package plugin

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-drift/drift/cmd/drift/internal/templates"
	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

func TestWriteRegistrantEmptyAndroid(t *testing.T) {
	dir := t.TempDir()
	changed, err := WriteRegistrant(dir, "android", nil)
	if err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}
	if len(changed) != 1 {
		t.Fatalf("expected 1 file changed, got %d", len(changed))
	}
	body, err := os.ReadFile(filepath.Join(dir, "app/src/main/java/com/drift/runner/DriftPluginRegistrant.kt"))
	if err != nil {
		t.Fatalf("read registrant: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "package com.drift.runner") {
		t.Errorf("registrant missing package decl")
	}
	if !strings.Contains(s, "import android.app.Activity") {
		t.Errorf("registrant missing Activity import (needed by preActivityCreate)")
	}
	if !strings.Contains(s, "object DriftPluginRegistrant") {
		t.Errorf("registrant missing object")
	}
	if !strings.Contains(s, "fun registerAll(host: DriftPluginHost)") {
		t.Errorf("registrant missing registerAll signature")
	}
	if !strings.Contains(s, "fun preActivityCreate(activity: Activity)") {
		t.Errorf("registrant missing preActivityCreate signature")
	}
}

func TestWriteRegistrantWithPreActivityEntries(t *testing.T) {
	dir := t.TempDir()
	ops := []driftplugin.Op{
		&driftplugin.OpAndroidPreActivityRegistrant{
			Base:   driftplugin.Base{Pkg: "github.com/foo/splash"},
			Symbol: "com.foo.splash.Android12SplashController.install",
		},
		&driftplugin.OpAndroidPreActivityRegistrant{
			Base:   driftplugin.Base{Pkg: "github.com/bar/other"},
			Symbol: "com.bar.other.OtherController.init",
		},
	}
	if _, err := WriteRegistrant(dir, "android", ops); err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "app/src/main/java/com/drift/runner/DriftPluginRegistrant.kt"))
	if err != nil {
		t.Fatalf("read registrant: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "com.foo.splash.Android12SplashController.install(activity)") {
		t.Errorf("preActivityCreate missing splash call:\n%s", s)
	}
	if !strings.Contains(s, "com.bar.other.OtherController.init(activity)") {
		t.Errorf("preActivityCreate missing other call:\n%s", s)
	}
	// Sorted order: bar < foo alphabetically.
	if strings.Index(s, "com.bar.other") > strings.Index(s, "com.foo.splash") {
		t.Errorf("preActivityCreate calls not in sorted order:\n%s", s)
	}
}

func TestWriteRegistrantWithIOSEntries(t *testing.T) {
	dir := t.TempDir()
	ops := []driftplugin.Op{
		&driftplugin.OpRegistrantIOS{Base: driftplugin.Base{Pkg: "p"}, Symbol: "FooPlugin.register"},
		&driftplugin.OpRegistrantIOS{Base: driftplugin.Base{Pkg: "p"}, Symbol: "BarPlugin.register"},
	}
	changed, err := WriteRegistrant(dir, "ios", ops)
	if err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}
	if len(changed) != 1 {
		t.Fatalf("expected 1 file changed, got %d", len(changed))
	}
	body, err := os.ReadFile(filepath.Join(dir, "Runner/DriftPluginRegistrant.swift"))
	if err != nil {
		t.Fatalf("read registrant: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "BarPlugin.register(host: host)") {
		t.Errorf("registrant missing BarPlugin call")
	}
	if !strings.Contains(s, "FooPlugin.register(host: host)") {
		t.Errorf("registrant missing FooPlugin call")
	}
	// Calls should be sorted for deterministic output.
	if strings.Index(s, "BarPlugin") > strings.Index(s, "FooPlugin") {
		t.Errorf("registrant calls not in sorted order: %s", s)
	}
}

func TestWriteRegistrantIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteRegistrant(dir, "android", nil); err != nil {
		t.Fatalf("first write: %v", err)
	}
	changed, err := WriteRegistrant(dir, "android", nil)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	if len(changed) != 0 {
		t.Errorf("expected zero changes on second write, got %v", changed)
	}
}

func TestEnsureRunnerSupportAndroidWritesHostAndHandler(t *testing.T) {
	dir := t.TempDir()
	changed, err := EnsureRunnerSupport(dir, "android")
	if err != nil {
		t.Fatalf("EnsureRunnerSupport: %v", err)
	}
	if len(changed) != len(AndroidRunnerSupportFiles) {
		t.Fatalf("expected %d files written (one per AndroidRunnerSupportFiles entry), got %d (%v)",
			len(AndroidRunnerSupportFiles), len(changed), changed)
	}
	host, err := os.ReadFile(filepath.Join(dir, "app/src/main/java/com/drift/runner/DriftPluginHost.kt"))
	if err != nil {
		t.Fatalf("read host: %v", err)
	}
	if !strings.Contains(string(host), "interface DriftPluginHost") {
		t.Errorf("host file does not declare the interface")
	}
	overlay, err := os.ReadFile(filepath.Join(dir, "app/src/main/java/com/drift/runner/DriftOverlayHost.kt"))
	if err != nil {
		t.Fatalf("read overlay host: %v", err)
	}
	if !strings.Contains(string(overlay), "interface DriftOverlayHost") {
		t.Errorf("overlay-host file does not declare the interface")
	}
}

// Guards against the original bug: EnsureRunnerSupport must write the exact
// bytes scaffold copies. Any drift between the embedded template and what
// EnsureRunnerSupport writes will cause the plugin pipeline to overwrite a
// freshly-scaffolded project on every build, breaking compilation.
func TestEnsureRunnerSupportMatchesTemplates(t *testing.T) {
	dir := t.TempDir()
	if _, err := EnsureRunnerSupport(dir, "android"); err != nil {
		t.Fatalf("EnsureRunnerSupport: %v", err)
	}
	for _, f := range AndroidRunnerSupportFiles {
		want, err := templates.ReadFile(f.TemplatePath)
		if err != nil {
			t.Fatalf("read template %s: %v", f.TemplatePath, err)
		}
		got, err := os.ReadFile(filepath.Join(dir, "app/src/main/java/com/drift/runner", f.Name))
		if err != nil {
			t.Fatalf("read written %s: %v", f.Name, err)
		}
		if !bytes.Equal(want, got) {
			t.Errorf("%s drift: scaffold and EnsureRunnerSupport must produce identical bytes.\nwant:\n%s\ngot:\n%s",
				f.Name, want, got)
		}
	}
}

// Guards against a specific class of drift: the MethodHandler interface must
// declare operator fun invoke, not fun handle. PlatformChannel.kt:155 calls
// handler(method, args) which requires the operator form. An earlier version
// of EnsureRunnerSupport silently overwrote the scaffold's operator-form
// template with a fun-handle constant, breaking every Android compile.
func TestMethodHandlerUsesOperatorInvoke(t *testing.T) {
	content, err := templates.ReadFile("android/runner/MethodHandler.kt")
	if err != nil {
		t.Fatalf("read MethodHandler.kt: %v", err)
	}
	if !strings.Contains(string(content), "operator fun invoke(") {
		t.Errorf("MethodHandler.kt must declare `operator fun invoke(...)` so PlatformChannel.kt's handler(method, args) call resolves; got:\n%s", content)
	}
}

func TestEnsureRunnerSupportIOSNoop(t *testing.T) {
	dir := t.TempDir()
	changed, err := EnsureRunnerSupport(dir, "ios")
	if err != nil {
		t.Fatalf("EnsureRunnerSupport: %v", err)
	}
	if len(changed) != 0 {
		t.Errorf("EnsureRunnerSupport ios should be no-op, got %v", changed)
	}
}
