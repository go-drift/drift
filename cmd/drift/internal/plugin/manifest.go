// Package plugin implements the Drift CLI plugin pipeline: drift.yaml
// parsing, bridge generation, dependency checks, op validation and
// application, registrant emission, and the Xcode 16 preflight.
//
// The public plugin authoring API lives in pkg/plugin; this package is
// internal to the CLI.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ConfiguredPlugin is one entry in the drift.yaml `plugins:` list.
type ConfiguredPlugin struct {
	// Package is the import path of the plugin's registration package,
	// e.g. github.com/foo/drift-splash/plugin.
	Package string
	// Config is the raw yaml.Node for the plugin's config block. Decode
	// happens inside the bridge against the plugin's typed Config struct.
	Config yaml.Node
}

// LoadFromDriftYAML reads drift.yaml from projectRoot and returns the
// configured plugins in source order. An absent or empty file yields an
// empty slice and no error: plugins are optional.
func LoadFromDriftYAML(projectRoot string) ([]ConfiguredPlugin, error) {
	path := filepath.Join(projectRoot, "drift.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read drift.yaml: %w", err)
	}
	return ParseManifest(data)
}

// ParseManifest extracts the plugins section from a drift.yaml document.
func ParseManifest(data []byte) ([]ConfiguredPlugin, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse drift.yaml: %w", err)
	}
	doc := documentRoot(&root)
	if doc == nil || doc.Kind != yaml.MappingNode {
		return nil, nil
	}
	pluginsNode := mappingValue(doc, "plugins")
	if pluginsNode == nil {
		return nil, nil
	}
	if pluginsNode.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("drift.yaml: plugins must be a sequence at line %d", pluginsNode.Line)
	}
	out := make([]ConfiguredPlugin, 0, len(pluginsNode.Content))
	for i, entry := range pluginsNode.Content {
		if entry.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("drift.yaml: plugins[%d] must be a mapping at line %d", i, entry.Line)
		}
		pkgNode := mappingValue(entry, "package")
		if pkgNode == nil || pkgNode.Value == "" {
			return nil, fmt.Errorf("drift.yaml: plugins[%d] missing package at line %d", i, entry.Line)
		}
		cfg := mappingValue(entry, "config")
		var cfgNode yaml.Node
		if cfg != nil {
			cfgNode = *cfg
		}
		out = append(out, ConfiguredPlugin{
			Package: pkgNode.Value,
			Config:  cfgNode,
		})
	}
	return out, nil
}

// ConfigYAML returns the plugin's raw config as canonical YAML text. Empty
// config decodes to an empty string.
func (p ConfiguredPlugin) ConfigYAML() (string, error) {
	if p.Config.Kind == 0 {
		return "", nil
	}
	data, err := yaml.Marshal(&p.Config)
	if err != nil {
		return "", fmt.Errorf("marshal config for %s: %w", p.Package, err)
	}
	return string(data), nil
}

// ConfigMap decodes the plugin's config block into a generic map. Used by the
// CLI for schema validation; the bridge re-decodes into the plugin's typed
// Config struct independently.
func (p ConfiguredPlugin) ConfigMap() (map[string]any, error) {
	if p.Config.Kind == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := p.Config.Decode(&out); err != nil {
		return nil, fmt.Errorf("decode config for %s: %w", p.Package, err)
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

func documentRoot(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	return n
}

func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}
