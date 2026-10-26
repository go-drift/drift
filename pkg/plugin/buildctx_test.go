package plugin

import (
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

func TestRecorderRejectsInvalidInputWithoutPanicking(t *testing.T) {
	ctx := NewTestCtx()
	ctx.Android.AddAppModuleFile("../escape.json", []byte("x"))
	ctx.IOS.AddPackageDependency("http://example.com/pkg", SPMRequirementFrom("1.0.0"), []string{"P"})
	ctx.IOS.Info.SetString("Valid", "kept")

	if got := len(ctx.Ops()); got != 1 {
		t.Fatalf("invalid ops must not be recorded: got %d ops, want 1", got)
	}
	err := ctx.Err()
	if err == nil {
		t.Fatal("expected Err() to report the invalid ops")
	}
	for _, want := range []string{"android.app_module.add_file", "ios.spm.add_package"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Err() = %q, want it to mention %s", err, want)
		}
	}
}

func TestRecorderErrIsNilWhenValid(t *testing.T) {
	ctx := NewTestCtx()
	ctx.Android.Resources.Colors.Set("brand", "#FFFFFF")
	ctx.Android.Resources.Strings.Set("greeting", "hi")
	if err := ctx.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
	if got := len(ctx.Ops()); got != 2 {
		t.Fatalf("got %d ops, want 2", got)
	}
}

func TestNativeConfig(t *testing.T) {
	ctx := NewTestCtx()
	ctx.NativeConfig(struct {
		Kinds []string `json:"kinds"`
	}{[]string{"contacts"}})
	if err := ctx.Err(); err != nil {
		t.Fatal(err)
	}
	ops := ctx.Ops()
	if len(ops) != 2 {
		t.Fatalf("ops = %v, want one per platform", ops)
	}
	ios, ok1 := ops[0].(*protocol.OpIOSPluginConfig)
	android, ok2 := ops[1].(*protocol.OpAndroidPluginConfig)
	if !ok1 || !ok2 || ios.Config != `{"kinds":["contacts"]}` || android.Config != ios.Config {
		t.Errorf("ops = %#v, %#v", ops[0], ops[1])
	}
}

func TestNativeConfigRejectsMisuse(t *testing.T) {
	for name, record := range map[string]func(*BuildCtx){
		"not an object": func(ctx *BuildCtx) { ctx.NativeConfig([]string{"a"}) },
		"nil":           func(ctx *BuildCtx) { ctx.NativeConfig(nil) },
		"unmarshalable": func(ctx *BuildCtx) { ctx.NativeConfig(map[string]any{"f": func() {}}) },
		"twice": func(ctx *BuildCtx) {
			ctx.NativeConfig(map[string]any{})
			ctx.NativeConfig(map[string]any{})
		},
	} {
		ctx := NewTestCtx()
		record(ctx)
		if ctx.Err() == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
