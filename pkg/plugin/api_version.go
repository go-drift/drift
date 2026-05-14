// Package plugin defines the Drift plugin authoring API.
//
// Plugins are Go modules that contribute build-time and runtime native
// behaviour to a Drift project. A plugin module exports a single typed Plugin
// value at <module>/plugin and is wired into a project via drift.yaml.
//
// A worked example lives at examples/plugins/demo/plugin; it ships as a
// sub-module so the parent test suite does not depend on it.
package plugin

// APIVersion is the major version of the plugin protocol shared by the Drift
// CLI and the generated bridge binary. The CLI refuses bridge responses whose
// api_version differs.
const APIVersion = 1
