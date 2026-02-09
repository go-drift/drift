package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/cache"
	"github.com/go-drift/drift/cmd/drift/internal/config"
	"github.com/go-drift/drift/cmd/drift/internal/workspace"
	"github.com/go-drift/drift/cmd/drift/internal/xtool"
)

func init() {
	RegisterCommand(&Command{
		Name:  "build",
		Short: "Build for iOS or Android",
		Long: `Build the Drift application for the specified platform.

Supported platforms:
  android   Build for Android (APK)
  ios       Build for iOS (requires macOS)
  xtool     Build for iOS using xtool (Linux/macOS, no Xcode required)

Flags:
  --release          Build a release version (default: debug)
  --device           Build for physical iOS device (default: simulator)
  --team-id TEAM_ID  Apple Developer Team ID for code signing (required for device)
  --no-fetch         Disable auto-download of missing Skia libraries

Skia libraries are automatically downloaded when missing. Use --no-fetch to
disable this behavior and fail with an error instead.

Set DRIFT_SKIA_DIR to use a custom Skia library location. The directory must
contain the standard {platform}/{arch}/libdrift_skia.a structure.

For xtool builds:
  drift build xtool                Build debug for device
  drift build xtool --release      Build release for device

To find your Team ID, run: grep -r "DEVELOPMENT_TEAM" ~/Library/MobileDevice/Provisioning\ Profiles/
Or check Xcode -> Settings -> Accounts -> select team -> View Details`,
		Usage: "drift build <platform> [--release] [--device] [--team-id TEAM_ID] [--no-fetch]",
		Run:   runBuild,
	})
}

type buildOptions struct {
	noFetch bool
	ejected bool
}

type iosBuildOptions struct {
	buildOptions
	release bool
	device  bool
	teamID  string
}

type xtoolBuildOptions struct {
	buildOptions
	release bool
	device  bool
}

type androidBuildOptions struct {
	buildOptions
	release bool
}

func runBuild(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("platform is required (android, ios, or xtool)\n\nUsage: drift build <platform>")
	}

	platform := strings.ToLower(args[0])
	androidOpts := androidBuildOptions{}
	iosOpts := iosBuildOptions{}
	xtoolOpts := xtoolBuildOptions{}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--release":
			androidOpts.release = true
			iosOpts.release = true
			xtoolOpts.release = true
		case "--device":
			iosOpts.device = true
			xtoolOpts.device = true
		case "--no-fetch":
			androidOpts.noFetch = true
			iosOpts.noFetch = true
			xtoolOpts.noFetch = true
		case "--team-id":
			if i+1 < len(args) {
				iosOpts.teamID = args[i+1]
				i++
			}
		}
	}

	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Resolve(root)
	if err != nil {
		return err
	}

	// Check if platform is ejected
	ejected := workspace.IsEjected(root, platform)

	ws, err := workspace.Prepare(root, cfg, platform)
	if err != nil {
		return err
	}

	switch platform {
	case "android":
		androidOpts.ejected = ejected
		return buildAndroid(ws, androidOpts)
	case "ios":
		iosOpts.ejected = ejected
		return buildIOS(ws, iosOpts)
	case "xtool":
		xtoolOpts.ejected = ejected
		return buildXtool(ws, xtoolOpts)
	default:
		return fmt.Errorf("unknown platform %q (use android, ios, or xtool)", platform)
	}
}

// buildAndroid builds the Android application.
func buildAndroid(ws *workspace.Workspace, opts androidBuildOptions) error {
	fmt.Println("Building for Android...")

	jniLibsDir := workspace.JniLibsDir(ws.BuildDir, opts.ejected)

	if err := compileGoForAndroid(androidCompileConfig{
		projectRoot: ws.Root,
		overlayPath: ws.Overlay,
		jniLibsDir:  jniLibsDir,
		noFetch:     opts.noFetch,
	}); err != nil {
		return err
	}

	fmt.Println("  Building APK...")

	gradlewName := "gradlew"
	if runtime.GOOS == "windows" {
		gradlewName = "gradlew.bat"
	}

	androidDir := ws.AndroidDir
	gradlew := filepath.Join(androidDir, gradlewName)
	if _, err := os.Stat(gradlew); err != nil {
		fmt.Println("  Note: Gradle wrapper not found, falling back to 'gradle' from PATH")
		gradlew = "gradle"
		if _, lookErr := exec.LookPath(gradlew); lookErr != nil {
			return fmt.Errorf("gradle not found in PATH; install Gradle or add a wrapper in %s", androidDir)
		}
	}

	buildTask := "assembleDebug"
	if opts.release {
		buildTask = "assembleRelease"
	}

	cmd := exec.Command(gradlew, buildTask)
	cmd.Dir = androidDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gradle build failed: %w", err)
	}

	apkDir := filepath.Join(ws.AndroidDir, "app", "build", "outputs", "apk")
	variant := "debug"
	if opts.release {
		variant = "release"
	}
	apkPath := filepath.Join(apkDir, variant, fmt.Sprintf("app-%s.apk", variant))

	fmt.Println()
	fmt.Printf("Build successful: %s\n", apkPath)

	return nil
}

// buildIOS builds the iOS application.
// If opts.device is true, builds for physical device (iphoneos SDK), otherwise simulator.
func buildIOS(ws *workspace.Workspace, opts iosBuildOptions) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("iOS builds require macOS")
	}

	arch := runtime.GOARCH
	target := "iOS Simulator"
	if opts.device {
		target = "iOS Device"
		arch = "arm64"
	} else {
		switch runtime.GOARCH {
		case "amd64", "arm64":
			// OK
		default:
			return fmt.Errorf("unsupported host architecture %q for iOS simulator", runtime.GOARCH)
		}
	}

	fmt.Printf("Building for %s...\n", target)
	fmt.Println("  Compiling Go code...")

	iosDir := filepath.Join(ws.IOSDir, "Runner")

	if err := compileGoForIOS(iosCompileConfig{
		projectRoot: ws.Root,
		overlayPath: ws.Overlay,
		libDir:      iosDir,
		device:      opts.device,
		arch:        arch,
		noFetch:     opts.noFetch,
	}); err != nil {
		return err
	}

	configuration := "Debug"
	if opts.release {
		configuration = "Release"
	}

	xcodeproj := filepath.Join(ws.IOSDir, "Runner.xcodeproj")
	if _, err := os.Stat(xcodeproj); os.IsNotExist(err) {
		fmt.Println()
		fmt.Println("Note: No Xcode project found in the generated workspace.")
		fmt.Printf("  Create one using Xcode in %s and re-run the build.\n", ws.IOSDir)
		return fmt.Errorf("xcode project setup required")
	}

	var buildArgs []string
	if opts.device {
		// Device build requires team ID for code signing
		if opts.teamID == "" {
			fmt.Println()
			fmt.Println("Error: --team-id is required for device builds.")
			fmt.Println()
			fmt.Println("To find your Team ID:")
			fmt.Println("  1. Open Xcode -> Settings -> Accounts")
			fmt.Println("  2. Select your Apple ID and team")
			fmt.Println("  3. The Team ID is shown in parentheses, e.g., 'My Team (ABC123XYZ)'")
			fmt.Println()
			fmt.Println("Then run: drift build ios --device --team-id ABC123XYZ")
			return fmt.Errorf("team ID required for device builds")
		}

		buildArgs = []string{
			"-project", xcodeproj,
			"-scheme", "Runner",
			"-configuration", configuration,
			"-destination", "generic/platform=iOS",
			"-allowProvisioningUpdates",
			"DEVELOPMENT_TEAM=" + opts.teamID,
			"build",
		}
	} else {
		buildArgs = []string{
			"-project", xcodeproj,
			"-scheme", "Runner",
			"-configuration", configuration,
			"-destination", "generic/platform=iOS Simulator",
		}
		buildArgs = append(buildArgs, simulatorArchBuildSettings()...)
		buildArgs = append(buildArgs, "build")
	}

	cmd := exec.Command("xcodebuild", buildArgs...)
	cmd.Dir = ws.IOSDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xcodebuild failed: %w", err)
	}

	fmt.Println()
	fmt.Println("Build successful!")

	return nil
}

func xcrunToolPath(sdk, tool string) (string, error) {
	output, err := exec.Command("xcrun", "--sdk", sdk, "--find", tool).Output()
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(output))
	if path == "" {
		return "", fmt.Errorf("xcrun returned empty path for %s", tool)
	}
	return path, nil
}

func xcrunSDKPath(sdk string) (string, error) {
	output, err := exec.Command("xcrun", "--sdk", sdk, "--show-sdk-path").Output()
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(output))
	if path == "" {
		return "", fmt.Errorf("xcrun returned empty sdk path")
	}
	return path, nil
}

func simulatorArchBuildSettings() []string {
	switch runtime.GOARCH {
	case "amd64":
		return []string{"ARCHS=x86_64"}
	case "arm64":
		return []string{"ARCHS=arm64"}
	default:
		return nil
	}
}

func iosSkiaLinkerFlags(skiaDir string) string {
	return strings.Join([]string{
		"-L" + skiaDir,
		"-ldrift_skia",
		"-lc++",
		"-framework Metal",
		"-framework CoreGraphics",
		"-framework Foundation",
		"-framework UIKit",
	}, " ")
}

func androidSkiaLinkerFlags(skiaDir string) string {
	return strings.Join([]string{
		"-L" + skiaDir,
		"-ldrift_skia",
		"-lc++_shared",
		"-lGLESv2",
		"-lEGL",
		"-landroid",
		"-llog",
		"-lm",
	}, " ")
}

func checkNDKVersion(ndkHome string) {
	propsPath := filepath.Join(ndkHome, "source.properties")
	data, err := os.ReadFile(propsPath)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Pkg.Revision") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			majorStr := strings.Split(strings.TrimSpace(parts[1]), ".")[0]
			var major int
			if _, err := fmt.Sscanf(majorStr, "%d", &major); err == nil && major < 25 {
				fmt.Printf("Warning: NDK r%d detected. Recommended r25 or newer.\n", major)
			}
			return
		}
	}
}

// detectNDKHostTag determines the NDK prebuilt toolchain directory for the current host.
// On Apple Silicon, falls back to darwin-x86_64 (Rosetta) if darwin-arm64 isn't available.
func detectNDKHostTag(ndkHome string) (string, error) {
	prebuiltBase := filepath.Join(ndkHome, "toolchains", "llvm", "prebuilt")

	// Candidates in order of preference
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			candidates = []string{"darwin-arm64", "darwin-x86_64"}
		} else {
			candidates = []string{"darwin-x86_64"}
		}
	case "windows":
		candidates = []string{"windows-x86_64"}
	default: // linux
		// Note: Linux arm64 has no Rosetta equivalent, so no fallback to x86_64
		if runtime.GOARCH == "arm64" {
			candidates = []string{"linux-aarch64"}
		} else {
			candidates = []string{"linux-x86_64"}
		}
	}

	for _, tag := range candidates {
		path := filepath.Join(prebuiltBase, tag)
		if _, err := os.Stat(path); err == nil {
			return tag, nil
		}
	}

	return "", fmt.Errorf("no NDK toolchain found in %s (tried: %v)", prebuiltBase, candidates)
}

func findSkiaLib(projectRoot, platform, arch string, noFetch bool) (string, string, error) {
	// Check DRIFT_SKIA_DIR override first (highest priority)
	if skiaBase := os.Getenv("DRIFT_SKIA_DIR"); skiaBase != "" {
		dir := filepath.Join(skiaBase, platform, arch)
		lib := filepath.Join(dir, "libdrift_skia.a")
		if _, err := os.Stat(lib); err == nil {
			fmt.Printf("Using Skia lib (DRIFT_SKIA_DIR): %s\n", lib)
			return lib, dir, nil
		}
		// If env var is set but path doesn't exist, give clear error
		return "", "", fmt.Errorf("DRIFT_SKIA_DIR set to %q but library not found at %s\n\nExpected structure: %s/{platform}/{arch}/libdrift_skia.a", skiaBase, lib, skiaBase)
	}

	// Find drift module root from this source file's location
	// build.go is at cmd/drift/cmd/build.go, so go up 3 levels
	_, thisFile, _, _ := runtime.Caller(0)
	driftRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")

	candidates := []string{
		// Drift module's third_party (source builds take priority)
		filepath.Join(driftRoot, "third_party", "drift_skia", platform, arch),
		// Project-relative (user customization)
		filepath.Join(projectRoot, "third_party", "drift_skia", platform, arch),
	}

	// Versioned lib path from cache
	if libDir, err := cache.LibDir(platform, arch); err == nil {
		candidates = append(candidates, libDir)
	}

	for _, dir := range candidates {
		lib := filepath.Join(dir, "libdrift_skia.a")
		if _, err := os.Stat(lib); err == nil {
			fmt.Printf("Using Skia lib: %s\n", lib)
			return lib, dir, nil
		}
	}

	// Library not found - try auto-fetch if enabled
	if !noFetch {
		// Determine which platform to fetch (ios-simulator maps to ios tarball)
		fetchPlatform := platform
		if platform == "ios-simulator" {
			fetchPlatform = "ios"
		}

		fmt.Printf("Skia library not found for %s/%s. Downloading...\n", platform, arch)

		opts := FetchSkiaOptions{}
		switch fetchPlatform {
		case "android":
			opts.Android = true
		case "ios":
			opts.IOS = true
		}

		if err := FetchSkia(context.Background(), opts); err != nil {
			return "", "", fmt.Errorf("auto-fetch failed: %w\n\nRun 'drift fetch-skia' to download prebuilt binaries", err)
		}

		// Retry finding the library after fetch
		if libDir, err := cache.LibDir(platform, arch); err == nil {
			lib := filepath.Join(libDir, "libdrift_skia.a")
			if _, err := os.Stat(lib); err == nil {
				return lib, libDir, nil
			}
		}
	}

	return "", "", fmt.Errorf("drift skia library not found for %s/%s\n\nRun 'drift fetch-skia' to download prebuilt binaries", platform, arch)
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// copyDir recursively copies a directory from src to dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Copy file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}

// buildXtool builds the iOS application using xtool (Linux/macOS, no Xcode).
func buildXtool(ws *workspace.Workspace, opts xtoolBuildOptions) error {
	fmt.Println("Building for iOS using xtool...")

	// Detect xtool SDK
	cfg, err := xtool.Detect()
	if err != nil {
		return err
	}

	fmt.Println("  Compiling Go code...")

	// Prepare libraries directory
	cdriftDir := filepath.Join(ws.XtoolDir, "Libraries", "CDrift")
	cskiaDir := filepath.Join(ws.XtoolDir, "Libraries", "CSkia")

	if err := os.MkdirAll(cdriftDir, 0o755); err != nil {
		return fmt.Errorf("failed to create CDrift directory: %w", err)
	}
	if err := os.MkdirAll(cskiaDir, 0o755); err != nil {
		return fmt.Errorf("failed to create CSkia directory: %w", err)
	}

	// Find Skia library for iOS device (arm64)
	skiaLib, _, err := findSkiaLib(ws.Root, "ios", "arm64", opts.noFetch)
	if err != nil {
		return err
	}

	// Build Go code as static library
	libPath := filepath.Join(cdriftDir, "libdrift.a")
	headerPath := filepath.Join(cdriftDir, "libdrift.h")

	cmd := exec.Command("go", "build",
		"-overlay", ws.Overlay,
		"-buildmode=c-archive",
		"-o", libPath,
		".")
	cmd.Dir = ws.Root
	cmd.Env = append(os.Environ(), cfg.CGOEnv()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build Go library: %w", err)
	}

	// Add archive index for Darwin linker (ld64 requires it)
	ranlibCmd := exec.Command("llvm-ranlib", libPath)
	ranlibCmd.Stdout = os.Stdout
	ranlibCmd.Stderr = os.Stderr
	if err := ranlibCmd.Run(); err != nil {
		// Try regular ranlib as fallback
		ranlibCmd = exec.Command("ranlib", libPath)
		ranlibCmd.Stdout = os.Stdout
		ranlibCmd.Stderr = os.Stderr
		if err := ranlibCmd.Run(); err != nil {
			return fmt.Errorf("failed to run ranlib on library (install llvm): %w", err)
		}
	}

	// The c-archive build mode generates a header file next to the archive
	// Move it if it ended up in the wrong place
	generatedHeader := filepath.Join(ws.Root, "libdrift.h")
	if _, err := os.Stat(generatedHeader); err == nil {
		if err := os.Rename(generatedHeader, headerPath); err != nil {
			return fmt.Errorf("failed to move header file: %w", err)
		}
	}

	// Copy Skia library
	fmt.Println("  Copying Skia library...")
	if err := copyFile(skiaLib, filepath.Join(cskiaDir, "libdrift_skia.a")); err != nil {
		return fmt.Errorf("failed to copy Skia library: %w", err)
	}

	// Build Swift package using xtool (handles cross-compilation properly)
	fmt.Println("  Building Swift package with xtool...")

	xtoolArgs := cfg.XtoolBuildArgs(opts.release)
	cmd = exec.Command(cfg.XtoolPath, xtoolArgs...)
	cmd.Dir = ws.XtoolDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xtool build failed: %w", err)
	}

	// xtool creates the app bundle - find it
	// xtool outputs to: xtool/<appname>.app
	appName := ws.Config.AppName
	xtoolAppDir := filepath.Join(ws.XtoolDir, "xtool", appName+".app")

	// Verify xtool output exists
	if _, err := os.Stat(xtoolAppDir); err != nil {
		return fmt.Errorf("xtool app bundle not found at %s: %w", xtoolAppDir, err)
	}

	// Copy to standard location for easier access
	appDir := filepath.Join(ws.XtoolDir, "Runner.app")
	if err := os.RemoveAll(appDir); err != nil {
		return fmt.Errorf("failed to clean app bundle: %w", err)
	}
	if err := copyDir(xtoolAppDir, appDir); err != nil {
		return fmt.Errorf("failed to copy app bundle: %w", err)
	}

	fmt.Println()
	fmt.Printf("Build successful: %s\n", appDir)

	return nil
}
