package workspace

import (
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// Refresh regenerates the native project exactly when the plugin output
// hash changes, so the hash must be stable for equal op lists and differ
// for any change, including dropping every plugin.
func TestPluginOpsHash(t *testing.T) {
	mk := func(ops ...protocol.Op) string {
		t.Helper()
		p, err := newPluginOps(ops)
		if err != nil {
			t.Fatal(err)
		}
		return p.hash
	}
	perm := func(name string) protocol.Op {
		return &protocol.OpAndroidManifestAddPermission{Base: protocol.Base{Pkg: "p"}, Name: name}
	}

	none := mk()
	if none != mk() {
		t.Error("empty op list hash is not stable")
	}
	one := mk(perm("android.permission.CAMERA"))
	if one != mk(perm("android.permission.CAMERA")) {
		t.Error("equal op lists hash differently")
	}
	if one == none {
		t.Error("adding an op did not change the hash")
	}
	if one == mk(perm("android.permission.INTERNET")) {
		t.Error("changing an op did not change the hash")
	}
}
