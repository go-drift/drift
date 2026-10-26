// Package protocol is the wire contract between the Drift CLI and the
// generated plugin bridge binary: the envelope the CLI sends, the response
// the bridge writes, the op types carried in it, and the config schema the
// bridge reports.
//
// Plugin authors use package plugin (github.com/go-drift/drift/pkg/plugin),
// which records these ops through BuildCtx. They only reach into protocol to
// inspect recorded ops in tests (BuildCtx.Ops).
package protocol

import "encoding/json"

// APIVersion is the major version of the plugin protocol shared by the Drift
// CLI and the generated bridge binary. The CLI refuses bridge responses whose
// api_version differs.
const APIVersion = 1

// DriftModulePath is the module path of the Drift framework. The bridge
// reports the version of this module it was compiled with.
const DriftModulePath = "github.com/go-drift/drift"

// DevelVersion is the DriftVersion of a bridge compiled against a local
// Drift checkout (a directory replace or a workspace module), whose source
// no version names.
const DevelVersion = "(devel)"

// Envelope is the JSON object the Drift CLI sends on the bridge stdin.
type Envelope struct {
	APIVersion  int    `json:"api_version"`
	Cmd         string `json:"cmd"`
	Platform    string `json:"platform,omitempty"`
	ProjectRoot string `json:"project_root,omitempty"`
	BuildDir    string `json:"build_dir,omitempty"`
	// AppID is the app's bundle identifier / application id (drift.yaml
	// app.id). Required for "build".
	AppID   string           `json:"app_id,omitempty"`
	Plugins []EnvelopePlugin `json:"plugins,omitempty"`
}

// EnvelopePlugin is one entry in the plugins array of the envelope.
type EnvelopePlugin struct {
	Package    string `json:"package"`
	ConfigYAML string `json:"config_yaml,omitempty"`
}

// Response is the JSON object the bridge writes to the response file.
type Response struct {
	APIVersion int `json:"api_version"`
	// DriftVersion is the version of DriftModulePath the bridge was compiled
	// with, and so of its pkg/plugin and op set: a module version (v0.27.0,
	// or a pseudo-version) or DevelVersion. Set on every response.
	DriftVersion string                  `json:"drift_version"`
	Ops          []json.RawMessage       `json:"ops,omitempty"`
	Schemas      map[string]PluginSchema `json:"schemas,omitempty"`
	Error        string                  `json:"error,omitempty"`
}
