// Command fileproxy adds a module to a file GOPROXY (the layout
// GOPROXY=file://<dir> reads), so scripts/install-check.sh can install a
// release before it is tagged.
//
// Usage:
//
//	fileproxy -dir <dir> [-repo <repo>] [-rev <rev>] <version>
//
// adds the module at the root of git repository <repo> (default: the one
// containing the working directory) at <rev> (default HEAD) to the proxy in
// <dir>, as <version>. Calls for further modules or versions add to the same
// proxy.
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
		fmt.Fprintln(os.Stderr, "fileproxy:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("fileproxy", flag.ContinueOnError)
	dir := fs.String("dir", "", "GOPROXY directory")
	repo := fs.String("repo", "", "git repository (default: the one containing the working directory)")
	rev := fs.String("rev", "HEAD", "git revision")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dir == "" {
		return errors.New("-dir is required")
	}
	if fs.NArg() != 1 {
		return errors.New("want exactly one version argument")
	}
	version, err := parseReleaseVersion(fs.Arg(0))
	if err != nil {
		return err
	}
	root, err := gitOutput(*repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	return writeProxy(*dir, strings.TrimSpace(string(root)), *rev, version)
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

// gitOutput runs git in dir (the working directory when empty).
func gitOutput(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}
