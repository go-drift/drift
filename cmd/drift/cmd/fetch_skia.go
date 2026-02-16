package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/cache"
	"github.com/go-drift/drift/cmd/drift/internal/fetch"
)

func init() {
	RegisterCommand(&Command{
		Name:  "fetch-skia",
		Short: "Download prebuilt Skia libraries",
		Long: `Download prebuilt Drift Skia libraries from GitHub Releases.

By default, downloads libraries for both Android and iOS platforms.
Use --android or --ios to download only one platform.

The version is determined in this order:
  1. --version flag
  2. DRIFT_VERSION environment variable
  3. CLI version (for release builds)
  4. Latest release from GitHub (fallback)

Libraries are stored in: ~/.drift/lib/<version>/<platform>/<arch>/`,
		Usage: "drift fetch-skia [--android] [--ios] [--version VERSION]",
		Run:   runFetchSkia,
	})
}

// FetchSkiaOptions configures which platforms to fetch.
type FetchSkiaOptions struct {
	Android bool
	IOS     bool
	Version string // Override version (empty = auto-detect)
}

func runFetchSkia(args []string) error {
	opts := FetchSkiaOptions{}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--android":
			opts.Android = true
		case "--ios":
			opts.IOS = true
		case "--version":
			if i+1 < len(args) {
				opts.Version = args[i+1]
				i++
			} else {
				return fmt.Errorf("--version requires a value")
			}
		default:
			if after, ok := strings.CutPrefix(args[i], "--version="); ok {
				opts.Version = after
			} else {
				return fmt.Errorf("unknown flag: %s", args[i])
			}
		}
	}

	// Default to both platforms if neither specified
	if !opts.Android && !opts.IOS {
		opts.Android = true
		opts.IOS = true
	}

	return FetchSkia(context.Background(), opts)
}

// FetchSkia downloads prebuilt Skia libraries for the specified platforms.
// This function is exported so it can be called from the build command.
func FetchSkia(ctx context.Context, opts FetchSkiaOptions) error {
	d := fetch.DefaultDownloader()

	// Resolve version with normalization at each step.
	// Values like "drift-v0.2.0" or pseudo-versions are normalized;
	// if normalization returns empty, fall back to next option.
	var version string

	// If user explicitly passed --version, require it to be valid
	if opts.Version != "" {
		version = cache.NormalizeVersion(opts.Version)
		if version == "" {
			return fmt.Errorf("invalid version %q (pseudo-versions and dev builds are not supported)\n\nUse a release version like v0.2.0 or omit --version to fetch latest", opts.Version)
		}
	}

	if version == "" {
		version = cache.NormalizeVersion(os.Getenv("DRIFT_VERSION"))
	}
	if version == "" {
		version = cache.NormalizeVersion(Version)
	}
	if version == "" {
		fmt.Println("Fetching latest release version from GitHub...")
		latest, err := fetch.FetchLatestRelease(ctx, d)
		if err != nil {
			return fmt.Errorf("failed to determine version: %w\n\nSet DRIFT_VERSION or use --version flag", err)
		}
		// Normalize the tag in case it has drift- prefix
		version = cache.NormalizeVersion(latest)
		if version == "" {
			return fmt.Errorf("latest release tag %q is not a valid version", latest)
		}
	}

	fmt.Printf("Fetching Drift Skia %s...\n", version)

	// Fetch manifest
	fmt.Println("  Downloading manifest...")
	manifest, err := fetch.FetchManifest(ctx, d, version)
	if err != nil {
		return err
	}

	if manifest.NDKVersion != "" {
		fmt.Printf("  Skia libraries built with NDK r%s\n", manifest.NDKVersion)
	}

	// Determine output directory
	cacheRoot, err := cache.Root()
	if err != nil {
		return err
	}
	libDir := filepath.Join(cacheRoot, "lib", version)

	// Download and extract platforms
	platforms := []struct {
		name     string
		enabled  bool
		manifest *fetch.PlatformManifest
	}{
		{"android", opts.Android, manifest.Android},
		{"ios", opts.IOS, manifest.IOS},
	}

	for _, p := range platforms {
		if !p.enabled {
			continue
		}
		if p.manifest == nil {
			fmt.Printf("  Warning: no %s artifact in manifest, skipping\n", p.name)
			continue
		}

		if err := fetchPlatform(ctx, d, version, p.name, p.manifest.SHA256, libDir); err != nil {
			return fmt.Errorf("failed to fetch %s: %w", p.name, err)
		}
	}

	fmt.Printf("Drift Skia artifacts extracted to %s\n", libDir)
	return nil
}

func fetchPlatform(ctx context.Context, d *fetch.Downloader, version, platform, expectedSHA256, libDir string) error {
	tarballName := fmt.Sprintf("drift-%s-%s.tar.gz", version, platform)
	url := fetch.TarballURL(version, platform)

	// Create temp file for download
	tmpDir, err := os.MkdirTemp("", "drift-fetch-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	tarPath := filepath.Join(tmpDir, tarballName)

	fmt.Printf("  Downloading %s...\n", tarballName)
	if err := d.Download(ctx, url, tarPath); err != nil {
		return err
	}

	// Verify checksum
	if err := fetch.VerifyChecksum(tarPath, expectedSHA256); err != nil {
		return err
	}

	fmt.Printf("  Extracting %s...\n", platform)
	if err := extractTarGz(tarPath, libDir); err != nil {
		return fmt.Errorf("failed to extract tarball: %w", err)
	}

	return nil
}

func extractTarGz(tarPath, destDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Validate and clean path to prevent directory traversal
		if !isValidTarPath(header.Name) {
			continue
		}

		cleanName := filepath.Clean(header.Name)
		target := filepath.Join(destDir, cleanName)

		// Final safety check: ensure target is within destDir
		if !strings.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}

// isValidTarPath checks if a tar entry path is safe to extract.
func isValidTarPath(name string) bool {
	// Reject empty names
	if name == "" {
		return false
	}

	// Reject absolute paths
	if filepath.IsAbs(name) {
		return false
	}

	// Clean the path and check for directory traversal
	clean := filepath.Clean(name)

	// Reject paths that escape the root (start with ..)
	if strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
		return false
	}

	return true
}
