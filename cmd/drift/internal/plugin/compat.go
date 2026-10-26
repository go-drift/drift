package plugin

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"

	"github.com/go-drift/drift/cmd/drift/internal/templates"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// Version compatibility between the CLI and the plugins it builds.
//
// A plugin's native code is compiled against the CLI's plugin API templates
// (DriftPluginAPI on iOS, com.drift.runner on Android), and its ops are
// decoded by the CLI. A plugin declares the API it was written against by
// the drift version its go.mod requires. The plugin API changes only in a
// minor release, so versions compare by major.minor: the CLI serves a
// plugin whose required major.minor is neither newer than the CLI's nor
// older than templates.PluginAPIFloor's, and a bridge compiled with its own
// drift major.minor. A development CLI (not a release version) skips these
// checks.

// PluginRequirement is a configured plugin and the drift version its module
// requires.
type PluginRequirement struct {
	Package string // drift.yaml package
	Module  string // module path and version, as go list resolved it
	Drift   string // drift version the module's go.mod requires
}

func (r PluginRequirement) String() string {
	return fmt.Sprintf("%s (%s) requires drift %s", r.Package, r.Module, r.Drift)
}

// VersionSkewError lists configured plugins this CLI cannot serve. At least
// one of TooNew and TooOld is non-empty.
type VersionSkewError struct {
	CLI string // the CLI's release version
	// TooNew require a drift major.minor newer than the CLI's: their native
	// code may use plugin API the CLI's templates lack.
	TooNew []PluginRequirement
	// TooOld require a drift major.minor older than
	// templates.PluginAPIFloor's: the plugin API changed incompatibly since.
	TooOld []PluginRequirement
}

func (e *VersionSkewError) Error() string {
	var sb strings.Builder
	if len(e.TooNew) > 0 {
		newest := e.TooNew[0].Drift
		fmt.Fprintf(&sb, "Plugins require a newer drift than this CLI (%s):\n", e.CLI)
		for _, r := range e.TooNew {
			fmt.Fprintf(&sb, "  - %s\n", r)
			if semver.Compare(r.Drift, newest) > 0 {
				newest = r.Drift
			}
		}
		fmt.Fprintf(&sb, "\nUpdate the CLI:\n  go install %s/cmd/drift@%s\n", protocol.DriftModulePath, newest)
	}
	if len(e.TooOld) > 0 {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "Plugins were written for an older plugin API than this CLI (%s) serves, which needs drift %s or later:\n",
			e.CLI, templates.PluginAPIFloor)
		for _, r := range e.TooOld {
			fmt.Fprintf(&sb, "  - %s\n", r)
		}
		sb.WriteString("\nUpdate them to a version for this CLI, or use the CLI they were written for.")
	}
	return strings.TrimRight(sb.String(), "\n")
}

// checkPluginCompat returns a *VersionSkewError naming every plugin whose
// required drift version the CLI cannot serve. cli is the CLI's release
// version; empty (a development build) skips the check. Plugins in the main
// module or in drift itself (first-party plugins) are skipped: the app's own
// drift requirement is checked through the bridge (checkBridgeDrift).
func checkPluginCompat(cli string, plugins []ConfiguredPlugin, infos []*PackageInfo) error {
	if cli == "" {
		return nil
	}
	skew := &VersionSkewError{CLI: cli}
	for i, p := range plugins {
		mod := infos[i].Module
		if mod.Main || mod.Path == protocol.DriftModulePath {
			continue
		}
		req, err := requiredDrift(mod)
		if err != nil {
			return fmt.Errorf("plugin %s: %w", p.Package, err)
		}
		r := PluginRequirement{Package: p.Package, Module: mod.String(), Drift: req}
		switch api := semver.MajorMinor(req); {
		case semver.Compare(api, semver.MajorMinor(cli)) > 0:
			skew.TooNew = append(skew.TooNew, r)
		case semver.Compare(api, semver.MajorMinor(templates.PluginAPIFloor)) < 0:
			skew.TooOld = append(skew.TooOld, r)
		}
	}
	if len(skew.TooNew) == 0 && len(skew.TooOld) == 0 {
		return nil
	}
	return skew
}

// requiredDrift reads the drift version mod's go.mod requires.
func requiredDrift(mod *ModuleInfo) (string, error) {
	if mod.GoMod == "" {
		return "", fmt.Errorf("go list reported no go.mod for module %s", mod)
	}
	data, err := os.ReadFile(mod.GoMod)
	if err != nil {
		return "", err
	}
	f, err := modfile.ParseLax(mod.GoMod, data, nil)
	if err != nil {
		return "", err
	}
	for _, r := range f.Require {
		if r.Mod.Path == protocol.DriftModulePath {
			if !semver.IsValid(r.Mod.Version) {
				return "", fmt.Errorf("%s requires %s at invalid version %q", mod.GoMod, protocol.DriftModulePath, r.Mod.Version)
			}
			return r.Mod.Version, nil
		}
	}
	return "", fmt.Errorf("%s does not require %s; a plugin module must", mod.GoMod, protocol.DriftModulePath)
}

// checkBridgeDrift fails when the bridge was compiled with a drift whose
// major.minor differs from the CLI's: its ops and the app's runtime may not
// match the CLI's templates. cli is the CLI's release version; empty (a
// development build) skips the comparison, as does a bridge built against a
// local drift checkout.
func checkBridgeDrift(cli, bridge string) error {
	if bridge == "" {
		return fmt.Errorf("plugin bridge reported no drift version: its github.com/go-drift/drift predates this CLI; " +
			"update the app's drift dependency")
	}
	if cli == "" || bridge == protocol.DevelVersion {
		return nil
	}
	if !semver.IsValid(bridge) {
		return fmt.Errorf("plugin bridge reported invalid drift version %q", bridge)
	}
	if semver.MajorMinor(bridge) == semver.MajorMinor(cli) {
		return nil
	}
	return fmt.Errorf(`The app builds with drift %s, but this CLI is %s; their plugin APIs differ.
Use the CLI matching the app:
  go install %s/cmd/drift@%s
or require drift %s.x in the app's go.mod.`,
		bridge, cli, protocol.DriftModulePath, bridge, semver.MajorMinor(cli))
}
