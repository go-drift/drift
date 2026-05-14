package plugin

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"

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
	schema PluginSchema
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

	return Binding{
		Package: pkgPath,
		Name:    name,
		buildAny: func(ctx *BuildCtx, configYAML []byte) error {
			var cfg T
			if len(configYAML) > 0 {
				// KnownFields rejects unknown keys so a typo in drift.yaml
				// surfaces as a build-time error instead of silently being
				// dropped. Required-field validation runs after decode.
				dec := yaml.NewDecoder(bytes.NewReader(configYAML))
				dec.KnownFields(true)
				if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
					return fmt.Errorf("plugin %s: decode config: %w", pkgPath, err)
				}
			}
			if err := validateRequired(pkgPath, schema, cfg); err != nil {
				return err
			}
			return p.Build(ctx, cfg)
		},
		schema: schema,
	}
}

// validateRequired returns an error naming the first required field that is
// zero-valued after decode. The drift CLI's `drift plugin sync` runs the
// same check; running it inline here means `drift build`/`run` (the main
// user path) also rejects missing required config.
func validateRequired[T any](pkgPath string, schema PluginSchema, cfg T) error {
	v := reflect.ValueOf(cfg)
	if v.Kind() != reflect.Struct {
		return nil
	}
	for _, f := range schema.Fields {
		if !f.Required {
			continue
		}
		fv := v.FieldByName(f.GoField)
		if !fv.IsValid() {
			continue
		}
		if fv.IsZero() {
			return fmt.Errorf("plugin %s: required config field %q is missing", pkgPath, f.Name)
		}
	}
	return nil
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
func (b Binding) Schema() PluginSchema { return b.schema }
