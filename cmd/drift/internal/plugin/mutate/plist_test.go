package mutate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
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

func info(key string) protocol.PlistEntry {
	return protocol.PlistEntry{File: protocol.PlistInfo, Key: key}
}

func plistOps[T protocol.PlistOp](ops []T) []protocol.PlistOp {
	out := make([]protocol.PlistOp, len(ops))
	for i, op := range ops {
		out[i] = op
	}
	return out
}

func TestApplyPlistSetString(t *testing.T) {
	path := writePlist(t)
	ops := []*protocol.OpPlistSetString{
		{Base: protocol.Base{Pkg: "p"}, PlistEntry: info("NSCameraUsageDescription"), Value: "Take photos"},
	}
	changed, err := ApplyPlist(path, plistOps(ops))
	if err != nil {
		t.Fatalf("ApplyPlist: %v", err)
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

func TestApplyPlistIdempotent(t *testing.T) {
	path := writePlist(t)
	ops := []*protocol.OpPlistSetString{
		{Base: protocol.Base{Pkg: "p"}, PlistEntry: info("NSCameraUsageDescription"), Value: "Take photos"},
	}
	if _, err := ApplyPlist(path, plistOps(ops)); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	changed, err := ApplyPlist(path, plistOps(ops))
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if changed {
		t.Errorf("expected idempotent re-apply, got changed=true")
	}
}

func TestApplyPlistAppendArrayItem(t *testing.T) {
	path := writePlist(t)
	ops := []*protocol.OpPlistAppendArrayItem{
		{Base: protocol.Base{Pkg: "a"}, PlistEntry: info("Schemes"), Value: "myapp"},
		{Base: protocol.Base{Pkg: "b"}, PlistEntry: info("Schemes"), Value: "other"},
		{Base: protocol.Base{Pkg: "c"}, PlistEntry: info("Schemes"), Value: "myapp"}, // dedupe
	}
	if _, err := ApplyPlist(path, plistOps(ops)); err != nil {
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

func TestApplyPlistSetBool(t *testing.T) {
	path := writePlist(t)
	ops := []*protocol.OpPlistSetBool{
		{Base: protocol.Base{Pkg: "p"}, PlistEntry: info("MyFlag"), Value: true},
	}
	if _, err := ApplyPlist(path, plistOps(ops)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "<key>MyFlag</key>") {
		t.Errorf("missing MyFlag: %s", body)
	}
}

// Appends extend an array another plugin set, whatever the op order.
func TestApplyPlistAppendsAfterSet(t *testing.T) {
	path := writePlist(t)
	ops := []protocol.PlistOp{
		&protocol.OpPlistAppendArrayItem{Base: protocol.Base{Pkg: "a"}, PlistEntry: info("Modes"), Value: "remote-notification"},
		&protocol.OpPlistSetStringArray{Base: protocol.Base{Pkg: "b"}, PlistEntry: info("Modes"), Values: []string{"fetch"}},
	}
	if _, err := ApplyPlist(path, ops); err != nil {
		t.Fatalf("apply: %v", err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "<string>fetch</string>") || !strings.Contains(string(body), "<string>remote-notification</string>") {
		t.Errorf("want both fetch and remote-notification: %s", body)
	}
}

// Appending to a key holding something other than an array is an error,
// not a silent overwrite of the user's value.
func TestApplyPlistAppendToNonArrayFails(t *testing.T) {
	path := writePlist(t)
	ops := []protocol.PlistOp{
		&protocol.OpPlistAppendArrayItem{Base: protocol.Base{Pkg: "a"}, PlistEntry: info("CFBundleName"), Value: "x"},
	}
	_, err := ApplyPlist(path, ops)
	if err == nil || !strings.Contains(err.Error(), "CFBundleName") {
		t.Fatalf("err = %v, want one naming CFBundleName", err)
	}
	body, _ := os.ReadFile(path)
	if string(body) != basePlist {
		t.Errorf("plist changed despite the error: %s", body)
	}
}
