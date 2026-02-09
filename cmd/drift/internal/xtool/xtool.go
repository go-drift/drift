// Package xtool provides SDK detection and configuration for building iOS apps using xtool.
// This enables cross-compilation of iOS applications from Linux using Swift Package Manager.
package xtool

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config holds the xtool SDK configuration for iOS cross-compilation.
type Config struct {
	XtoolPath string // Path to xtool binary
	SDKPath   string // Path to iPhoneOS.sdk
	ClangPath string // Path to xtool's clang
}

// Detect locates xtool and the iOS SDK, returning a configuration for cross-compilation.
func Detect() (*Config, error) {
	xtoolPath, err := findXtool()
	if err != nil {
		return nil, err
	}

	sdkPath, err := findSDK()
	if err != nil {
		return nil, err
	}

	clangPath, err := findClang(sdkPath)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		XtoolPath: xtoolPath,
		SDKPath:   sdkPath,
		ClangPath: clangPath,
	}

	return cfg, nil
}

// CGOEnv returns environment variables for CGO cross-compilation to iOS arm64.
func (c *Config) CGOEnv() []string {
	// Use --target for cross-compilation (works on Linux clang)
	// -arch is macOS-specific and won't work on Linux
	target := "--target=arm64-apple-ios14.0"
	cflags := fmt.Sprintf("%s -isysroot %s", target, c.SDKPath)
	// Use -x objective-c++ to compile .mm files correctly
	// Also add framework search path
	frameworkPath := filepath.Join(c.SDKPath, "System", "Library", "Frameworks")
	cxxflags := fmt.Sprintf("%s -isysroot %s -std=c++17 -x objective-c++ -fobjc-arc -F%s", target, c.SDKPath, frameworkPath)
	ldflags := fmt.Sprintf("%s -isysroot %s -F%s", target, c.SDKPath, frameworkPath)

	return []string{
		"CGO_ENABLED=1",
		"GOOS=ios",
		"GOARCH=arm64",
		"CC=" + c.ClangPath,
		"CXX=" + c.ClangPath + "++",
		"CGO_CFLAGS=" + cflags,
		"CGO_CXXFLAGS=" + cxxflags,
		"CGO_LDFLAGS=" + ldflags,
	}
}

// SwiftBuildEnv returns environment variables for swift build with iOS SDK.
// Note: We intentionally don't set SDKROOT here because that would affect
// the Package.swift manifest compilation (which must run on the host).
// The SDK is passed via --sdk argument instead.
func (c *Config) SwiftBuildEnv() []string {
	return []string{}
}

// XtoolBuildArgs returns arguments for xtool dev build targeting iOS.
// xtool handles cross-compilation properly unlike raw swift build.
func (c *Config) XtoolBuildArgs(release bool) []string {
	args := []string{"dev", "build"}
	if release {
		args = append(args, "-c", "release")
	}
	return args
}

func findXtool() (string, error) {
	// Check PATH first
	if path, err := exec.LookPath("xtool"); err == nil {
		return path, nil
	}

	// Check common locations
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}

	candidates := []string{
		filepath.Join(home, ".local", "bin", "xtool"),
		"/usr/local/bin/xtool",
		"/opt/xtool/bin/xtool",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("xtool not found; install from https://github.com/xtool-org/xtool")
}

func findSDK() (string, error) {
	// Check XTOOL_SDK_PATH first
	if sdkPath := os.Getenv("XTOOL_SDK_PATH"); sdkPath != "" {
		if _, err := os.Stat(sdkPath); err == nil {
			return sdkPath, nil
		}
		return "", fmt.Errorf("XTOOL_SDK_PATH set to %q but directory does not exist", sdkPath)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}

	// Check common SDK locations
	candidates := []string{
		filepath.Join(home, ".xtool", "sdk", "iPhoneOS.sdk"),
		filepath.Join(home, ".xtool", "SDKs", "iPhoneOS.sdk"),
		"/opt/xtool/SDKs/iPhoneOS.sdk",
	}

	// Also check for versioned SDKs
	xtoolSDKDir := filepath.Join(home, ".xtool", "sdk")
	if entries, err := os.ReadDir(xtoolSDKDir); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "iPhoneOS") && strings.HasSuffix(entry.Name(), ".sdk") {
				candidates = append([]string{filepath.Join(xtoolSDKDir, entry.Name())}, candidates...)
			}
		}
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("iOS SDK not found; run 'xtool setup' with Xcode.xip or set XTOOL_SDK_PATH")
}

func findClang(sdkPath string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}

	// Priority 1: Swift toolchain's clang (has Objective-C support)
	// This is required for compiling Objective-C++ code (.mm files)
	swiftClangPaths := []string{
		"/opt/swift/usr/bin/clang",
		filepath.Join(home, ".swiftly", "toolchains", "swift-latest", "usr", "bin", "clang"),
		"/usr/share/swift/usr/bin/clang",
	}

	// Check for Swift toolchain paths with glob pattern for versioned installs
	swiftlyToolchains := filepath.Join(home, ".swiftly", "toolchains")
	if entries, err := os.ReadDir(swiftlyToolchains); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), "swift-") {
				swiftClangPaths = append(swiftClangPaths,
					filepath.Join(swiftlyToolchains, entry.Name(), "usr", "bin", "clang"))
			}
		}
	}

	for _, path := range swiftClangPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Priority 2: xtool's bundled clang
	xtoolPaths := []string{
		filepath.Join(home, ".xtool", "toolchain", "bin", "clang"),
		filepath.Join(home, ".xtool", "usr", "bin", "clang"),
		"/opt/xtool/toolchain/bin/clang",
	}

	for _, path := range xtoolPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Priority 3: System clang (works for most cases, Objective-C support varies)
	clangNames := []string{"clang", "clang-18", "clang-17", "clang-16", "clang-15", "clang-14"}
	for _, name := range clangNames {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("clang not found; install Swift toolchain from https://swift.org/download/ (includes clang with Objective-C support)")
}
