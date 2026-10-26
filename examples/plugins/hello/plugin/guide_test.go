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
	guide, err := os.ReadFile("../../../../website-docs/guides/plugin-authoring.md")
	if err != nil {
		t.Fatal(err)
	}
	// The guide's walkthrough builds the plugin as example.com/hello, so
	// hello-app's quotes use that module path.
	const modulePath, guidePath = "github.com/go-drift/drift/examples/plugins/hello", "example.com/hello"
	quotes := []struct {
		file, start, end string
		walkthrough      bool
	}{
		{"plugin.go", "package plugin", "", false},
		{"ios/HelloPlugin.swift", "import DriftPluginAPI", "", false},
		{"android/HelloPlugin.kt", "package com.example.hello", "", false},
		{"../runtime/hello.go", "package runtime", "", false},
		{"plugin_test.go", "// build runs", "func TestBuildRegistersNativeClasses", false},
		{"../../hello-app/main.go", "package main", "", true},
		{"../../hello-app/drift.yaml", "plugins:", "", true},
	}
	for _, q := range quotes {
		src, err := os.ReadFile(q.file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(src)
		if q.walkthrough {
			text = strings.ReplaceAll(text, modulePath, guidePath)
		}
		i := strings.Index(text, q.start)
		if i < 0 {
			t.Fatalf("%s: no %q", q.file, q.start)
		}
		text = text[i:]
		if q.end != "" {
			text = text[:strings.Index(text, q.end)]
		}
		if !strings.Contains(string(guide), strings.TrimRight(text, "\n")+"\n```") {
			t.Errorf("website-docs/guides/plugin-authoring.md does not quote %s verbatim (from %q); update the guide", q.file, q.start)
		}
	}
}
