package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/cache"
)

// iosCompileConfig holds parameters for iOS Go cross-compilation.
type iosCompileConfig struct {
	projectRoot string
	overlayPath string
	libDir      string // output directory for libdrift.a and libdrift_skia.a
	device      bool
	arch        string // "arm64" or "amd64"
	noFetch     bool
}

// compileGoForIOS compiles Go code to a static library for iOS and copies the Skia library.
func compileGoForIOS(cfg iosCompileConfig) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("iOS compilation requires macOS")
	}

	sdk := "iphonesimulator"
	if cfg.device {
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

	if err := os.MkdirAll(cfg.libDir, 0o755); err != nil {
		return fmt.Errorf("failed to create library directory: %w", err)
	}

	skiaPlatform := "ios-simulator"
	if cfg.device {
		skiaPlatform = "ios"
	}
	skiaLib, skiaDir, err := findSkiaLib(cfg.projectRoot, skiaPlatform, cfg.arch, cfg.noFetch)
	if err != nil {
		return err
	}

	// Copy Skia library if version has changed
	skiaVersion := cache.DriftSkiaVersion()
	skiaVersionFile := filepath.Join(cfg.libDir, ".drift-skia-version")
	if needsSkiaCopy(skiaVersionFile, skiaVersion) {
		fmt.Println("  Copying Skia library...")
		if err := copyFile(skiaLib, filepath.Join(cfg.libDir, "libdrift_skia.a")); err != nil {
			return fmt.Errorf("failed to copy Skia library: %w", err)
		}
		if err := os.WriteFile(skiaVersionFile, []byte(skiaVersion), 0o644); err != nil {
			return fmt.Errorf("failed to write Skia version marker: %w", err)
		}
	}

	libPath := filepath.Join(cfg.libDir, "libdrift.a")

	iosArch := "x86_64"
	if cfg.arch == "arm64" {
		iosArch = "arm64"
	}

	versionMinFlag := "-mios-simulator-version-min=16.0"
	if cfg.device {
		versionMinFlag = "-miphoneos-version-min=16.0"
	}
	cgoCflags := fmt.Sprintf("-isysroot %s -arch %s %s", sdkRoot, iosArch, versionMinFlag)
	cgoCxxflags := fmt.Sprintf("-isysroot %s -arch %s %s -std=c++17 -x objective-c++", sdkRoot, iosArch, versionMinFlag)

	cmd := exec.Command("go", "build",
		"-overlay", cfg.overlayPath,
		"-buildmode=c-archive",
		"-o", libPath,
		".")
	cmd.Dir = cfg.projectRoot
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=1",
		"GOOS=ios",
		"GOARCH="+cfg.arch,
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

	return nil
}

// androidCompileConfig holds parameters for Android Go cross-compilation.
type androidCompileConfig struct {
	projectRoot string
	overlayPath string
	jniLibsDir  string
	noFetch     bool
	targetABI   string // when set, compile only this ABI (e.g. "arm64-v8a")
}

// findADB locates the adb binary, checking ANDROID_SDK_ROOT and
// ANDROID_HOME before falling back to bare "adb" on PATH.
func findADB() string {
	if sdkRoot := os.Getenv("ANDROID_SDK_ROOT"); sdkRoot != "" {
		return filepath.Join(sdkRoot, "platform-tools", "adb")
	}
	if androidHome := os.Getenv("ANDROID_HOME"); androidHome != "" {
		return filepath.Join(androidHome, "platform-tools", "adb")
	}
	return "adb"
}

// compileGoForAndroid compiles Go code to shared libraries for all Android ABIs.
func compileGoForAndroid(cfg androidCompileConfig) error {
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
		{"arm64-v8a", "arm64", "", "aarch64-linux-android31-clang", "aarch64-linux-android", "arm64"},
		{"armeabi-v7a", "arm", "7", "armv7a-linux-androideabi31-clang", "arm-linux-androideabi", "arm"},
		{"x86_64", "amd64", "", "x86_64-linux-android31-clang", "x86_64-linux-android", "amd64"},
	}

	if cfg.targetABI != "" {
		found := false
		for _, abi := range abis {
			if abi.abi == cfg.targetABI {
				found = true
				break
			}
		}
		if !found {
			supported := make([]string, len(abis))
			for i, abi := range abis {
				supported[i] = abi.abi
			}
			return fmt.Errorf("unsupported Android ABI %q (supported: %s)", cfg.targetABI, strings.Join(supported, ", "))
		}
	}

	for _, abi := range abis {
		if cfg.targetABI != "" && abi.abi != cfg.targetABI {
			continue
		}
		fmt.Printf("  Compiling for %s...\n", abi.abi)

		_, skiaDir, err := findSkiaLib(cfg.projectRoot, "android", abi.skiaArch, cfg.noFetch)
		if err != nil {
			return err
		}

		outDir := filepath.Join(cfg.jniLibsDir, abi.abi)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}

		cmd := exec.Command("go", "build",
			"-overlay", cfg.overlayPath,
			"-buildmode=c-shared",
			"-o", filepath.Join(outDir, "libdrift.so"),
			".")
		cmd.Dir = cfg.projectRoot
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
			if _, err := os.Stat(cppShared); err == nil {
				fmt.Println("  Warning: using libc++_shared.so from local NDK (may cause ABI issues with older releases)")
			}
		}
		if _, err := os.Stat(cppShared); err == nil {
			if err := copyFile(cppShared, filepath.Join(outDir, "libc++_shared.so")); err != nil {
				return fmt.Errorf("failed to copy libc++_shared.so: %w", err)
			}
		}

		os.Remove(filepath.Join(outDir, "libdrift.h"))
	}

	return nil
}
