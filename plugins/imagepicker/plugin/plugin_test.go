package plugin

import (
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

func build(t *testing.T, config string) ([]protocol.Op, error) {
	t.Helper()
	ctx := driftplugin.NewTestCtx()
	if err := driftplugin.Bind("github.com/go-drift/drift/plugins/imagepicker/plugin", Plugin).Build(ctx, []byte(config)); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx.Err: %v", err)
	}
	return ctx.Ops(), nil
}

func cameraUsage(ops []protocol.Op) (string, bool) {
	for _, op := range ops {
		if o, ok := op.(*protocol.OpPlistSetString); ok && o.Key == "NSCameraUsageDescription" {
			return o.Value, true
		}
	}
	return "", false
}

func TestBuildWithCamera(t *testing.T) {
	ops, err := build(t, "camera:\n  description: Takes your profile photo.\n")
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := cameraUsage(ops); !ok || got != "Takes your profile photo." {
		t.Errorf("camera usage string = %q, %v", got, ok)
	}
	for _, op := range ops {
		if o, ok := op.(*protocol.OpAndroidManifestAddPermission); ok {
			t.Errorf("declared %s; the system camera and photo pickers need no permission", o.Name)
		}
	}
}

// The library picker needs no usage string.
func TestBuildWithoutCamera(t *testing.T) {
	ops, err := build(t, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cameraUsage(ops); ok {
		t.Error("camera usage string set without camera:")
	}
}

func TestBuildRejectsEmptyDescription(t *testing.T) {
	if _, err := build(t, "camera:\n  description: \" \"\n"); err == nil || !strings.Contains(err.Error(), "camera.description") {
		t.Errorf("err = %v", err)
	}
}
