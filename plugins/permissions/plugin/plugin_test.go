package plugin

import (
	"slices"
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// build runs the plugin the way the bridge does (schema check, decode,
// Build), recording for every platform.
func build(t *testing.T, config string) ([]protocol.Op, error) {
	t.Helper()
	ctx := driftplugin.NewTestCtx()
	if err := driftplugin.Bind("github.com/go-drift/drift/plugins/permissions/plugin", Plugin).Build(ctx, []byte(config)); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx.Err: %v", err)
	}
	return ctx.Ops(), nil
}

func mustBuild(t *testing.T, config string) []protocol.Op {
	t.Helper()
	ops, err := build(t, config)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return ops
}

// Only the configured kinds are declared and registered.
func TestBuildDeclaresConfiguredKindsOnly(t *testing.T) {
	ops := mustBuild(t, "contacts:\n  description: Finds friends.\nphotos:\n  description: Picks a profile photo.\n")

	plist := map[string]string{}
	var perms []string
	var maxSDK = map[string]int{}
	var iosCfg, androidCfg string
	for _, op := range ops {
		switch o := op.(type) {
		case *protocol.OpPlistSet:
			if s, ok := o.Value.(string); ok {
				plist[o.Key] = s
			}
		case *protocol.OpAndroidManifestAddPermission:
			perms = append(perms, o.Name)
			maxSDK[o.Name] = o.MaxSDK
		case *protocol.OpAddFile:
			content, _ := protocol.DecodeContent(o.Content)
			switch o.Location {
			case protocol.FileIOSPluginConfig:
				iosCfg = string(content)
			case protocol.FileAndroidPluginConfig:
				androidCfg = string(content)
			}
		}
	}
	if plist["NSContactsUsageDescription"] != "Finds friends." || plist["NSPhotoLibraryUsageDescription"] != "Picks a profile photo." {
		t.Errorf("usage strings = %v", plist)
	}
	if _, ok := plist["NSMicrophoneUsageDescription"]; ok {
		t.Error("microphone usage string set without a microphone: section")
	}
	slices.Sort(perms)
	if want := []string{
		"android.permission.READ_CONTACTS",
		"android.permission.READ_EXTERNAL_STORAGE",
		"android.permission.READ_MEDIA_IMAGES",
	}; !slices.Equal(perms, want) {
		t.Errorf("permissions = %v, want %v", perms, want)
	}
	if maxSDK["android.permission.READ_EXTERNAL_STORAGE"] != 32 {
		t.Errorf("READ_EXTERNAL_STORAGE max SDK = %d, want 32", maxSDK["android.permission.READ_EXTERNAL_STORAGE"])
	}
	if want := `{"kinds":["contacts","photos"]}`; iosCfg != want || androidCfg != want {
		t.Errorf("native config = %s (iOS), %s (Android), want %s", iosCfg, androidCfg, want)
	}
}

// iOS 17 asks for calendar access with the full-access string.
func TestBuildCalendarSetsBothUsageStrings(t *testing.T) {
	ops := mustBuild(t, "calendar:\n  description: Adds events.\n")
	keys := map[string]bool{}
	for _, op := range ops {
		if o, ok := op.(*protocol.OpPlistSet); ok {
			keys[o.Key] = true
		}
	}
	if !keys["NSCalendarsFullAccessUsageDescription"] || !keys["NSCalendarsUsageDescription"] {
		t.Errorf("calendar usage strings = %v", keys)
	}
}

func TestBuildRejects(t *testing.T) {
	for config, want := range map[string]string{
		"":                                  "at least one",
		"contacts:\n  description: \" \"\n": "contacts.description",
		"contacts: {}\n":                    "description",
	} {
		if _, err := build(t, config); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("config %q: err = %v, want one mentioning %q", config, err, want)
		}
	}
}
