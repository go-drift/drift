// Package plugin is a throwaway spike: it asks SwiftPM for firebase-ios-sdk
// and ships a Swift source that imports FirebaseCore and FirebaseMessaging,
// to test whether plugin Swift compiled into the app target can import
// modules that reach it only through the Drift/Plugins sidecar package.
package plugin

import (
	"embed"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

//go:embed ios
var iosSources embed.FS

type Config struct{}

type spike struct{}

func (spike) Name() string { return "firebasespike" }

func (spike) Build(ctx *driftplugin.BuildCtx, _ Config) error {
	ctx.IOS.AddPackageDependency(
		"https://github.com/firebase/firebase-ios-sdk",
		driftplugin.SPMRequirementFrom("11.0.0"),
		[]string{"FirebaseCore", "FirebaseMessaging"},
	)
	ctx.IOS.Sources.AddFS("firebasespike", iosSources, "ios")
	ctx.IOS.Registrant("FirebaseSpikePlugin.register")
	return nil
}

var Plugin driftplugin.Plugin[Config] = spike{}
