package plugin

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
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

// ModuleInfo is the subset of go list module metadata we read.
type ModuleInfo struct {
	Path    string
	Version string
	Replace *Replace
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
		info.Module = &ModuleInfo{Path: raw.Module.Path, Version: raw.Module.Version}
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
	Packages []string
}

func (e *MissingPluginError) Error() string {
	pkgs := e.Packages
	if len(pkgs) == 1 {
		return fmt.Sprintf(`Plugin package not found: %s

Run:
  go get %s@latest

After installing, commit drift.yaml and tools/drift-plugins/main.go together.`,
			pkgs[0], pkgs[0],
		)
	}
	var sb strings.Builder
	sb.WriteString("Plugin packages not found:\n")
	for _, p := range pkgs {
		fmt.Fprintf(&sb, "  - %s\n", p)
	}
	sb.WriteString("\nRun:\n")
	for _, p := range pkgs {
		fmt.Fprintf(&sb, "  go get %s@latest\n", p)
	}
	sb.WriteString("\nAfter installing, commit drift.yaml and tools/drift-plugins/main.go together.")
	return sb.String()
}

// CheckPluginDeps verifies each configured plugin is reachable through the
// module graph. All missing entries are collected so one CLI invocation
// surfaces every required `go get`.
func CheckPluginDeps(plugins []ConfiguredPlugin, resolver PackageResolver) ([]*PackageInfo, error) {
	out := make([]*PackageInfo, len(plugins))
	var missing []string
	for i, p := range plugins {
		info, err := resolver(p.Package)
		if err != nil {
			return nil, err
		}
		if info == nil || info.Module == nil || info.Error != "" {
			missing = append(missing, p.Package)
			continue
		}
		out[i] = info
	}
	if len(missing) > 0 {
		return nil, &MissingPluginError{Packages: missing}
	}
	return out, nil
}
