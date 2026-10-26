package plugin

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points the cache root at a temporary directory, so bridges the
// tests build never land in the user's ~/.drift.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "drift-plugin-test-cache-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("DRIFT_CACHE_DIR", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
