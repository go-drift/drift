package plugin

import (
	"os"
	"strings"
	"testing"
)

// The plugin guide quotes this plugin's code; keep the quotes verbatim.
// Each file is quoted from its first line of code (start) to its end; a
// test file is quoted from start up to end.
func TestGuideQuotesThisPlugin(t *testing.T) {
	guide, err := os.ReadFile("../../../../website-docs/guides/plugins.md")
	if err != nil {
		t.Fatal(err)
	}
	quotes := []struct{ file, start, end string }{
		{"plugin.go", "package plugin", ""},
		{"ios/HelloPlugin.swift", "import DriftPluginAPI", ""},
		{"android/HelloPlugin.kt", "package com.example.hello", ""},
		{"../runtime/hello.go", "package runtime", ""},
		{"plugin_test.go", "// build runs", "func TestBuildRegistersNativeClasses"},
	}
	for _, q := range quotes {
		src, err := os.ReadFile(q.file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		i := strings.Index(text, q.start)
		if i < 0 {
			t.Fatalf("%s: no %q", q.file, q.start)
		}
		text = text[i:]
		if q.end != "" {
			text = text[:strings.Index(text, q.end)]
		}
		if !strings.Contains(string(guide), strings.TrimRight(text, "\n")+"\n```") {
			t.Errorf("website-docs/guides/plugins.md does not quote %s verbatim (from %q); update the guide", q.file, q.start)
		}
	}
}
