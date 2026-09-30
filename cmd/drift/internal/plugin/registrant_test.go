package plugin

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-drift/drift/cmd/drift/internal/templates"
	driftplugin "github.com/go-drift/drift/pkg/plugin"
	"github.com/go-drift/drift/pkg/plugin/protocol"
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
	ops := []protocol.Op{
		&protocol.OpAndroidPreActivityRegistrant{
			Base:   protocol.Base{Pkg: "github.com/foo/splash"},
			Symbol: "com.foo.splash.Android12SplashController.install",
		},
		&protocol.OpAndroidPreActivityRegistrant{
			Base:   protocol.Base{Pkg: "github.com/bar/other"},
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
	ops := []protocol.Op{
		&protocol.OpRegistrantIOS{Base: protocol.Base{Pkg: "p"}, Symbol: "FooPlugin.register"},
		&protocol.OpRegistrantIOS{Base: protocol.Base{Pkg: "p"}, Symbol: "BarPlugin.register"},
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

// Zero AppDelegate ops still produce all six callback methods (empty plugin
// fan-out, built-in dispatchers present). The AppDelegate template calls
// these unconditionally so they must exist.
func TestWriteRegistrantIOSAppDelegateMethodsAlwaysEmitted(t *testing.T) {
	dir := t.TempDir()
	if _, err := WriteRegistrant(dir, "ios", nil); err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "Runner/DriftPluginRegistrant.swift"))
	if err != nil {
		t.Fatalf("read registrant: %v", err)
	}
	s := string(body)
	requiredSignatures := []string{
		"static func didFinishLaunching(",
		"static func openURL(",
		"static func continueUserActivity(",
		"static func didRegisterForRemoteNotifications(",
		"static func didFailToRegisterForRemoteNotifications(",
		"static func didReceiveRemoteNotification(",
	}
	for _, sig := range requiredSignatures {
		if !strings.Contains(s, sig) {
			t.Errorf("expected %q in generated registrant:\n%s", sig, s)
		}
	}
	// Built-ins must run even with zero plugin entries.
	requiredBuiltins := []string{
		"NotificationHandler.start()",
		"DeepLinkHandler.handle(url: url, source: source)",
		"NotificationHandler.handleDeviceToken(deviceToken)",
		"NotificationHandler.handleRemoteNotificationError(error)",
		"NotificationHandler.handleRemoteNotification(userInfo, isForeground: application.applicationState == .active)",
	}
	for _, call := range requiredBuiltins {
		if !strings.Contains(s, call) {
			t.Errorf("expected built-in %q in registrant:\n%s", call, s)
		}
	}
	// Zero-plugins didReceiveRemoteNotification hands the coordinator an
	// empty handler list, which reports .newData (Drift's historical result).
	if !strings.Contains(s, "let handlers: [DriftPluginCoordinator.BackgroundFetchHandler] = []") {
		t.Errorf("zero-plugin handlers list should be empty:\n%s", s)
	}
	if !strings.Contains(s, "DriftPluginCoordinator.dispatchBackgroundFetch(") {
		t.Errorf("didReceiveRemoteNotification must delegate to dispatchBackgroundFetch:\n%s", s)
	}
	// No `var` in generated code: an unmutated var is a Swift warning.
	if strings.Contains(s, "var ") {
		t.Errorf("generated registrant should not declare vars:\n%s", s)
	}
	if !strings.Contains(s, "import UIKit") {
		t.Errorf("registrant missing import UIKit (needed for UIApplication, UIBackgroundFetchResult)")
	}
}

// One plugin per callback produces the call inside each method, sorted lex,
// with the right argument shape.
func TestWriteRegistrantIOSAppDelegateOneCallbackEach(t *testing.T) {
	dir := t.TempDir()
	ops := []protocol.Op{
		&protocol.OpIOSAppDelegateRegistrant{
			Base:     protocol.Base{Pkg: "github.com/foo/firebase"},
			Callback: protocol.IOSCallbackDidFinishLaunching,
			Symbol:   "FirebaseDriftPlugin.didFinishLaunching",
		},
		&protocol.OpIOSAppDelegateRegistrant{
			Base:     protocol.Base{Pkg: "github.com/bar/branch"},
			Callback: protocol.IOSCallbackOpenURL,
			Symbol:   "BranchDriftPlugin.openURL",
		},
	}
	if _, err := WriteRegistrant(dir, "ios", ops); err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "Runner/DriftPluginRegistrant.swift"))
	if err != nil {
		t.Fatalf("read registrant: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "FirebaseDriftPlugin.didFinishLaunching(application: application, launchOptions: launchOptions)") {
		t.Errorf("didFinishLaunching call missing or malformed:\n%s", s)
	}
	if !strings.Contains(s, "if BranchDriftPlugin.openURL(url: url) { return }") {
		t.Errorf("openURL plugin claim check missing:\n%s", s)
	}
}

// Two plugins on the same callback both appear, in lex order.
func TestWriteRegistrantIOSAppDelegateLexSorted(t *testing.T) {
	dir := t.TempDir()
	ops := []protocol.Op{
		&protocol.OpIOSAppDelegateRegistrant{
			Base:     protocol.Base{Pkg: "p"},
			Callback: protocol.IOSCallbackDidFinishLaunching,
			Symbol:   "ZetaPlugin.didFinishLaunching",
		},
		&protocol.OpIOSAppDelegateRegistrant{
			Base:     protocol.Base{Pkg: "p"},
			Callback: protocol.IOSCallbackDidFinishLaunching,
			Symbol:   "AlphaPlugin.didFinishLaunching",
		},
	}
	if _, err := WriteRegistrant(dir, "ios", ops); err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "Runner/DriftPluginRegistrant.swift"))
	if err != nil {
		t.Fatalf("read registrant: %v", err)
	}
	s := string(body)
	alpha := strings.Index(s, "AlphaPlugin.didFinishLaunching")
	zeta := strings.Index(s, "ZetaPlugin.didFinishLaunching")
	if alpha < 0 || zeta < 0 {
		t.Fatalf("both plugin calls must appear:\n%s", s)
	}
	if alpha > zeta {
		t.Errorf("plugin calls not lex-sorted (Alpha should precede Zeta):\n%s", s)
	}
}

// One plugin on didReceiveRemoteNotification produces a handlers array with
// one plugin closure (the built-in dispatch runs before it, outside the
// merge), delegating dispatch to the
// coordinator. The lock + timer + once-only scaffolding lives in
// DriftPluginCoordinator.swift (testable separately), not in codegen.
func TestWriteRegistrantIOSAppDelegateCoordinatorDelegates(t *testing.T) {
	dir := t.TempDir()
	ops := []protocol.Op{
		&protocol.OpIOSAppDelegateRegistrant{
			Base:     protocol.Base{Pkg: "p"},
			Callback: protocol.IOSCallbackDidReceiveRemoteNotification,
			Symbol:   "FCMDriftPlugin.didReceiveRemoteNotification",
		},
	}
	if _, err := WriteRegistrant(dir, "ios", ops); err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "Runner/DriftPluginRegistrant.swift"))
	if err != nil {
		t.Fatalf("read registrant: %v", err)
	}
	s := string(body)
	required := []string{
		"let handlers: [DriftPluginCoordinator.BackgroundFetchHandler] = [",
		"NotificationHandler.handleRemoteNotification(userInfo, isForeground: application.applicationState == .active)",
		"FCMDriftPlugin.didReceiveRemoteNotification(userInfo: userInfo, completion: completion)",
		"DriftPluginCoordinator.dispatchBackgroundFetch(",
	}
	for _, want := range required {
		if !strings.Contains(s, want) {
			t.Errorf("coordinator delegation missing %q:\n%s", want, s)
		}
	}
	// Race-condition surface must NOT be in codegen — it lives in the
	// hand-written coordinator file the swift-test harness exercises.
	forbidden := []string{
		"DispatchGroup()",
		"NSLock()",
		"perPluginFired_",
	}
	for _, bad := range forbidden {
		if strings.Contains(s, bad) {
			t.Errorf("codegen should delegate scaffold to coordinator, found %q in generated body:\n%s", bad, s)
		}
	}
}

// Two plugins on didReceiveRemoteNotification both appear as closures in
// the handlers array, in lex-sorted symbol order.
func TestWriteRegistrantIOSAppDelegateMultiplePluginsHandlers(t *testing.T) {
	dir := t.TempDir()
	ops := []protocol.Op{
		&protocol.OpIOSAppDelegateRegistrant{
			Base:     protocol.Base{Pkg: "p"},
			Callback: protocol.IOSCallbackDidReceiveRemoteNotification,
			Symbol:   "ZetaPlugin.didReceiveRemoteNotification",
		},
		&protocol.OpIOSAppDelegateRegistrant{
			Base:     protocol.Base{Pkg: "p"},
			Callback: protocol.IOSCallbackDidReceiveRemoteNotification,
			Symbol:   "AlphaPlugin.didReceiveRemoteNotification",
		},
	}
	if _, err := WriteRegistrant(dir, "ios", ops); err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "Runner/DriftPluginRegistrant.swift"))
	if err != nil {
		t.Fatalf("read registrant: %v", err)
	}
	s := string(body)
	alpha := strings.Index(s, "AlphaPlugin.didReceiveRemoteNotification(userInfo: userInfo, completion: completion)")
	zeta := strings.Index(s, "ZetaPlugin.didReceiveRemoteNotification(userInfo: userInfo, completion: completion)")
	if alpha < 0 || zeta < 0 {
		t.Fatalf("both plugin closures must appear in handlers array:\n%s", s)
	}
	if alpha > zeta {
		t.Errorf("plugin closures not lex-sorted (Alpha should precede Zeta):\n%s", s)
	}
}

// xtool writes to Sources/Runner/, not Runner/. Same body shape.
func TestWriteRegistrantXtoolAppDelegateMethods(t *testing.T) {
	dir := t.TempDir()
	ops := []protocol.Op{
		&protocol.OpIOSAppDelegateRegistrant{
			Base:     protocol.Base{Pkg: "p"},
			Callback: protocol.IOSCallbackOpenURL,
			Symbol:   "BranchDriftPlugin.openURL",
		},
	}
	if _, err := WriteRegistrant(dir, "xtool", ops); err != nil {
		t.Fatalf("WriteRegistrant xtool: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "Sources/Runner/DriftPluginRegistrant.swift"))
	if err != nil {
		t.Fatalf("read registrant: %v", err)
	}
	if !strings.Contains(string(body), "if BranchDriftPlugin.openURL(url: url) { return }") {
		t.Errorf("xtool registrant missing openURL plugin call:\n%s", body)
	}
}

// The placeholder DriftPluginRegistrant.swift template must be byte-identical
// to what writeIOSRegistrant emits for zero ops. Otherwise every plugin sync
// rewrites the file on a fresh scaffold and produces a spurious diff.
func TestIOSRegistrantPlaceholderMatchesEmptyCodegen(t *testing.T) {
	tmpl, err := templates.ReadFile("ios/DriftPluginRegistrant.swift")
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	dir := t.TempDir()
	if _, err := WriteRegistrant(dir, "ios", nil); err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}
	generated, err := os.ReadFile(filepath.Join(dir, "Runner/DriftPluginRegistrant.swift"))
	if err != nil {
		t.Fatalf("read generated: %v", err)
	}
	if !bytes.Equal(tmpl, generated) {
		t.Errorf("placeholder template and empty-input codegen drift; fix the template so plugin sync stays idempotent on a fresh scaffold.\n--- template ---\n%s\n--- generated ---\n%s", tmpl, generated)
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

// CheckEjectedIOS looks for the generated-registrant call sites that the
// current templates contain. If a template stops calling one, ejected
// checks would demand wiring the templates themselves lack.
func TestIOSCallbackCallSitesPresentInTemplates(t *testing.T) {
	for _, cb := range protocol.IOSAppDelegateCallbacks {
		file, call := iosCallbackCallSite(cb)
		body, err := templates.ReadFile("ios/" + file)
		if err != nil {
			t.Fatalf("read ios/%s: %v", file, err)
		}
		if !strings.Contains(string(body), call) {
			t.Errorf("ios/%s does not call %s", file, call)
		}
	}
}

func TestCheckEjectedIOS(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A project ejected before plugin hooks existed.
	mustWrite("Runner/AppDelegate.swift", "NotificationHandler.start()\n")
	mustWrite("Runner/SceneDelegate.swift", "DeepLinkHandler.handle(url: url, source: \"open_url\")\n")
	mustWrite("Runner.xcodeproj/project.pbxproj", "// no package refs\n")

	base := protocol.Base{Pkg: "github.com/acme/signin"}
	ops := []protocol.Op{
		&protocol.OpIOSAppDelegateRegistrant{Base: base, Callback: protocol.IOSCallbackOpenURL, Symbol: "SignIn.openURL"},
		&protocol.OpIOSAddPackageDependency{Base: base, URL: "https://github.com/google/GoogleSignIn-iOS", Requirement: driftplugin.SPMRequirementFrom("7.0.0"), Products: []string{"GoogleSignIn"}},
	}
	err := CheckEjectedIOS(dir, ops)
	if err == nil {
		t.Fatalf("expected wiring error")
	}
	for _, want := range []string{"SceneDelegate.swift", "DriftPluginRegistrant.openURL(", "github.com/acme/signin", "Drift/Plugins"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q:\n%v", want, err)
		}
	}

	// Current templates are fully wired.
	scene, _ := templates.ReadFile("ios/SceneDelegate.swift")
	mustWrite("Runner/SceneDelegate.swift", string(scene))
	pbx, _ := templates.ReadFile("xcodeproj/project.pbxproj.tmpl")
	mustWrite("Runner.xcodeproj/project.pbxproj", string(pbx))
	if err := CheckEjectedIOS(dir, ops); err != nil {
		t.Errorf("template-wired project should pass: %v", err)
	}
	// No plugin features in use: nothing to check.
	mustWrite("Runner/SceneDelegate.swift", "")
	if err := CheckEjectedIOS(dir, nil); err != nil {
		t.Errorf("no ops should pass: %v", err)
	}
}

// Ejected projects predate DriftPluginCoordinator; EnsureRunnerSupport must
// supply it next to the generated registrant, byte-identical to the template.
func TestEnsureRunnerSupportIOSWritesCoordinator(t *testing.T) {
	for platform, rel := range map[string]string{"ios": "Runner", "xtool": "Sources/Runner"} {
		dir := t.TempDir()
		if _, err := EnsureRunnerSupport(dir, platform); err != nil {
			t.Fatalf("%s: %v", platform, err)
		}
		got, err := os.ReadFile(filepath.Join(dir, rel, "DriftPluginCoordinator.swift"))
		if err != nil {
			t.Fatalf("%s: %v", platform, err)
		}
		want, _ := templates.ReadFile("ios/DriftPluginCoordinator.swift")
		if !bytes.Equal(got, want) {
			t.Errorf("%s: coordinator differs from template", platform)
		}
		changed, err := EnsureRunnerSupport(dir, platform)
		if err != nil || len(changed) != 0 {
			t.Errorf("%s: rerun should be a no-op, got %v, %v", platform, changed, err)
		}
	}
}

// Plugins get first refusal on a URL, in lex order; Drift's deep-link
// channel only sees it when no plugin claims it.
func TestWriteRegistrantIOSOpenURLClaimOrder(t *testing.T) {
	dir := t.TempDir()
	ops := []protocol.Op{
		&protocol.OpIOSAppDelegateRegistrant{Base: protocol.Base{Pkg: "p"}, Callback: protocol.IOSCallbackOpenURL, Symbol: "ZetaPlugin.openURL"},
		&protocol.OpIOSAppDelegateRegistrant{Base: protocol.Base{Pkg: "p"}, Callback: protocol.IOSCallbackOpenURL, Symbol: "AlphaPlugin.openURL"},
		&protocol.OpIOSAppDelegateRegistrant{Base: protocol.Base{Pkg: "p"}, Callback: protocol.IOSCallbackContinueUserActivity, Symbol: "AlphaPlugin.continueUserActivity"},
	}
	if _, err := WriteRegistrant(dir, "ios", ops); err != nil {
		t.Fatalf("WriteRegistrant: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "Runner/DriftPluginRegistrant.swift"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	wantOpenURL := "    static func openURL(_ url: URL, source: String) {\n" +
		"        if AlphaPlugin.openURL(url: url) { return }\n" +
		"        if ZetaPlugin.openURL(url: url) { return }\n" +
		"        DeepLinkHandler.handle(url: url, source: source)\n" +
		"    }\n"
	if !strings.Contains(s, wantOpenURL) {
		t.Errorf("openURL dispatch wrong, want:\n%s\ngot:\n%s", wantOpenURL, s)
	}
	wantActivity := "    static func continueUserActivity(_ userActivity: NSUserActivity, source: String) {\n" +
		"        if AlphaPlugin.continueUserActivity(userActivity: userActivity) { return }\n" +
		"        if let url = userActivity.webpageURL {\n" +
		"            DeepLinkHandler.handle(url: url, source: source)\n" +
		"        }\n" +
		"    }\n"
	if !strings.Contains(s, wantActivity) {
		t.Errorf("continueUserActivity dispatch wrong, want:\n%s\ngot:\n%s", wantActivity, s)
	}
}
