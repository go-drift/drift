package plugin

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// build runs the plugin the way the bridge does (schema check, decode,
// Build) for platform.
func build(t *testing.T, platform, config string) ([]protocol.Op, error) {
	t.Helper()
	ctx := driftplugin.NewTestCtxFor(t.TempDir(), platform)
	if err := driftplugin.Bind("github.com/go-drift/drift/plugins/location/plugin", Plugin).Build(ctx, []byte(config)); err != nil {
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

const whenInUse = "when_in_use_description: Finds stores near you.\n"

const withAlways = whenInUse + "always:\n  description: Tracks runs in the background.\n"

func plistStrings(ops []protocol.Op) map[string]string {
	keys := map[string]string{}
	for _, op := range ops {
		if o, ok := op.(*protocol.OpPlistSet); ok && o.File == protocol.PlistInfo {
			if s, ok := o.Value.(string); ok {
				keys[o.Key] = s
			}
		}
	}
	return keys
}

func permissions(ops []protocol.Op) []string {
	var names []string
	for _, op := range ops {
		if o, ok := op.(*protocol.OpAndroidManifestAddPermission); ok {
			names = append(names, o.Name)
		}
	}
	return names
}

func TestBuildIOS(t *testing.T) {
	ops := mustBuild(t, "ios", whenInUse)
	keys := plistStrings(ops)
	if keys["NSLocationWhenInUseUsageDescription"] != "Finds stores near you." {
		t.Errorf("when-in-use usage string = %q", keys["NSLocationWhenInUseUsageDescription"])
	}
	if _, ok := keys["NSLocationAlwaysAndWhenInUseUsageDescription"]; ok {
		t.Error("Always usage string set without always:")
	}
	if !slices.ContainsFunc(ops, func(op protocol.Op) bool {
		o, ok := op.(*protocol.OpIOSPlugin)
		return ok && o.Class == "DriftLocationPlugin"
	}) {
		t.Error("iOS plugin class not registered")
	}
}

func TestBuildAndroid(t *testing.T) {
	ops := mustBuild(t, "android", whenInUse)
	if got := permissions(ops); !slices.Equal(got, []string{
		"android.permission.ACCESS_FINE_LOCATION",
		"android.permission.ACCESS_COARSE_LOCATION",
	}) {
		t.Errorf("permissions = %v, want fine and coarse only", got)
	}
	if !slices.ContainsFunc(ops, func(op protocol.Op) bool {
		o, ok := op.(*protocol.OpAndroidGradleAddDependency)
		return ok && o.Coord == playServicesLocation
	}) {
		t.Error("play-services-location dependency missing")
	}
	if !slices.ContainsFunc(ops, func(op protocol.Op) bool {
		o, ok := op.(*protocol.OpAndroidPlugin)
		return ok && o.Class == androidPackage+".DriftLocationPlugin"
	}) {
		t.Error("Android plugin class not registered")
	}
}

func backgroundModes(ops []protocol.Op) []string {
	var modes []string
	for _, op := range ops {
		if o, ok := op.(*protocol.OpPlistAppendArrayItem); ok && o.File == protocol.PlistInfo && o.Key == "UIBackgroundModes" {
			modes = append(modes, o.Item())
		}
	}
	return modes
}

func services(ops []protocol.Op) []string {
	var xml []string
	for _, op := range ops {
		if o, ok := op.(*protocol.OpAndroidManifestAddComponent); ok && o.Tag() == "service" {
			xml = append(xml, o.XML)
		}
	}
	return xml
}

// androidConfig decodes the native config the Android build ships.
func androidConfig(t *testing.T, ops []protocol.Op) nativeConfig {
	t.Helper()
	for _, op := range ops {
		if o, ok := op.(*protocol.OpAddFile); ok && o.Location == protocol.FileAndroidPluginConfig {
			data, err := protocol.DecodeContent(o.Content)
			if err != nil {
				t.Fatal(err)
			}
			var cfg nativeConfig
			if err := json.Unmarshal(data, &cfg); err != nil {
				t.Fatal(err)
			}
			return cfg
		}
	}
	t.Fatal("no native config")
	return nativeConfig{}
}

// Without always:, nothing for the background is declared (App Review,
// Google Play policy).
func TestBuildWithoutAlwaysDeclaresNoBackground(t *testing.T) {
	if got := backgroundModes(mustBuild(t, "ios", whenInUse)); len(got) != 0 {
		t.Errorf("UIBackgroundModes = %v, want none", got)
	}
	android := mustBuild(t, "android", whenInUse)
	if got := services(android); len(got) != 0 {
		t.Errorf("services = %v, want none", got)
	}
	if got := androidConfig(t, android); got != (nativeConfig{}) {
		t.Errorf("native config = %+v, want none of the background's", got)
	}
}

// Always adds the Always usage string and the location background mode on
// iOS: background updates need both, and the native half checks for them.
func TestBuildAlwaysIOS(t *testing.T) {
	ops := mustBuild(t, "ios", withAlways)
	if got := plistStrings(ops)["NSLocationAlwaysAndWhenInUseUsageDescription"]; got != "Tracks runs in the background." {
		t.Errorf("Always usage string = %q", got)
	}
	// Appended, so other plugins' modes (remote-notification) merge.
	if got := backgroundModes(ops); !slices.Equal(got, []string{"location"}) {
		t.Errorf("UIBackgroundModes = %v, want [location]", got)
	}
}

// Always adds, on Android, the background permission and the foreground
// service that keeps background updates running, with the permissions
// Android 14+ needs to start it, and its notification's text in the native
// config.
func TestBuildAlwaysAndroid(t *testing.T) {
	ops := mustBuild(t, "android", withAlways)
	if got := permissions(ops); !slices.Equal(got, []string{
		"android.permission.ACCESS_FINE_LOCATION",
		"android.permission.ACCESS_COARSE_LOCATION",
		"android.permission.ACCESS_BACKGROUND_LOCATION",
		"android.permission.FOREGROUND_SERVICE",
		"android.permission.FOREGROUND_SERVICE_LOCATION",
		"android.permission.POST_NOTIFICATIONS",
	}) {
		t.Errorf("permissions = %v", got)
	}
	svc := services(ops)
	if len(svc) != 1 ||
		!strings.Contains(svc[0], `android:name="com.drift.plugin.location.DriftLocationService"`) ||
		!strings.Contains(svc[0], `android:foregroundServiceType="location"`) ||
		!strings.Contains(svc[0], `android:exported="false"`) {
		t.Errorf("services = %v, want DriftLocationService of type location, not exported", svc)
	}
	if got := androidConfig(t, ops); got.NotificationTitle != "Using your location" ||
		got.NotificationText != "Location updates continue while the app is in the background." {
		t.Errorf("default notification = %+v", got)
	}
}

func TestBuildAlwaysNotificationText(t *testing.T) {
	config := withAlways + "  notification_title: Recording your run\n  notification_text: You're on mile 3.\n"
	got := androidConfig(t, mustBuild(t, "android", config))
	if got.NotificationTitle != "Recording your run" || got.NotificationText != "You're on mile 3." {
		t.Errorf("notification = %+v", got)
	}
}

func TestBuildRejectsEmptyDescriptions(t *testing.T) {
	for config, key := range map[string]string{
		"when_in_use_description: \"  \"\n":          "when_in_use_description",
		whenInUse + "always:\n  description: \"\"\n": "always.description",
		withAlways + "  notification_title: \" \"\n": "always.notification_title",
		withAlways + "  notification_text: \"\"\n":   "always.notification_text",
	} {
		if _, err := build(t, "ios", config); err == nil || !strings.Contains(err.Error(), key) {
			t.Errorf("config %q: err = %v, want one naming %s", config, err, key)
		}
	}
	if _, err := build(t, "ios", ""); err == nil || !strings.Contains(err.Error(), "when_in_use_description") {
		t.Errorf("missing description: err = %v", err)
	}
}

// xtool (iOS development on Linux) gets the iOS integration.
func TestBuildXtoolEmitsIOS(t *testing.T) {
	xtool, err := protocol.MarshalOpList(mustBuild(t, "xtool", withAlways))
	if err != nil {
		t.Fatal(err)
	}
	ios, err := protocol.MarshalOpList(mustBuild(t, "ios", withAlways))
	if err != nil {
		t.Fatal(err)
	}
	if string(xtool) != string(ios) {
		t.Errorf("xtool ops differ from iOS ops:\nxtool: %s\nios:   %s", xtool, ios)
	}
}
