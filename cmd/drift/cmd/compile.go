package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/config"
	"github.com/go-drift/drift/cmd/drift/internal/workspace"
)

func init() {
	RegisterCommand(&Command{
		Name:  "compile",
		Short: "Compile Go code for platform",
		Long: `Compile Go code to native libraries for the specified platform.

This command is designed for IDE build hooks (Xcode Run Script, Gradle tasks)
that need to compile Go code before the native build step.

Platforms:
  ios       Compile to libdrift.a for iOS
  android   Compile to libdrift.so for Android (all ABIs)

Flags:
  --device     Build for physical device (iOS only, default: simulator)
  --no-fetch   Disable auto-download of missing Skia libraries

Output locations:
  Ejected:  ./platform/<platform>/
  Managed:  ~/.drift/build/<module>/<platform>/<hash>/

This command:
  1. Compiles Go code to static/shared library
  2. Generates bridge files
  3. iOS only: copies Skia library if missing or version mismatch
     (Android statically links Skia into libdrift.so)

When called from Xcode, automatically detects device vs simulator from
SDK_NAME environment variable.

Note: "drift compile all" is not supported. Compile targets a single platform
because IDE hooks call it for one platform at a time.`,
		Usage: "drift compile <ios|android> [--device] [--no-fetch]",
		Run:   runCompile,
	})
}

type compileOptions struct {
	noFetch bool
	device  bool
}

func runCompile(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("platform is required\n\nUsage: drift compile <ios|android> [--device] [--no-fetch]")
	}

	platform := strings.ToLower(args[0])
	opts := compileOptions{}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--no-fetch":
			opts.noFetch = true
		case "--device":
			opts.device = true
		}
	}

	if platform == "all" {
		return fmt.Errorf("'drift compile all' is not supported\n\nCompile one platform at a time: drift compile ios && drift compile android")
	}

	if platform != "ios" && platform != "android" {
		return fmt.Errorf("unknown platform %q (use ios or android)", platform)
	}

	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Resolve(root)
	if err != nil {
		return err
	}

	ejected := workspace.IsEjected(root, platform)

	var buildDir string
	if ejected {
		buildDir = workspace.EjectedBuildDir(root, platform)
		fmt.Printf("Using ejected %s project: %s\n", platform, buildDir)
		// Refresh .drift.env so driftw stays current if drift is reinstalled
		if err := workspace.WriteDriftEnv(buildDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update .drift.env: %v\n", err)
		}
	} else {
		var err error
		buildDir, err = workspace.ManagedBuildDir(root, cfg, platform)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(buildDir, 0o755); err != nil {
			return fmt.Errorf("failed to create build directory: %w", err)
		}
	}

	bridgeDir := workspace.BridgeDir(buildDir)
	if err := os.MkdirAll(bridgeDir, 0o755); err != nil {
		return fmt.Errorf("failed to create bridge directory: %w", err)
	}

	// Write bridge files
	if err := workspace.WriteBridgeFiles(bridgeDir, cfg); err != nil {
		return err
	}

	// Write overlay file for Go compilation
	overlayPath := filepath.Join(buildDir, "overlay.json")
	if err := workspace.WriteOverlay(overlayPath, bridgeDir, root); err != nil {
		return err
	}

	switch platform {
	case "ios":
		return compileIOS(root, buildDir, overlayPath, ejected, opts)
	case "android":
		return compileAndroid(root, buildDir, overlayPath, ejected, opts)
	default:
		return fmt.Errorf("unknown platform %q", platform)
	}
}

func compileIOS(projectRoot, buildDir, overlayPath string, ejected bool, opts compileOptions) error {
	// Detect device vs simulator from Xcode environment or --device flag
	// Xcode sets SDK_NAME to "iphoneos17.0" or "iphonesimulator17.0"
	device := opts.device
	arch := runtime.GOARCH

	if sdkName := os.Getenv("SDK_NAME"); sdkName != "" {
		device = strings.HasPrefix(sdkName, "iphoneos")
		// Use ARCHS from Xcode if available
		if archs := os.Getenv("ARCHS"); archs != "" {
			// ARCHS can be "arm64" or "x86_64" or space-separated
			archList := strings.Fields(archs)
			if len(archList) > 0 {
				switch archList[0] {
				case "arm64":
					arch = "arm64"
				case "x86_64":
					arch = "amd64"
				}
			}
		}
	}

	target := "iOS Simulator"
	if device {
		target = "iOS Device"
		arch = "arm64" // Physical devices are always arm64
	}

	fmt.Printf("Compiling Go code for %s (%s)...\n", target, arch)

	var libDir string
	if ejected {
		libDir = filepath.Join(buildDir, "Runner")
	} else {
		libDir = filepath.Join(buildDir, "ios", "Runner")
	}

	if err := compileGoForIOS(iosCompileConfig{
		projectRoot: projectRoot,
		overlayPath: overlayPath,
		libDir:      libDir,
		device:      device,
		arch:        arch,
		noFetch:     opts.noFetch,
	}); err != nil {
		return err
	}

	fmt.Printf("Compiled: %s\n", filepath.Join(libDir, "libdrift.a"))
	return nil
}

func compileAndroid(projectRoot, buildDir, overlayPath string, ejected bool, opts compileOptions) error {
	fmt.Println("Compiling Go code for Android...")

	jniLibsDir := workspace.JniLibsDir(buildDir, ejected)

	if err := compileGoForAndroid(androidCompileConfig{
		projectRoot: projectRoot,
		overlayPath: overlayPath,
		jniLibsDir:  jniLibsDir,
		noFetch:     opts.noFetch,
	}); err != nil {
		return err
	}

	fmt.Printf("Compiled to: %s\n", jniLibsDir)
	return nil
}

func needsSkiaCopy(versionFile, expectedVersion string) bool {
	if expectedVersion == "" {
		return true // Always copy for dev builds
	}

	data, err := os.ReadFile(versionFile)
	if err != nil {
		return true // File doesn't exist, copy needed
	}

	return strings.TrimSpace(string(data)) != expectedVersion
}
