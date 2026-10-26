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
	for _, want := range []string{"file.add", "ios.spm.add_package"} {
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
	want := protocol.EncodeContent([]byte(`{"kinds":["contacts"]}`))
	for i, loc := range []protocol.FileLocation{protocol.FileIOSPluginConfig, protocol.FileAndroidPluginConfig} {
		f, ok := ops[i].(*protocol.OpAddFile)
		if !ok || f.Location != loc || f.Content != want {
			t.Errorf("ops[%d] = %#v, want the config at %s", i, ops[i], loc)
		}
	}
}

// AddService and AddReceiver record one component op, and each refuses
// the other's element.
func TestManifestComponentsMatchTheirRecorder(t *testing.T) {
	ctx := NewTestCtx()
	ctx.Android.Manifest.AddService(`<service android:name="com.p.S" android:exported="false"/>`)
	ctx.Android.Manifest.AddReceiver(`<receiver android:name="com.p.R" android:exported="false"/>`)
	if err := ctx.Err(); err != nil {
		t.Fatal(err)
	}
	for i, tag := range []string{"service", "receiver"} {
		if c, ok := ctx.Ops()[i].(*protocol.OpAndroidManifestAddComponent); !ok || c.Tag() != tag {
			t.Errorf("ops[%d] = %#v, want a <%s>", i, ctx.Ops()[i], tag)
		}
	}

	ctx = NewTestCtx()
	ctx.Android.Manifest.AddService(`<receiver android:name="com.p.R" android:exported="false"/>`)
	ctx.Android.Manifest.AddReceiver(`<service android:name="com.p.S" android:exported="false"/>`)
	if err := ctx.Err(); err == nil || len(ctx.Ops()) != 0 {
		t.Errorf("Err() = %v, ops = %v; want both refused", err, ctx.Ops())
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
