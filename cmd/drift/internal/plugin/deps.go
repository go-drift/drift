package plugin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/cache"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// PackageInfo summarises the bits of `go list -e -json <pkg>` we care about.
type PackageInfo struct {
	// ImportPath is echoed back from the command.
	ImportPath string
	// Module is nil when the package is unknown to the module graph.
	Module *ModuleInfo
	// Error is set when `go list` reported a package-level error.
	Error string
}

// notFound reports whether no module in the graph provides the package, so
// the remedy is to require one: go list's "no required module provides
// package" (with -mod=mod, "cannot find module providing package"). Any
// other error, such as a missing go.sum entry for the module that does
// provide it, is not this.
func (p *PackageInfo) notFound() bool {
	return p.Error == "" && p.Module == nil ||
		strings.Contains(p.Error, "no required module provides package") ||
		strings.Contains(p.Error, "cannot find module providing package")
}

// ModuleInfo is the subset of go list module metadata we read.
type ModuleInfo struct {
	Path    string
	Version string
	Replace *Replace
	// Main is set for the main module (the app itself).
	Main bool
	// GoMod is the module's go.mod file: in the module cache, or in the
	// local directory of a replaced or workspace module.
	GoMod string
}

// String names the module and where its source comes from.
func (m *ModuleInfo) String() string {
	switch {
	case m.Replace != nil && m.Replace.Version == "":
		return m.Path + " => " + m.Replace.Path
	case m.Replace != nil:
		return m.Path + " => " + m.Replace.Path + "@" + m.Replace.Version
	case m.Version == "":
		return m.Path + " (local)"
	default:
		return m.Path + "@" + m.Version
	}
}

// Replace mirrors the Module.Replace structure from go list.
type Replace struct {
	Path    string
	Version string
}

// PackageResolver resolves a Go import path to its PackageInfo. The default
// implementation shells out to `go list`; tests inject a stub.
type PackageResolver func(pkg string) (*PackageInfo, error)

// NewGoListResolver returns a PackageResolver that runs `go list -e -json`
// from the given project root.
func NewGoListResolver(projectRoot string) PackageResolver {
	return func(pkg string) (*PackageInfo, error) {
		cmd := exec.Command("go", "list", "-e", "-json", pkg)
		cmd.Dir = projectRoot
		out, err := cmd.Output()
		if err != nil {
			// go list with -e shouldn't normally fail, but if it does
			// (e.g. malformed go.mod), surface the stderr.
			if ee, ok := err.(*exec.ExitError); ok {
				return nil, fmt.Errorf("go list %s: %s", pkg, strings.TrimSpace(string(ee.Stderr)))
			}
			return nil, fmt.Errorf("go list %s: %w", pkg, err)
		}
		return parseGoList(out)
	}
}

func parseGoList(data []byte) (*PackageInfo, error) {
	var raw struct {
		ImportPath string
		Error      *struct {
			Err string
		}
		Module *struct {
			Path    string
			Version string
			Main    bool
			GoMod   string
			Replace *struct {
				Path    string
				Version string
			}
		}
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode go list output: %w", err)
	}
	info := &PackageInfo{ImportPath: raw.ImportPath}
	if raw.Error != nil {
		info.Error = raw.Error.Err
	}
	if raw.Module != nil {
		info.Module = &ModuleInfo{
			Path:    raw.Module.Path,
			Version: raw.Module.Version,
			Main:    raw.Module.Main,
			GoMod:   raw.Module.GoMod,
		}
		if raw.Module.Replace != nil {
			info.Module.Replace = &Replace{
				Path:    raw.Module.Replace.Path,
				Version: raw.Module.Replace.Version,
			}
		}
	}
	return info, nil
}

// MissingPluginError lists plugin packages declared in drift.yaml that the
// module graph does not know about. Always carries the full list; callers
// read Packages.
type MissingPluginError struct {
	// CLI is the CLI's release version; empty for a development build.
	CLI      string
	Packages []string
}

func (e *MissingPluginError) Error() string {
	var sb strings.Builder
	if len(e.Packages) == 1 {
		fmt.Fprintf(&sb, "Plugin package not found: %s\n", e.Packages[0])
	} else {
		sb.WriteString("Plugin packages not found:\n")
		for _, p := range e.Packages {
			fmt.Fprintf(&sb, "  - %s\n", p)
		}
	}
	var firstParty, thirdParty []string
	for _, p := range e.Packages {
		if isDriftPackage(p) {
			firstParty = append(firstParty, p)
		} else {
			thirdParty = append(thirdParty, p)
		}
	}
	if len(thirdParty) > 0 {
		sb.WriteString("\nRun:\n")
		for _, p := range thirdParty {
			fmt.Fprintf(&sb, "  go get %s@latest\n", p)
		}
		if e.CLI != "" {
			fmt.Fprintf(&sb, "\nIf a plugin's latest version requires a drift newer than this CLI (%s), get an earlier version of it.\n", e.CLI)
		}
	}
	if len(firstParty) > 0 {
		fmt.Fprintf(&sb, "\nFirst-party plugins ship in %s itself. Check the package path in drift.yaml, "+
			"or update the app to a drift that has the plugin:\n  go get %s@%s\n",
			protocol.DriftModulePath, protocol.DriftModulePath, e.driftVersion())
	}
	sb.WriteString("\nThen commit drift.yaml and tools/drift-plugins/main.go together.")
	return sb.String()
}

// driftVersion is the drift version to update an app to for a missing
// first-party plugin: a release CLI names its own version, since a newer
// drift needs a newer CLI; a development CLI names the latest.
func (e *MissingPluginError) driftVersion() string {
	if e.CLI != "" {
		return e.CLI
	}
	return "latest"
}

// isDriftPackage reports whether pkg is in the drift module, which holds the
// first-party plugins.
func isDriftPackage(pkg string) bool {
	return pkg == protocol.DriftModulePath || strings.HasPrefix(pkg, protocol.DriftModulePath+"/")
}

// UnresolvedPluginError lists plugin packages that go could not load for a
// reason other than no module providing them, such as a missing go.sum
// entry, with go's error for each.
type UnresolvedPluginError struct {
	// ProjectRoot is the app's root, named in the go mod tidy hint.
	ProjectRoot string
	Packages    []UnresolvedPackage
}

// UnresolvedPackage is a plugin package and go list's error for it.
type UnresolvedPackage struct {
	Package string
	Err     string
}

func (e *UnresolvedPluginError) Error() string {
	var sb strings.Builder
	tidy := false
	for i, p := range e.Packages {
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "Plugin package %s could not be loaded:\n  %s", p.Package, strings.ReplaceAll(p.Err, "\n", "\n  "))
		tidy = tidy || needsTidy(p.Err)
	}
	if tidy {
		sb.WriteString(tidyHint(e.ProjectRoot))
	}
	return sb.String()
}

// CheckPluginDeps verifies each configured plugin is reachable through the
// module graph of the app at projectRoot. All problems are collected so one
// CLI invocation surfaces every fix: plugins no module provides as a
// *MissingPluginError, plugins go could not load for another reason as an
// *UnresolvedPluginError, joined when both occur. cliVersion is the CLI's
// version; a release names its own drift version in the hint for
// first-party plugins.
func CheckPluginDeps(cliVersion, projectRoot string, plugins []ConfiguredPlugin, resolver PackageResolver) ([]*PackageInfo, error) {
	out := make([]*PackageInfo, len(plugins))
	var missing []string
	var unresolved []UnresolvedPackage
	for i, p := range plugins {
		info, err := resolver(p.Package)
		if err != nil {
			return nil, err
		}
		switch {
		case info == nil || info.notFound():
			missing = append(missing, p.Package)
		case info.Error != "":
			unresolved = append(unresolved, UnresolvedPackage{Package: p.Package, Err: info.Error})
		default:
			out[i] = info
		}
	}
	var errs []error
	if len(missing) > 0 {
		errs = append(errs, &MissingPluginError{CLI: cache.NormalizeVersion(cliVersion), Packages: missing})
	}
	if len(unresolved) > 0 {
		errs = append(errs, &UnresolvedPluginError{ProjectRoot: projectRoot, Packages: unresolved})
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return out, nil
}

// hasLocalSources reports whether any source in the bridge build lives on
// the local filesystem rather than at a pinned module version: a plugin
// package in the main module or a workspace module, a directory replace
// anywhere in the module graph (including github.com/go-drift/drift
// itself), or a go.work spanning more than one module.
func hasLocalSources(projectRoot string, infos []*PackageInfo) (bool, error) {
	for _, info := range infos {
		if info != nil && info.Module != nil && (info.Module.Replace != nil || info.Module.Version == "") {
			return true, nil
		}
	}
	cmd := exec.Command("go", "list", "-mod=readonly", "-m", "-json", "all")
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		msg := commandError(err).Error()
		if needsTidy(msg) {
			return false, fmt.Errorf("go list -m all: %s%s", msg, tidyHint(projectRoot))
		}
		return false, fmt.Errorf("go list -m all: %s", msg)
	}
	return graphHasLocalModules(out)
}

// graphHasLocalModules decodes the `go list -m -json all` stream and
// reports a directory replace (a replace without a version) or more than
// one main module (a multi-module workspace).
func graphHasLocalModules(data []byte) (bool, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	mains := 0
	for {
		var m struct {
			Main    bool
			Replace *struct{ Version string }
		}
		if err := dec.Decode(&m); err == io.EOF {
			break
		} else if err != nil {
			return false, fmt.Errorf("decode go list -m output: %w", err)
		}
		if m.Main {
			mains++
		}
		if m.Replace != nil && m.Replace.Version == "" {
			return true, nil
		}
	}
	return mains > 1, nil
}

// commandError returns err with the command's stderr when available.
func commandError(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return errors.New(strings.TrimSpace(string(ee.Stderr)))
	}
	return err
}
