// Package cache provides centralized cache directory resolution for Drift.
//
// Priority order: --cache-dir flag > DRIFT_CACHE_DIR env > ~/.drift default.
package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var global struct {
	version    string
	rawVersion string
	cacheDir   string
	warned     bool
}

// SetGlobal initializes the cache resolver with the CLI version.
// This should be called at startup from root.go.
func SetGlobal(version string) {
	global.rawVersion = strings.TrimSpace(version)
	global.version = NormalizeVersion(version)
}

// NormalizeVersion returns a clean release version, or empty if the version
// is not a valid release (e.g., dev builds, pseudo-versions from go install).
// Explicit prerelease tags (v0.2.0-rc1) are allowed.
//
// Examples:
//
//	"v0.1.0"                          -> "v0.1.0"
//	"0.1.0"                           -> "v0.1.0"
//	"drift-v0.1.0"                    -> "v0.1.0"
//	"v0.2.0-rc1"                      -> "v0.2.0-rc1" (prerelease allowed)
//	"0.1.0-dev"                       -> "" (dev build)
//	"v0.2.1-0.20260122153045-abc123"  -> "" (pseudo-version)
func NormalizeVersion(version string) string {
	version = strings.TrimPrefix(version, "drift-")

	// Reject -dev builds
	if strings.HasSuffix(version, "-dev") {
		return ""
	}

	// Reject Go pseudo-versions (v0.2.1-0.20260122153045-abc123)
	if strings.Contains(version, "-0.") {
		return ""
	}

	// Must have X.Y.Z format (with optional prerelease suffix)
	base := version
	if idx := strings.Index(version, "-"); idx != -1 {
		base = version[:idx]
	}
	base = strings.TrimPrefix(base, "v")
	if strings.Count(base, ".") != 2 {
		return ""
	}

	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	return version
}

// SetCacheDir sets an override for the cache directory.
// This is typically called when parsing the --cache-dir flag.
func SetCacheDir(dir string) {
	global.cacheDir = dir
}

// Root returns the cache root directory.
// Priority: --cache-dir flag > DRIFT_CACHE_DIR env > ~/.drift default.
func Root() (string, error) {
	if global.cacheDir != "" {
		return global.cacheDir, nil
	}

	if envDir := os.Getenv("DRIFT_CACHE_DIR"); envDir != "" {
		return envDir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}

	return filepath.Join(home, ".drift"), nil
}

// BuildRoot returns the build cache directory for a module.
// Returns: <cache_root>/build/<module_slug>
func BuildRoot(modulePath string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}

	moduleSlug := strings.ReplaceAll(modulePath, "/", "_")
	return filepath.Join(root, "build", moduleSlug), nil
}

// LibDir returns the versioned library directory for prebuilt binaries.
// Returns: <cache_root>/lib/<version>/<platform>/<arch>
//
// If the CLI version is not a release (dev build, pseudo-version), this
// searches for any available version in the cache and prints a warning.
func LibDir(platform, arch string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}

	version := global.version
	if version == "" {
		// Non-release build: find any cached version
		version, err = findCachedVersion(root, platform, arch)
		if err != nil {
			return "", err
		}
		if !global.warned {
			raw := global.rawVersion
			if raw == "" {
				raw = "unknown"
			}
			fmt.Fprintf(os.Stderr, "Warning: using cached libraries (%s) for non-release CLI version %s\n", version, raw)
			global.warned = true
		}
	}

	return filepath.Join(root, "lib", version, platform, arch), nil
}

// findCachedVersion looks for available versions in the cache that have
// the requested platform/arch. Returns the highest semver version found.
func findCachedVersion(root, platform, arch string) (string, error) {
	libDir := filepath.Join(root, "lib")
	entries, err := os.ReadDir(libDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("cache directory %s does not exist; run 'drift fetch-skia'", libDir)
		}
		return "", fmt.Errorf("failed to read cache directory %s: %w", libDir, err)
	}

	var candidates []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		lib := filepath.Join(libDir, entry.Name(), platform, arch, "libdrift_skia.a")
		if _, err := os.Stat(lib); err == nil {
			candidates = append(candidates, entry.Name())
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no cached libraries found for %s/%s; run 'drift fetch-skia'", platform, arch)
	}

	// Pick highest semver version
	best := candidates[0]
	for _, v := range candidates[1:] {
		if semverCompare(v, best) > 0 {
			best = v
		}
	}

	return best, nil
}

// semver represents a parsed semantic version.
type semver struct {
	major, minor, patch int
	valid               bool
}

// parseSemver parses a version string like "v1.2.3" into components.
// Prerelease versions (v1.2.3-rc1) are treated as invalid since parsing
// the prerelease portion is not implemented. They will sort before valid
// versions, so the highest clean release is preferred.
func parseSemver(version string) semver {
	version = strings.TrimPrefix(version, "v")
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return semver{}
	}

	var s semver
	if _, err := fmt.Sscanf(parts[0], "%d", &s.major); err != nil {
		return semver{}
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &s.minor); err != nil {
		return semver{}
	}
	if _, err := fmt.Sscanf(parts[2], "%d", &s.patch); err != nil {
		return semver{}
	}
	s.valid = true
	return s
}

// semverCompare compares two version strings.
// Returns: -1 if a < b, 0 if a == b, 1 if a > b.
// Invalid versions (including prereleases) sort before valid ones.
func semverCompare(a, b string) int {
	sa, sb := parseSemver(a), parseSemver(b)

	if !sa.valid && !sb.valid {
		return strings.Compare(a, b)
	}
	if !sa.valid {
		return -1 // a sorts before b
	}
	if !sb.valid {
		return 1 // b sorts before a
	}

	if sa.major != sb.major {
		return intCompare(sa.major, sb.major)
	}
	if sa.minor != sb.minor {
		return intCompare(sa.minor, sb.minor)
	}
	return intCompare(sa.patch, sb.patch)
}

func intCompare(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// DriftSkiaVersion returns the drift CLI version used as the Skia library version tag.
// This is used to check if ejected projects have matching Skia libraries.
func DriftSkiaVersion() string {
	if global.version != "" {
		return global.version
	}
	return global.rawVersion
}
