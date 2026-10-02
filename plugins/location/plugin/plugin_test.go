package plugin

import (
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
		if o, ok := op.(*protocol.OpPlistSetString); ok && o.File == protocol.PlistInfo {
			keys[o.Key] = o.Value
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

// Always adds the background permission and its usage string; nothing
// else does (Google Play policy).
func TestBuildAlways(t *testing.T) {
	ios := plistStrings(mustBuild(t, "ios", withAlways))
	if ios["NSLocationAlwaysAndWhenInUseUsageDescription"] != "Tracks runs in the background." {
		t.Errorf("Always usage string = %q", ios["NSLocationAlwaysAndWhenInUseUsageDescription"])
	}
	if got := permissions(mustBuild(t, "android", withAlways)); !slices.Contains(got, "android.permission.ACCESS_BACKGROUND_LOCATION") {
		t.Errorf("permissions = %v, want ACCESS_BACKGROUND_LOCATION", got)
	}
}

func TestBuildRejectsEmptyDescriptions(t *testing.T) {
	for _, config := range []string{
		"when_in_use_description: \"  \"\n",
		whenInUse + "always:\n  description: \"\"\n",
	} {
		if _, err := build(t, "ios", config); err == nil || !strings.Contains(err.Error(), "description") {
			t.Errorf("config %q: err = %v, want one naming the description", config, err)
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
