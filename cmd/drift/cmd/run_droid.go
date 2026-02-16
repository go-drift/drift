package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/config"
	"github.com/go-drift/drift/cmd/drift/internal/workspace"
)

// runAndroid builds, installs, and runs on Android.
func runAndroid(ws *workspace.Workspace, cfg *config.Resolved, opts runOptions) error {
	adb := findADB()

	buildOpts := androidBuildOptions{buildOptions: buildOptions{noFetch: opts.noFetch}, release: false}
	if opts.watch {
		// Only compile for the connected device's ABI during watch mode
		if abi := detectDeviceABI(adb); abi != "" {
			fmt.Printf("  Detected device ABI: %s\n", abi)
			buildOpts.targetABI = abi
		}
	}

	if err := buildAndroid(ws, buildOpts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Installing on device...")

	if err := installAndroidAPK(adb, ws); err != nil {
		return err
	}

	fmt.Println("Launching application...")

	if err := launchAndroidApp(adb, cfg); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Application running!")
	fmt.Println()

	if opts.watch {
		ctx, cancel := watchContext()
		defer cancel()
		if !opts.noLogs {
			go streamAndroidLogs(ctx, cfg.AppID)
		}
		return watchAndRun(ctx, ws, func() error {
			exec.Command(adb, "shell", "am", "force-stop", cfg.AppID).Run()
			if err := ws.Refresh(); err != nil {
				return err
			}
			if err := buildAndroid(ws, buildOpts); err != nil {
				return err
			}
			if err := installAndroidAPK(adb, ws); err != nil {
				return err
			}
			return launchAndroidApp(adb, cfg)
		})
	}

	if !opts.noLogs {
		return logAndroid(cfg.AppID)
	}
	return nil
}

// detectDeviceABI queries the connected Android device for its primary ABI.
// Returns an empty string if detection fails.
func detectDeviceABI(adb string) string {
	out, err := exec.Command(adb, "shell", "getprop", "ro.product.cpu.abi").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// installAndroidAPK installs the debug APK onto the connected Android device
// using adb install.
func installAndroidAPK(adb string, ws *workspace.Workspace) error {
	apkPath := filepath.Join(ws.AndroidDir, "app", "build", "outputs", "apk", "debug", "app-debug.apk")
	cmd := exec.Command(adb, "install", "-r", apkPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install APK: %w", err)
	}
	return nil
}

// launchAndroidApp starts the app's main activity on the connected Android
// device using adb shell am start.
func launchAndroidApp(adb string, cfg *config.Resolved) error {
	activityName := fmt.Sprintf("%s/.MainActivity", cfg.AppID)
	cmd := exec.Command(adb, "shell", "am", "start", "-n", activityName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to launch application: %w", err)
	}
	return nil
}
