// Package plugin defines the Drift plugin authoring API.
//
// Plugins are Go modules that contribute build-time and runtime native
// behaviour to a Drift project. A plugin module exports a single typed Plugin
// value at <module>/plugin and is wired into a project via drift.yaml.
//
// The wire contract between the Drift CLI and the generated bridge binary
// (envelope, response, op types, config schema) lives in the protocol
// sub-package.
//
// The plugin guide (website-docs/guides/plugins.md) covers authoring.
// examples/plugins/hello is the smallest complete plugin, quoted by the
// guide; the first-party plugins under plugins/ (splash, firebase) are full
// references.
package plugin
