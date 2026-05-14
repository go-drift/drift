// Package plugin is a worked example of a Drift build-time plugin.
//
// It exists to show the authoring contract end to end: a typed Config with
// drift validation tags, a Build method that records ops on both platform
// surfaces, and a typed Plugin[Config] value the bridge can consume. The
// behaviour is deliberately trivial; treat this as the smallest thing that
// exercises every part of the API.
//
// Wire into a project by adding to drift.yaml:
//
//	plugins:
//	  - package: github.com/go-drift/drift/examples/plugins/demo/plugin
//	    config:
//	      greeting: "Hello from drift"
//	      shout: true
package plugin

import (
	"strings"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// Config is the typed shape of the drift.yaml `config:` block for this plugin.
// drift tags are read by the schema sub-command and double as documentation.
type Config struct {
	Greeting string `yaml:"greeting" drift:"required"`
	Shout    bool   `yaml:"shout"    drift:"default=false"`
}

type demo struct{}

func (demo) Name() string { return "demo" }

func (demo) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	message := cfg.Greeting
	if cfg.Shout {
		message = strings.ToUpper(message)
	}
	ctx.IOS.Info.SetString("DriftDemoGreeting", message)
	ctx.Android.Resources.Strings.Set("drift_demo_greeting", message)
	return nil
}

// Plugin is the binding consumed by the generated bridge in
// tools/drift-plugins/main.go. It must be a typed Plugin[Config] value so
// generic inference reaches the Config type parameter.
var Plugin driftplugin.Plugin[Config] = demo{}
