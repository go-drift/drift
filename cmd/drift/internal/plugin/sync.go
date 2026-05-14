package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// SyncOptions controls drift plugin sync.
type SyncOptions struct {
	ProjectRoot string
	CLIVersion  string
	// Tidy runs go mod tidy after bridge regeneration.
	Tidy bool
}

// SyncResult is the outcome of a sync run, surfaced to the CLI for reporting.
type SyncResult struct {
	Plugins           []ConfiguredPlugin
	BridgeRegenerated bool
	Missing           []string
	Diagnostics       []driftplugin.ValidationDiagnostic
	Schemas           map[string]driftplugin.PluginSchema
}

// Sync implements the drift plugin sync command. It regenerates
// tools/drift-plugins/main.go, rebuilds the cached bridge if needed, runs
// schema validation against drift.yaml, and (optionally) runs go mod tidy.
// Sync never edits go.mod on its own; --tidy delegates to `go mod tidy`.
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

	resolver := NewGoListResolver(opts.ProjectRoot)
	// Walk all plugins individually so a single missing entry doesn't mask
	// the others: users get one `go get` line per missing package.
	infos := make([]*PackageInfo, len(plugins))
	for i, p := range plugins {
		info, err := resolver(p.Package)
		if err != nil {
			return res, err
		}
		if info == nil || info.Module == nil || info.Error != "" {
			res.Missing = append(res.Missing, p.Package)
			continue
		}
		infos[i] = info
	}
	if len(res.Missing) > 0 {
		return res, &MissingPluginError{Packages: res.Missing}
	}

	bridge, err := EnsureBridge(opts.ProjectRoot, opts.CLIVersion, plugins, infos)
	if err != nil {
		return res, err
	}
	// EnsureBridge always writes when content differs; treat the flag as
	// best-effort and report true if the file exists.
	res.BridgeRegenerated = true

	schemaResp, err := RunBridge(bridge, driftplugin.Envelope{
		APIVersion: driftplugin.APIVersion,
		Cmd:        "schema",
	}, "")
	if err != nil {
		return res, fmt.Errorf("schema lookup: %w", err)
	}
	res.Schemas = schemaResp.Schemas

	for _, p := range plugins {
		schema, ok := schemaResp.Schemas[p.Package]
		if !ok {
			res.Diagnostics = append(res.Diagnostics, driftplugin.ValidationDiagnostic{
				Plugin:  p.Package,
				Message: "bridge returned no schema for plugin",
			})
			continue
		}
		cfg, err := p.ConfigMap()
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, driftplugin.ValidationDiagnostic{
				Plugin:  p.Package,
				Message: err.Error(),
			})
			continue
		}
		res.Diagnostics = append(res.Diagnostics, schema.ValidateConfig(cfg)...)
	}

	if opts.Tidy {
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = opts.ProjectRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return res, fmt.Errorf("go mod tidy: %w", err)
		}
	}
	return res, nil
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
	infos, depErr := CheckPluginDeps(plugins, resolver)
	if depErr != nil {
		// We still want to display rows for the resolved subset and mark
		// missing entries; reset and re-resolve individually.
		infos = make([]*PackageInfo, len(plugins))
		for i, p := range plugins {
			info, _ := resolver(p.Package)
			infos[i] = info
		}
	}

	var schemas map[string]driftplugin.PluginSchema
	if resolve {
		bridge, err := EnsureBridge(projectRoot, cliVersion, plugins, infos)
		if err == nil {
			resp, err := RunBridge(bridge, driftplugin.Envelope{
				APIVersion: driftplugin.APIVersion,
				Cmd:        "schema",
			}, "")
			if err == nil {
				schemas = resp.Schemas
			}
		}
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
