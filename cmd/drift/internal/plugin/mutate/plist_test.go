package mutate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

const basePlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>Showcase</string>
	<key>CFBundleIdentifier</key>
	<string>com.example.showcase</string>
	<key>NSAppTransportSecurity</key>
	<dict>
		<key>NSAllowsArbitraryLoads</key>
		<true/>
	</dict>
</dict>
</plist>
`

func writePlist(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "Info.plist")
	if err := os.WriteFile(path, []byte(basePlist), 0o644); err != nil {
		t.Fatalf("seed plist: %v", err)
	}
	return path
}

func TestApplyInfoPlistSetString(t *testing.T) {
	path := writePlist(t)
	ops := []*driftplugin.OpInfoPlistSetString{
		{Base: driftplugin.Base{Pkg: "p"}, Key: "NSCameraUsageDescription", Value: "Take photos"},
	}
	changed, err := ApplyInfoPlist(path, ops, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("ApplyInfoPlist: %v", err)
	}
	if !changed {
		t.Errorf("expected changed=true")
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "NSCameraUsageDescription") {
		t.Errorf("plist did not pick up new key: %s", body)
	}
	if !strings.Contains(string(body), "CFBundleName") {
		t.Errorf("plist dropped existing user key: %s", body)
	}
}

func TestApplyInfoPlistIdempotent(t *testing.T) {
	path := writePlist(t)
	ops := []*driftplugin.OpInfoPlistSetString{
		{Base: driftplugin.Base{Pkg: "p"}, Key: "NSCameraUsageDescription", Value: "Take photos"},
	}
	if _, err := ApplyInfoPlist(path, ops, nil, nil, nil, nil); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	changed, err := ApplyInfoPlist(path, ops, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if changed {
		t.Errorf("expected idempotent re-apply, got changed=true")
	}
}

func TestApplyInfoPlistAppendArrayItem(t *testing.T) {
	path := writePlist(t)
	ops := []*driftplugin.OpInfoPlistAppendArrayItem{
		{Base: driftplugin.Base{Pkg: "a"}, Key: "Schemes", Value: "myapp"},
		{Base: driftplugin.Base{Pkg: "b"}, Key: "Schemes", Value: "other"},
		{Base: driftplugin.Base{Pkg: "c"}, Key: "Schemes", Value: "myapp"}, // dedupe
	}
	if _, err := ApplyInfoPlist(path, nil, nil, nil, ops, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}
	body, _ := os.ReadFile(path)
	if strings.Count(string(body), "<string>myapp</string>") != 1 {
		t.Errorf("expected single myapp entry: %s", body)
	}
	if !strings.Contains(string(body), "<string>other</string>") {
		t.Errorf("missing other entry: %s", body)
	}
}

func TestApplyInfoPlistSetBool(t *testing.T) {
	path := writePlist(t)
	ops := []*driftplugin.OpInfoPlistSetBool{
		{Base: driftplugin.Base{Pkg: "p"}, Key: "MyFlag", Value: true},
	}
	if _, err := ApplyInfoPlist(path, nil, ops, nil, nil, nil); err != nil {
		t.Fatalf("apply: %v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "<key>MyFlag</key>") {
		t.Errorf("missing MyFlag: %s", body)
	}
}
