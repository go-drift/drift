package plugin

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/go-drift/drift/pkg/plugin/protocol"

	"gopkg.in/yaml.v3"
)

// Plugin is the generic plugin interface. Plugin authors implement Build to
// emit ops on the supplied BuildCtx. Name returns the short friendly
// identifier used in diagnostics and the drift plugin list NAME column.
type Plugin[T any] interface {
	Name() string
	Build(ctx *BuildCtx, cfg T) error
}

// Binding pairs a plugin value with the drift.yaml package path it was loaded
// from. The bridge tool calls Bind for each configured plugin and forwards
// the slice to Main; the slice is the entire registry for the run.
type Binding struct {
	Package string
	Name    string
	// buildAny closes over the typed Plugin[T] and runs Build for a raw YAML
	// config blob. T is resolved at the Bind call site, so no reflection on
	// the user type is needed.
	buildAny func(ctx *BuildCtx, configYAML []byte) error
	// schema is captured at Bind time so the schema subcommand can describe
	// the plugin's Config without re-running Build.
	schema protocol.PluginSchema
}

// Bind wraps a typed Plugin[T] into a non-generic Binding suitable for Main.
// Bind preserves T via closure capture so config decoding can target the
// exact struct without reflecting back to a runtime type.
func Bind[T any](pkgPath string, p Plugin[T]) Binding {
	if p == nil {
		panic(fmt.Sprintf("drift plugin: Bind(%q): plugin value is nil", pkgPath))
	}
	if pkgPath == "" {
		panic("drift plugin: Bind: package path is empty")
	}
	name := p.Name()
	if name == "" {
		panic(fmt.Sprintf("drift plugin: Bind(%q): Plugin.Name() returned empty string; "+
			"return a short identifier (e.g. \"splash\") so duplicate detection and the "+
			"plugin list NAME column have something to display", pkgPath))
	}
	var zero T
	schema := schemaFor(pkgPath, name, reflect.TypeOf(zero))
	if err := schema.Check(); err != nil {
		panic(fmt.Sprintf("drift plugin: Bind(%q): config schema: %v", pkgPath, err))
	}

	return Binding{
		Package: pkgPath,
		Name:    name,
		buildAny: func(ctx *BuildCtx, configYAML []byte) error {
			cfg, err := decodeConfig[T](schema, configYAML, ctx.ProjectRoot())
			if err != nil {
				return fmt.Errorf("plugin %s: %w", pkgPath, err)
			}
			return p.Build(ctx, cfg)
		},
		schema: schema,
	}
}

// decodeConfig turns a plugin's raw drift.yaml config into T: the generic
// mapping is checked against the schema (the same ValidateConfig that
// `drift plugin sync` runs), defaults fill absent keys, and the result is
// decoded strictly into T.
func decodeConfig[T any](schema protocol.PluginSchema, configYAML []byte, projectRoot string) (T, error) {
	var cfg T
	raw := map[string]any{}
	if err := yaml.Unmarshal(configYAML, &raw); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}
	if raw == nil {
		raw = map[string]any{}
	}
	if err := protocol.DiagnosticsError(schema.ValidateConfig(raw, projectRoot)); err != nil {
		return cfg, fmt.Errorf("invalid config: %w", err)
	}
	resolved, err := yaml.Marshal(schema.ApplyDefaults(raw))
	if err != nil {
		return cfg, fmt.Errorf("encode config: %w", err)
	}
	// KnownFields keeps typed decoding strict for anything the schema walk
	// cannot see (e.g. map values).
	dec := yaml.NewDecoder(bytes.NewReader(resolved))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return cfg, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

// Build dispatches a single plugin's Build with the raw YAML config bytes.
// Exposed for the bridge runtime; plugin authors do not call this.
func (b Binding) Build(ctx *BuildCtx, configYAML []byte) error {
	if b.buildAny == nil {
		return fmt.Errorf("plugin %s: binding missing build hook", b.Package)
	}
	return b.buildAny(ctx, configYAML)
}

// Schema returns the captured PluginSchema for the binding.
func (b Binding) Schema() protocol.PluginSchema { return b.schema }
