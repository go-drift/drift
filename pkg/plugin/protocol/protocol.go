// Package protocol is the wire contract between the Drift CLI and the
// generated plugin bridge binary: the envelope the CLI sends, the response
// the bridge writes, the op types carried in it, and the config schema the
// bridge reports.
//
// Plugin authors use package plugin (github.com/go-drift/drift/pkg/plugin),
// which records these ops through BuildCtx. They only reach into protocol to
// inspect recorded ops in tests (BuildCtx.Ops).
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
)

// APIVersion is the major version of the plugin protocol shared by the Drift
// CLI and the generated bridge binary. The CLI refuses bridge responses whose
// api_version differs.
const APIVersion = 2

// DriftModulePath is the module path of the Drift framework. The bridge
// reports the version of this module it was compiled with.
const DriftModulePath = "github.com/go-drift/drift"

// DevelVersion is the DriftVersion of a bridge compiled against a local
// Drift checkout (a directory replace or a workspace module), whose source
// no version names.
const DevelVersion = "(devel)"

// Envelope is the JSON object the Drift CLI sends on the bridge stdin.
type Envelope struct {
	APIVersion int    `json:"api_version"`
	Cmd        string `json:"cmd"`
	// Platform is the platform being built: "android", "ios" or "xtool",
	// or "all" from drift plugin sync, which checks config for every
	// platform and builds none (so BuildDir is empty).
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
	// Error reports a request the bridge could not serve, such as an
	// envelope naming an unbound plugin.
	Error string `json:"error,omitempty"`
	// PluginError reports the plugin whose Build failed; the build stops
	// at it.
	PluginError *PluginError `json:"plugin_error,omitempty"`
}

// Err returns the response's failure, or nil.
func (r *Response) Err() error {
	if r.PluginError != nil {
		return r.PluginError
	}
	if r.Error != "" {
		return errors.New(r.Error)
	}
	return nil
}

// PluginError is a plugin's failed Build: its config did not decode, Build
// returned an error, a recorder rejected its input, or an op did not
// encode.
type PluginError struct {
	Plugin  string `json:"plugin"`
	Message string `json:"message"`
}

func (e *PluginError) Error() string {
	return fmt.Sprintf("plugin %s build: %s", e.Plugin, e.Message)
}
