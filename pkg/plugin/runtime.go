package plugin

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

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
	var env protocol.Envelope
	if err := json.Unmarshal(envBytes, &env); err != nil {
		fatal("decode envelope: %v", err)
	}
	if env.APIVersion != protocol.APIVersion {
		fatal("api version mismatch: bridge=%d cli=%d", protocol.APIVersion, env.APIVersion)
	}

	resp := protocol.Response{APIVersion: protocol.APIVersion}

	switch env.Cmd {
	case "build":
		resp = doBuild(env, bindings)
	case "schema":
		resp = doSchema(bindings)
	case "version":
		resp.Version = fmt.Sprintf("drift-plugin-bridge api=%d", protocol.APIVersion)
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

func doBuild(env protocol.Envelope, bindings []Binding) protocol.Response {
	if env.AppID == "" {
		return protocol.Response{APIVersion: protocol.APIVersion, Error: "build envelope has no app_id"}
	}
	byPkg := make(map[string]Binding, len(bindings))
	for _, b := range bindings {
		byPkg[b.Package] = b
	}

	var allOps []json.RawMessage
	resp := protocol.Response{APIVersion: protocol.APIVersion}

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
		ctx := newBuildCtx(b.Package, b.Name, env)
		if err := b.Build(ctx, []byte(ep.ConfigYAML)); err != nil {
			resp.Error = fmt.Sprintf("plugin %s build: %v", b.Package, err)
			return resp
		}
		if err := ctx.Err(); err != nil {
			resp.Error = fmt.Sprintf("plugin %s build: %v", b.Package, err)
			return resp
		}
		for _, op := range ctx.ops {
			raw, err := protocol.MarshalOp(op)
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

func doSchema(bindings []Binding) protocol.Response {
	out := protocol.Response{APIVersion: protocol.APIVersion, Schemas: make(map[string]protocol.PluginSchema, len(bindings))}
	for _, b := range bindings {
		out.Schemas[b.Package] = b.schema
	}
	return out
}

func writeResponse(path string, r protocol.Response) error {
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
