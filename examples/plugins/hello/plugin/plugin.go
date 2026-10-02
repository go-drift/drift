// Package plugin is the build half of the hello plugin: the smallest
// complete Drift plugin, quoted by the plugin guide
// (website-docs/guides/plugins.md). Copy it to start a new plugin.
//
// It takes a greeting from drift.yaml and ships native code that returns
// it to Go:
//
//	plugins:
//	  - package: github.com/go-drift/drift/examples/plugins/hello/plugin
//	    config:
//	      greeting: Hello from drift.yaml
//
// The Go API is github.com/go-drift/drift/examples/plugins/hello/runtime.
package plugin

import (
	"embed"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

//go:embed ios
var iosSources embed.FS

//go:embed android
var androidSources embed.FS

// Config is the plugin's drift.yaml config block.
type Config struct {
	Greeting string `yaml:"greeting" drift:"required"`
}

type hello struct{}

func (hello) Name() string { return "hello" }

func (hello) Build(ctx *driftplugin.BuildCtx, cfg Config) error {
	// iOS: the greeting goes in Info.plist; HelloPlugin.swift reads it.
	ctx.IOS.Info.SetString("HelloGreeting", cfg.Greeting)
	ctx.IOS.Sources.AddFS("Hello", iosSources, "ios")
	ctx.IOS.Plugin("HelloPlugin")

	// Android: the greeting is a string resource; HelloPlugin.kt reads it.
	ctx.Android.Resources.Strings.Set("hello_greeting", cfg.Greeting)
	ctx.Android.Sources.AddFS("com.example.hello", androidSources, "android")
	ctx.Android.Plugin("com.example.hello.HelloPlugin")
	return nil
}

// Plugin is the value the generated bridge binds: this exact name, typed
// as driftplugin.Plugin[Config].
var Plugin driftplugin.Plugin[Config] = hello{}
