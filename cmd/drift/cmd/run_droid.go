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
func runAndroid(ws *workspace.Workspace, cfg *config.Resolved, args []string, opts runOptions) error {
	adb := findADB()

	deviceID, _ := parseDeviceFlag(args)
	serial, err := resolveAndroidDevice(adb, deviceID)
	if err != nil {
		return err
	}

	buildOpts := androidBuildOptions{buildOptions: buildOptions{noFetch: opts.noFetch}, release: false}
	if opts.watch {
		// Only compile for the connected device's ABI during watch mode
		if abi := detectDeviceABI(adb, serial); abi != "" {
			fmt.Printf("  Detected device ABI: %s\n", abi)
			buildOpts.targetABI = abi
		}
	}

	if err := buildAndroid(ws, buildOpts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Installing on device...")

	if err := installAndroidAPK(adb, serial, ws); err != nil {
		return err
	}

	fmt.Println("Launching application...")

	if err := launchAndroidApp(adb, serial, cfg); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Application running!")
	fmt.Println()

	if opts.watch {
		ctx, cancel := signalContext()
		defer cancel()
		if !opts.noLogs {
			go streamAndroidLogs(ctx, serial)
		}
		return watchAndRun(ctx, ws, func() error {
			adbCommand(adb, serial, "shell", "am", "force-stop", cfg.AppID).Run()
			if err := ws.Refresh(); err != nil {
				return err
			}
			if err := buildAndroid(ws, buildOpts); err != nil {
				return err
			}
			if err := installAndroidAPK(adb, serial, ws); err != nil {
				return err
			}
			return launchAndroidApp(adb, serial, cfg)
		})
	}

	if !opts.noLogs {
		ctx, cancel := signalContext()
		defer cancel()
		streamAndroidLogs(ctx, serial)
	}
	return nil
}

// adbCommand builds an exec.Cmd for adb, prepending `-s serial` when serial
// is non-empty.
func adbCommand(adb, serial string, args ...string) *exec.Cmd {
	if serial != "" {
		args = append([]string{"-s", serial}, args...)
	}
	return exec.Command(adb, args...)
}

// detectDeviceABI queries the connected Android device for its primary ABI.
// Returns an empty string if detection fails.
func detectDeviceABI(adb, serial string) string {
	out, err := adbCommand(adb, serial, "shell", "getprop", "ro.product.cpu.abi").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// installAndroidAPK installs the debug APK onto the connected Android device
// using adb install.
func installAndroidAPK(adb, serial string, ws *workspace.Workspace) error {
	apkPath := filepath.Join(ws.AndroidDir, "app", "build", "outputs", "apk", "debug", "app-debug.apk")
	cmd := adbCommand(adb, serial, "install", "-r", apkPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install APK: %w", err)
	}
	return nil
}

// launchAndroidApp starts the app's main activity on the connected Android
// device using adb shell am start.
func launchAndroidApp(adb, serial string, cfg *config.Resolved) error {
	activityName := fmt.Sprintf("%s/.MainActivity", cfg.AppID)
	cmd := adbCommand(adb, serial, "shell", "am", "start", "-n", activityName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to launch application: %w", err)
	}
	return nil
}
