package plugin

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

// Envelope is the JSON object the Drift CLI sends on the bridge stdin.
type Envelope struct {
	APIVersion  int              `json:"api_version"`
	Cmd         string           `json:"cmd"`
	Platform    string           `json:"platform,omitempty"`
	ProjectRoot string           `json:"project_root,omitempty"`
	BuildDir    string           `json:"build_dir,omitempty"`
	Plugins     []EnvelopePlugin `json:"plugins,omitempty"`
}

// EnvelopePlugin is one entry in the plugins array of the envelope.
type EnvelopePlugin struct {
	Package    string `json:"package"`
	ConfigYAML string `json:"config_yaml,omitempty"`
}

// Response is the JSON object the bridge writes to the response file.
type Response struct {
	APIVersion int                     `json:"api_version"`
	Ops        []json.RawMessage       `json:"ops,omitempty"`
	Schemas    map[string]PluginSchema `json:"schemas,omitempty"`
	Version    string                  `json:"version,omitempty"`
	Error      string                  `json:"error,omitempty"`
}

// Main is the bridge tool entry point. The generated tools/drift-plugins/main.go
// calls Main with one Binding per configured plugin.
//
// Main panics on duplicate plugin names: the bridge fails fast at startup
// rather than letting two plugins with the same friendly name confuse later
// diagnostics. It also panics on duplicate package paths.
func Main(bindings ...Binding) {
	checkBindings(bindings)

	var responseFile string
	fs := flag.NewFlagSet("drift-plugin-bridge", flag.ContinueOnError)
	fs.StringVar(&responseFile, "response-file", "", "path to write the JSON response")
	if err := fs.Parse(os.Args[1:]); err != nil {
		fatal("argument parse: %v", err)
	}
	if responseFile == "" {
		fatal("--response-file is required")
	}

	envBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		fatal("read stdin: %v", err)
	}
	var env Envelope
	if err := json.Unmarshal(envBytes, &env); err != nil {
		fatal("decode envelope: %v", err)
	}
	if env.APIVersion != APIVersion {
		fatal("api version mismatch: bridge=%d cli=%d", APIVersion, env.APIVersion)
	}

	resp := Response{APIVersion: APIVersion}

	switch env.Cmd {
	case "build":
		resp = doBuild(env, bindings)
	case "schema":
		resp = doSchema(bindings)
	case "version":
		resp.Version = fmt.Sprintf("drift-plugin-bridge api=%d", APIVersion)
	default:
		resp.Error = fmt.Sprintf("unknown cmd %q", env.Cmd)
	}

	if err := writeResponse(responseFile, resp); err != nil {
		fatal("write response: %v", err)
	}
}

func checkBindings(bindings []Binding) {
	seenPkg := make(map[string]bool, len(bindings))
	seenName := make(map[string]string, len(bindings))
	for _, b := range bindings {
		if b.Package == "" {
			panic("drift plugin: binding with empty package path")
		}
		if seenPkg[b.Package] {
			panic(fmt.Sprintf("drift plugin: duplicate binding for package %q", b.Package))
		}
		seenPkg[b.Package] = true
		if other, dup := seenName[b.Name]; dup {
			panic(fmt.Sprintf(
				"drift plugin: duplicate plugin name %q (in %s and %s)",
				b.Name, other, b.Package,
			))
		}
		seenName[b.Name] = b.Package
	}
}

func doBuild(env Envelope, bindings []Binding) Response {
	byPkg := make(map[string]Binding, len(bindings))
	for _, b := range bindings {
		byPkg[b.Package] = b
	}

	var allOps []json.RawMessage
	resp := Response{APIVersion: APIVersion}

	for _, ep := range env.Plugins {
		b, ok := byPkg[ep.Package]
		if !ok {
			resp.Error = fmt.Sprintf(
				"envelope references plugin %q with no matching binding; "+
					"regenerate tools/drift-plugins/main.go from drift.yaml",
				ep.Package,
			)
			return resp
		}
		ctx := newBuildCtx(b.Package, b.Name, env.ProjectRoot, env.BuildDir, env.Platform)
		if err := b.Build(ctx, []byte(ep.ConfigYAML)); err != nil {
			resp.Error = fmt.Sprintf("plugin %s build: %v", b.Package, err)
			return resp
		}
		if err := ctx.Err(); err != nil {
			resp.Error = fmt.Sprintf("plugin %s build: %v", b.Package, err)
			return resp
		}
		for _, op := range ctx.ops {
			raw, err := MarshalOp(op)
			if err != nil {
				resp.Error = fmt.Sprintf("plugin %s op encode: %v", b.Package, err)
				return resp
			}
			allOps = append(allOps, raw)
		}
	}

	resp.Ops = allOps
	return resp
}

func doSchema(bindings []Binding) Response {
	out := Response{APIVersion: APIVersion, Schemas: make(map[string]PluginSchema, len(bindings))}
	for _, b := range bindings {
		out.Schemas[b.Package] = b.schema
	}
	return out
}

func writeResponse(path string, r Response) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "drift-plugin-bridge: "+format+"\n", args...)
	os.Exit(1)
}
