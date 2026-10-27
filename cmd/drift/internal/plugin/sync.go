package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/config"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// SyncOptions controls drift plugin sync.
type SyncOptions struct {
	ProjectRoot string
	CLIVersion  string
	// Tidy runs go mod tidy after regenerating the bridge source, before
	// anything reads the module graph.
	Tidy bool
}

// SyncResult is the outcome of a sync run, surfaced to the CLI for reporting.
type SyncResult struct {
	Plugins           []ConfiguredPlugin
	BridgeRegenerated bool
	Missing           []string
	Diagnostics       []protocol.ValidationDiagnostic
	Schemas           map[string]protocol.PluginSchema
}

// Sync implements the drift plugin sync command. It regenerates
// tools/drift-plugins/main.go, rebuilds the cached bridge if needed, runs
// schema validation against drift.yaml and, if that passes, every plugin's
// Build (see checkBuilds). It never edits go.mod or go.sum on its own; with
// Tidy it runs `go mod tidy` after regenerating the bridge source and
// before resolving the plugins.
func Sync(opts SyncOptions) (*SyncResult, error) {
	plugins, err := LoadFromDriftYAML(opts.ProjectRoot)
	if err != nil {
		return nil, err
	}
	res := &SyncResult{Plugins: plugins}

	if len(plugins) == 0 {
		// Zero plugins: remove the stale bridge file if present.
		bridgePath := filepath.Join(opts.ProjectRoot, BridgeFilePath)
		if _, err := os.Stat(bridgePath); err == nil {
			if err := os.Remove(bridgePath); err != nil {
				return res, fmt.Errorf("remove stale bridge: %w", err)
			}
			res.BridgeRegenerated = true
		}
		return res, nil
	}

	if opts.Tidy {
		// Tidy against the regenerated bridge source, before resolving the
		// plugins and building the bridge: both run -mod=readonly and need
		// the entries tidy adds for a plugin just required.
		if _, err := writeBridgeSource(opts.ProjectRoot, plugins); err != nil {
			return res, err
		}
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = opts.ProjectRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return res, fmt.Errorf("go mod tidy: %w", err)
		}
	}

	infos, err := CheckPluginDeps(opts.CLIVersion, opts.ProjectRoot, plugins, NewGoListResolver(opts.ProjectRoot))
	if err != nil {
		var missing *MissingPluginError
		if errors.As(err, &missing) {
			res.Missing = missing.Packages
		}
		return res, err
	}

	bridge, err := EnsureBridge(opts.ProjectRoot, opts.CLIVersion, plugins, infos)
	if err != nil {
		return res, err
	}
	// EnsureBridge always writes when content differs; treat the flag as
	// best-effort and report true if the file exists.
	res.BridgeRegenerated = true

	schemaResp, err := RunBridge(bridge, protocol.Envelope{
		APIVersion: protocol.APIVersion,
		Cmd:        "schema",
	}, "")
	if err != nil {
		return res, fmt.Errorf("schema lookup: %w", err)
	}
	res.Schemas = schemaResp.Schemas

	for _, p := range plugins {
		schema, ok := schemaResp.Schemas[p.Package]
		if !ok {
			res.Diagnostics = append(res.Diagnostics, protocol.ValidationDiagnostic{
				Plugin:  p.Package,
				Message: "bridge returned no schema for plugin",
			})
			continue
		}
		cfg, err := p.ConfigMap()
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, protocol.ValidationDiagnostic{
				Plugin:  p.Package,
				Message: err.Error(),
			})
			continue
		}
		res.Diagnostics = append(res.Diagnostics, schema.ValidateConfig(cfg, opts.ProjectRoot)...)
	}
	if len(res.Diagnostics) > 0 {
		return res, nil
	}

	cfg, err := config.Resolve(opts.ProjectRoot)
	if err != nil {
		return res, err
	}
	res.Diagnostics, err = checkBuilds(bridge, opts.ProjectRoot, cfg.AppID, plugins)
	return res, err
}

// checkBuilds runs each plugin's Build, as a build would, and returns its
// failures as diagnostics: schema-valid config can still fail a plugin's
// own checks. The platform is "all", so Build checks the config for every
// platform it configures without the sync choosing one; the ops are
// discarded. Each plugin gets its own envelope, so one failure does not
// mask the next.
func checkBuilds(bridge *Bridge, projectRoot, appID string, plugins []ConfiguredPlugin) ([]protocol.ValidationDiagnostic, error) {
	var diags []protocol.ValidationDiagnostic
	for _, p := range plugins {
		y, err := p.ConfigYAML()
		if err != nil {
			return nil, err
		}
		_, err = RunBridge(bridge, protocol.Envelope{
			APIVersion:  protocol.APIVersion,
			Cmd:         "build",
			Platform:    "all",
			ProjectRoot: projectRoot,
			AppID:       appID,
			Plugins:     []protocol.EnvelopePlugin{{Package: p.Package, ConfigYAML: y}},
		}, "")
		var pluginErr *protocol.PluginError
		switch {
		case errors.As(err, &pluginErr):
			diags = append(diags, protocol.ValidationDiagnostic{Plugin: pluginErr.Plugin, Message: pluginErr.Message})
		case err != nil:
			return nil, err
		}
	}
	return diags, nil
}

// ListResult is the result of drift plugin list.
type ListResult struct {
	Entries []ListEntry
}

// ListEntry is one row of drift plugin list.
type ListEntry struct {
	Package string `json:"package"`
	Version string `json:"version,omitempty"`
	Module  string `json:"module,omitempty"`
	Status  string `json:"status"`
	Name    string `json:"name,omitempty"`
}

// List implements drift plugin list. With resolve=false, status is derived
// purely from drift.yaml + go list; with resolve=true, the bridge is built
// and schema/version is executed so Name and incompatible-api detection
// become available.
func List(projectRoot, cliVersion string, resolve bool) (*ListResult, error) {
	plugins, err := LoadFromDriftYAML(projectRoot)
	if err != nil {
		return nil, err
	}
	out := &ListResult{}
	if len(plugins) == 0 {
		return out, nil
	}

	resolver := NewGoListResolver(projectRoot)
	infos, depErr := CheckPluginDeps(cliVersion, projectRoot, plugins, resolver)
	if depErr != nil {
		// We still want to display rows for the resolved subset and mark
		// missing entries; reset and re-resolve individually.
		infos = make([]*PackageInfo, len(plugins))
		for i, p := range plugins {
			info, _ := resolver(p.Package)
			infos[i] = info
		}
	}

	var schemas map[string]protocol.PluginSchema
	// A bridge importing missing packages cannot build, so resolving only
	// happens once every plugin is installed; the rows below mark the
	// missing ones either way.
	if resolve && depErr == nil {
		bridge, err := EnsureBridge(projectRoot, cliVersion, plugins, infos)
		if err != nil {
			return nil, err
		}
		resp, err := RunBridge(bridge, protocol.Envelope{
			APIVersion: protocol.APIVersion,
			Cmd:        "schema",
		}, "")
		if err != nil {
			return nil, err
		}
		schemas = resp.Schemas
	}

	for i, p := range plugins {
		entry := ListEntry{Package: p.Package, Status: "ok"}
		info := infos[i]
		if info == nil || info.Module == nil || info.Error != "" {
			entry.Status = "missing"
		} else {
			entry.Version = info.Module.Version
			entry.Module = info.Module.Path
			if info.Module.Replace != nil {
				entry.Status = "local-replace"
			}
		}
		if schemas != nil {
			if s, ok := schemas[p.Package]; ok {
				entry.Name = s.Name
			}
		}
		out.Entries = append(out.Entries, entry)
	}
	return out, nil
}

// FormatListTable renders a ListResult as the default plaintext table.
func FormatListTable(res *ListResult, withName bool) string {
	cols := []string{"PACKAGE", "VERSION", "STATUS"}
	if withName {
		cols = []string{"PACKAGE", "VERSION", "NAME", "STATUS"}
	}
	rows := make([][]string, 0, len(res.Entries))
	for _, e := range res.Entries {
		row := []string{e.Package, e.Version, e.Status}
		if withName {
			row = []string{e.Package, e.Version, e.Name, e.Status}
		}
		rows = append(rows, row)
	}
	return formatTable(cols, rows)
}

// FormatListJSON renders a ListResult as JSON.
func FormatListJSON(res *ListResult) ([]byte, error) {
	return json.MarshalIndent(res.Entries, "", "  ")
}

func formatTable(cols []string, rows [][]string) string {
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = len(c)
	}
	for _, row := range rows {
		for i, cell := range row {
			if i < len(widths) && len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	var b strings.Builder
	writeRow := func(cells []string) {
		for i, cell := range cells {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(cell)
			b.WriteString(strings.Repeat(" ", widths[i]-len(cell)))
		}
		b.WriteByte('\n')
	}
	writeRow(cols)
	for _, row := range rows {
		writeRow(row)
	}
	return b.String()
}
