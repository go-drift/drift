// Command releasetool keeps Drift's modules releasable in lockstep. A
// release tags the root module (github.com/go-drift/drift) and every plugin
// module under plugins/ at one version, and every module in the repository
// requires those modules at that version.
//
// Usage, from anywhere in the repository:
//
//	releasetool bump <version>    require the released modules at <version> in every module, then go mod tidy them
//	releasetool check <version>   fail, naming each go.mod, unless every module requires them at <version>
//	releasetool tags <version>    print the tags a release of <version> creates, one per line
//	releasetool proxy -dir <dir> -rev <rev> <version>
//	                              add the released modules at <rev> to a file GOPROXY in <dir>, as <version>
//
// scripts/release.sh runs bump (prepare) and check; the release workflow
// runs check and tags; scripts/install-check.sh runs proxy.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "releasetool:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: releasetool bump|check|tags|proxy ...")
	}
	cmd, args := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	dir := fs.String("dir", "", "proxy: GOPROXY directory")
	rev := fs.String("rev", "HEAD", "proxy: git revision")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("%s: want exactly one version argument", cmd)
	}
	version, err := parseReleaseVersion(fs.Arg(0))
	if err != nil {
		return err
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}

	switch cmd {
	case "bump":
		mods, err := listModules(root, "")
		if err != nil {
			return err
		}
		return bump(root, mods, version)
	case "check":
		mods, err := listModules(root, "")
		if err != nil {
			return err
		}
		if bad := mismatches(mods, version); len(bad) > 0 {
			return fmt.Errorf("modules do not require the released modules at %s:\n  %s\nRun: scripts/release.sh prepare %s",
				version, strings.Join(bad, "\n  "), version)
		}
		return nil
	case "tags":
		mods, err := listModules(root, "")
		if err != nil {
			return err
		}
		for _, t := range tags(mods, version) {
			fmt.Println(t)
		}
		return nil
	case "proxy":
		if *dir == "" {
			return errors.New("proxy: -dir is required")
		}
		mods, err := listModules(root, *rev)
		if err != nil {
			return err
		}
		return writeProxy(*dir, root, *rev, mods, version)
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

// parseReleaseVersion accepts a canonical semantic version tag such as
// v0.27.0 or v0.27.0-rc.1: no build metadata, and no pseudo-version, which
// names a commit rather than a release.
func parseReleaseVersion(v string) (string, error) {
	if !semver.IsValid(v) || semver.Canonical(v) != v || module.IsPseudoVersion(v) {
		return "", fmt.Errorf("invalid release version %q: want vX.Y.Z or vX.Y.Z-prerelease", v)
	}
	return v, nil
}

func repoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
