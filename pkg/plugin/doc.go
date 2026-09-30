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
// A worked example lives at examples/plugins/demo/plugin; it ships as a
// sub-module so the parent test suite does not depend on it.
package plugin
