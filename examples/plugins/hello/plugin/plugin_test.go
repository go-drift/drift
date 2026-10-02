package plugin

import (
	"strings"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// build runs the plugin the way the bridge does (config checked against
// the schema, defaulted, decoded, then Build), recording for every
// platform.
func build(t *testing.T, config string) ([]protocol.Op, error) {
	t.Helper()
	ctx := driftplugin.NewTestCtx()
	if err := driftplugin.Bind("github.com/go-drift/drift/examples/plugins/hello/plugin", Plugin).
		Build(ctx, []byte(config)); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx.Err: %v", err)
	}
	return ctx.Ops(), nil
}

func TestBuildShipsGreeting(t *testing.T) {
	ops, err := build(t, "greeting: hello")
	if err != nil {
		t.Fatal(err)
	}
	var plist, resource string
	for _, op := range ops {
		switch o := op.(type) {
		case *protocol.OpPlistSetString:
			if o.Key == "HelloGreeting" {
				plist = o.Value
			}
		case *protocol.OpAndroidStringSet:
			if o.Name == "hello_greeting" {
				resource = o.Value
			}
		}
	}
	if plist != "hello" || resource != "hello" {
		t.Errorf("Info.plist greeting = %q, string resource = %q, want hello for both", plist, resource)
	}
}

func TestBuildRegistersNativeClasses(t *testing.T) {
	ops, err := build(t, "greeting: hello")
	if err != nil {
		t.Fatal(err)
	}
	var classes, sources []string
	for _, op := range ops {
		switch o := op.(type) {
		case *protocol.OpIOSPlugin:
			classes = append(classes, o.Class)
		case *protocol.OpAndroidPlugin:
			classes = append(classes, o.Class)
		case *protocol.OpAddIOSSource:
			sources = append(sources, o.RelPath)
		case *protocol.OpAddKotlinSource:
			sources = append(sources, o.RelPath)
		}
	}
	if got := strings.Join(classes, ","); got != "HelloPlugin,com.example.hello.HelloPlugin" {
		t.Errorf("plugin classes = %s", got)
	}
	if got := strings.Join(sources, ","); got != "HelloPlugin.swift,HelloPlugin.kt" {
		t.Errorf("sources = %s", got)
	}
}

func TestBuildRequiresGreeting(t *testing.T) {
	if _, err := build(t, "{}"); err == nil || !strings.Contains(err.Error(), "greeting") {
		t.Errorf("err = %v, want one naming greeting", err)
	}
}
