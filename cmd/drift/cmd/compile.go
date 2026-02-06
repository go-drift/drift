package cmd

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/cache"
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
	} else {
		// Use managed build directory with hash (matches workspace.go)
		moduleRoot, err := workspace.BuildRoot(cfg)
		if err != nil {
			return err
		}
		hash := sha1.Sum([]byte(root))
		shortHash := hex.EncodeToString(hash[:6])
		buildDir = filepath.Join(moduleRoot, platform, shortHash)
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
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("iOS compilation requires macOS")
	}

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

	// Select SDK based on target
	sdk := "iphonesimulator"
	if device {
		sdk = "iphoneos"
	}

	clangPath, err := xcrunToolPath(sdk, "clang")
	if err != nil {
		return fmt.Errorf("failed to locate clang for %s: %w", sdk, err)
	}

	clangXXPath, err := xcrunToolPath(sdk, "clang++")
	if err != nil {
		return fmt.Errorf("failed to locate clang++ for %s: %w", sdk, err)
	}

	sdkRoot, err := xcrunSDKPath(sdk)
	if err != nil {
		return fmt.Errorf("failed to locate %s SDK: %w", sdk, err)
	}

	// Output directory for libraries
	var libDir string
	if ejected {
		libDir = filepath.Join(buildDir, "Runner")
	} else {
		libDir = filepath.Join(buildDir, "ios", "Runner")
	}
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return fmt.Errorf("failed to create library directory: %w", err)
	}

	// Find and copy Skia library if needed
	skiaPlatform := "ios-simulator"
	if device {
		skiaPlatform = "ios"
	}
	skiaLib, skiaDir, err := findSkiaLib(projectRoot, skiaPlatform, arch, opts.noFetch)
	if err != nil {
		return err
	}

	skiaVersion := cache.DriftSkiaVersion()
	skiaVersionFile := filepath.Join(libDir, ".drift-skia-version")
	if needsSkiaCopy(skiaVersionFile, skiaVersion) {
		fmt.Println("  Copying Skia library...")
		if err := copyFile(skiaLib, filepath.Join(libDir, "libdrift_skia.a")); err != nil {
			return fmt.Errorf("failed to copy Skia library: %w", err)
		}
		if err := os.WriteFile(skiaVersionFile, []byte(skiaVersion), 0o644); err != nil {
			return fmt.Errorf("failed to write Skia version marker: %w", err)
		}
	}

	libPath := filepath.Join(libDir, "libdrift.a")

	// CGO flags for iOS cross-compilation
	iosArch := "x86_64"
	if arch == "arm64" {
		iosArch = "arm64"
	}

	versionMinFlag := "-mios-simulator-version-min=14.0"
	if device {
		versionMinFlag = "-miphoneos-version-min=14.0"
	}
	cgoCflags := fmt.Sprintf("-isysroot %s -arch %s %s", sdkRoot, iosArch, versionMinFlag)
	cgoCxxflags := fmt.Sprintf("-isysroot %s -arch %s %s -std=c++17 -x objective-c++", sdkRoot, iosArch, versionMinFlag)

	cmd := exec.Command("go", "build",
		"-overlay", overlayPath,
		"-buildmode=c-archive",
		"-o", libPath,
		".")
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"GOOS=ios",
		"GOARCH="+arch,
		"CC="+clangPath,
		"CXX="+clangXXPath,
		"SDKROOT="+sdkRoot,
		"CGO_CFLAGS="+cgoCflags,
		"CGO_CXXFLAGS="+cgoCxxflags,
		"CGO_LDFLAGS="+iosSkiaLinkerFlags(skiaDir),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build Go library: %w", err)
	}

	fmt.Printf("Compiled: %s\n", libPath)
	return nil
}

func compileAndroid(projectRoot, buildDir, overlayPath string, ejected bool, opts compileOptions) error {
	fmt.Println("Compiling Go code for Android...")

	ndkHome := os.Getenv("ANDROID_NDK_HOME")
	if ndkHome == "" {
		ndkHome = os.Getenv("ANDROID_NDK_ROOT")
	}
	if ndkHome == "" {
		return fmt.Errorf("ANDROID_NDK_HOME or ANDROID_NDK_ROOT must be set")
	}

	checkNDKVersion(ndkHome)

	hostTag, err := detectNDKHostTag(ndkHome)
	if err != nil {
		return err
	}

	toolchain := filepath.Join(ndkHome, "toolchains", "llvm", "prebuilt", hostTag, "bin")
	sysrootLib := filepath.Join(ndkHome, "toolchains", "llvm", "prebuilt", hostTag, "sysroot", "usr", "lib")

	abis := []struct {
		abi      string
		goarch   string
		goarm    string
		cc       string
		triple   string
		skiaArch string
	}{
		{"arm64-v8a", "arm64", "", "aarch64-linux-android21-clang", "aarch64-linux-android", "arm64"},
		{"armeabi-v7a", "arm", "7", "armv7a-linux-androideabi21-clang", "arm-linux-androideabi", "arm"},
		{"x86_64", "amd64", "", "x86_64-linux-android21-clang", "x86_64-linux-android", "amd64"},
	}

	jniLibsDir := workspace.JniLibsDir(buildDir, ejected)

	for _, abi := range abis {
		fmt.Printf("  Compiling for %s...\n", abi.abi)

		// Find Skia library for this architecture (Skia is statically linked into libdrift.so)
		_, skiaDir, err := findSkiaLib(projectRoot, "android", abi.skiaArch, opts.noFetch)
		if err != nil {
			return err
		}

		outDir := filepath.Join(jniLibsDir, abi.abi)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		cmd := exec.Command("go", "build",
			"-overlay", overlayPath,
			"-buildmode=c-shared",
			"-o", filepath.Join(outDir, "libdrift.so"),
			".")
		cmd.Dir = projectRoot
		cmd.Env = append(os.Environ(),
			"CGO_ENABLED=1",
			"GOOS=android",
			"GOARCH="+abi.goarch,
			"CC="+filepath.Join(toolchain, abi.cc),
			"CXX="+filepath.Join(toolchain, abi.cc+"++"),
			"CGO_LDFLAGS="+androidSkiaLinkerFlags(skiaDir),
		)
		if abi.goarm != "" {
			cmd.Env = append(cmd.Env, "GOARM="+abi.goarm)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to build for %s: %w", abi.abi, err)
		}

		// Copy libc++_shared.so from Skia cache (bundled with matching NDK)
		cppShared := filepath.Join(skiaDir, "libc++_shared.so")
		if _, err := os.Stat(cppShared); err != nil {
			// Fallback to user's NDK (for custom DRIFT_SKIA_DIR or old cache)
			cppShared = filepath.Join(sysrootLib, abi.triple, "libc++_shared.so")
		}
		if _, err := os.Stat(cppShared); err == nil {
			if err := copyFile(cppShared, filepath.Join(outDir, "libc++_shared.so")); err != nil {
				return fmt.Errorf("failed to copy libc++_shared.so: %w", err)
			}
		}

		// Clean up header file
		os.Remove(filepath.Join(outDir, "libdrift.h"))
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
