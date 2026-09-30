package plugin

import (
	"strings"
	"testing"
)

func TestRecorderRejectsInvalidInputWithoutPanicking(t *testing.T) {
	ctx := NewTestCtx()
	ctx.Android.AddAsset("../escape.bin", []byte("x"))
	ctx.IOS.AddPackageDependency("http://example.com/pkg", SPMRequirementFrom("1.0.0"), []string{"P"})
	ctx.IOS.Info.SetString("Valid", "kept")

	if got := len(ctx.Ops()); got != 1 {
		t.Fatalf("invalid ops must not be recorded: got %d ops, want 1", got)
	}
	err := ctx.Err()
	if err == nil {
		t.Fatal("expected Err() to report the invalid ops")
	}
	for _, want := range []string{"android.assets.add", "ios.spm.add_package"} {
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
