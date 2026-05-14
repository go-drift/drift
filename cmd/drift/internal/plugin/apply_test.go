package plugin

import (
	b64 "encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

func TestApplyWritesKotlinSources(t *testing.T) {
	dir := t.TempDir()
	body := "package com.foo.camera\nclass Stub {}\n"
	ops := []driftplugin.Op{
		&driftplugin.OpAddKotlinSource{
			Base:    driftplugin.Base{Pkg: "p"},
			Package: "com.foo.camera",
			RelPath: "Stub.kt",
			Content: base64(body),
		},
	}
	changed, err := Apply(ops, dir, "android")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	want := filepath.Join(dir, "app/src/main/java/com/foo/camera/Stub.kt")
	if len(changed) != 1 || changed[0] != want {
		t.Fatalf("changed = %v, want one path %q", changed, want)
	}
	got, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(got) != body {
		t.Errorf("body roundtrip: got %q want %q", got, body)
	}
}

func TestApplySkipsOtherPlatformOps(t *testing.T) {
	dir := t.TempDir()
	ops := []driftplugin.Op{
		&driftplugin.OpAndroidManifestAddPermission{Base: driftplugin.Base{Pkg: "p"}, Name: "android.permission.CAMERA"},
	}
	// Targeting iOS but op is android: should be skipped without error.
	if _, err := Apply(ops, dir, "ios"); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func TestDecodeOpsParsesJSONList(t *testing.T) {
	ops := []driftplugin.Op{
		&driftplugin.OpInfoPlistSetString{Base: driftplugin.Base{Pkg: "p"}, Key: "K", Value: "V"},
	}
	raw, err := driftplugin.MarshalOpList(ops)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(raw, &raws); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	decoded, err := DecodeOps(raws)
	if err != nil {
		t.Fatalf("DecodeOps: %v", err)
	}
	if len(decoded) != 1 {
		t.Errorf("expected 1 op, got %d", len(decoded))
	}
}

func TestDecodeOpsRejectsUnknown(t *testing.T) {
	raws := []json.RawMessage{[]byte(`{"type":"not.a.real.op"}`)}
	if _, err := DecodeOps(raws); err == nil || !strings.Contains(err.Error(), "not.a.real.op") {
		t.Errorf("expected unknown-op error, got %v", err)
	}
}

func base64(s string) string {
	return b64.StdEncoding.EncodeToString([]byte(s))
}
