package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/config"
	"github.com/go-drift/drift/cmd/drift/internal/workspace"
)

func init() {
	RegisterCommand(&Command{
		Name:  "run",
		Short: "Build and run on a device or simulator",
		Long: `Build the application and run it on a connected device or simulator.

Supported platforms:
  android   Run on Android device or emulator
  ios       Run on iOS device or simulator (requires macOS)
  xtool     Run on iOS device using xtool (Linux/macOS, no Xcode required)

The command will:
  1. Build the application (debug mode)
  2. Install it on the target device
  3. Launch the application

Flags:
  --no-logs          Launch without streaming logs
  --device [UDID]    Run on a physical iOS device (optional: specify UDID)
  --simulator NAME   Run on a specific iOS simulator (default: iPhone 15)
  --team-id TEAM_ID  Apple Developer Team ID for code signing (required for --device)

For Android, you can specify a device with ADB:
  ANDROID_SERIAL=<device-id> drift run android

For iOS simulators:
  drift run ios --simulator "iPhone 15"

For physical iOS devices:
  drift run ios --device --team-id ABC123XYZ
  drift run ios --device 00008030-... --team-id ABC123XYZ

For xtool (Linux/macOS):
  drift run xtool                     Run on connected device
  drift run xtool --device UDID       Run on specific device

Note: Physical device deployment requires ios-deploy (brew install ios-deploy)
      or ideviceinstaller (part of libimobiledevice)`,
		Usage: "drift run <platform> [--no-logs] [--device [UDID]] [--simulator NAME] [--team-id TEAM_ID]",
		Run:   runRun,
	})
}

func runRun(args []string) error {
	platformArgs, noLogs := parseRunArgs(args)
	if len(platformArgs) == 0 {
		return fmt.Errorf("platform is required (android, ios, or xtool)\n\nUsage: drift run <platform> [--no-logs]")
	}

	platform := strings.ToLower(platformArgs[0])

	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Resolve(root)
	if err != nil {
		return err
	}

	ws, err := workspace.Prepare(root, cfg, platform)
	if err != nil {
		return err
	}

	switch platform {
	case "android":
		return runAndroid(ws, cfg, platformArgs[1:], noLogs)
	case "ios":
		return runIOS(ws, cfg, platformArgs[1:], noLogs)
	case "xtool":
		return runXtool(ws, cfg, platformArgs[1:], noLogs)
	default:
		return fmt.Errorf("unknown platform %q (use android, ios, or xtool)", platform)
	}
}

type iosRunOptions struct {
	device    bool
	deviceID  string
	simulator string
	teamID    string
	noLogs    bool
}

type xtoolRunOptions struct {
	deviceID string
	noLogs   bool
}

func parseRunArgs(args []string) ([]string, bool) {
	noLogs := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--no-logs" {
			noLogs = true
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered, noLogs
}

func parseIOSRunArgs(args []string) iosRunOptions {
	opts := iosRunOptions{
		simulator: "iPhone 15",
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--no-logs":
			opts.noLogs = true
		case "--device":
			opts.device = true
			// Check if next arg is a UDID (not another flag)
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				opts.deviceID = args[i+1]
				i++
			}
		case "--simulator":
			if i+1 < len(args) {
				opts.simulator = args[i+1]
				i++
			}
		case "--team-id":
			if i+1 < len(args) {
				opts.teamID = args[i+1]
				i++
			}
		}
	}
	return opts
}

// runAndroid builds, installs, and runs on Android.
func runAndroid(ws *workspace.Workspace, cfg *config.Resolved, args []string, noLogs bool) error {
	if err := buildAndroid(ws, false); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Installing on device...")

	adb := "adb"
	if sdkRoot := os.Getenv("ANDROID_SDK_ROOT"); sdkRoot != "" {
		adb = filepath.Join(sdkRoot, "platform-tools", "adb")
	} else if androidHome := os.Getenv("ANDROID_HOME"); androidHome != "" {
		adb = filepath.Join(androidHome, "platform-tools", "adb")
	}

	apkPath := filepath.Join(ws.AndroidDir, "app", "build", "outputs", "apk", "debug", "app-debug.apk")

	cmd := exec.Command(adb, "install", "-r", apkPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install APK: %w", err)
	}

	fmt.Println("Launching application...")

	activityName := fmt.Sprintf("%s/.MainActivity", cfg.AppID)
	cmd = exec.Command(adb, "shell", "am", "start", "-n", activityName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to launch application: %w", err)
	}

	fmt.Println()
	fmt.Println("Application running!")
	fmt.Println()

	if !noLogs {
		if err := logAndroid(); err != nil {
			return err
		}
	}

	return nil
}

// runIOS builds and runs on iOS simulator or physical device.
func runIOS(ws *workspace.Workspace, cfg *config.Resolved, args []string, noLogs bool) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("iOS development requires macOS")
	}

	opts := parseIOSRunArgs(args)
	if noLogs {
		opts.noLogs = true
	}

	if opts.device {
		return runIOSDevice(ws, cfg, opts)
	}
	return runIOSSimulator(ws, cfg, opts)
}

// runIOSSimulator builds and runs on iOS simulator.
func runIOSSimulator(ws *workspace.Workspace, cfg *config.Resolved, opts iosRunOptions) error {
	buildOpts := iosBuildOptions{release: false, device: false}
	if err := buildIOS(ws, buildOpts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Running on iOS Simulator...")

	fmt.Printf("  Booting %s...\n", opts.simulator)
	cmd := exec.Command("xcrun", "simctl", "boot", opts.simulator)
	if err := cmd.Run(); err != nil {
		// Exit code 149 means simulator is already booted - that's OK
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 149 {
			// Already booted, continue
		} else {
			return fmt.Errorf("failed to boot simulator %s: %w", opts.simulator, err)
		}
	}

	cmd = exec.Command("open", "-a", "Simulator")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open Simulator app: %w", err)
	}

	xcodeproj := filepath.Join(ws.IOSDir, "Runner.xcodeproj")
	if _, err := os.Stat(xcodeproj); os.IsNotExist(err) {
		return fmt.Errorf("xcode project not found in workspace - create one in %s", ws.IOSDir)
	}

	cmd = exec.Command("xcodebuild",
		"-project", xcodeproj,
		"-scheme", "Runner",
		"-configuration", "Debug",
		"-destination", fmt.Sprintf("platform=iOS Simulator,name=%s", opts.simulator),
		"-derivedDataPath", filepath.Join(ws.BuildDir, "DerivedData"),
		"build")
	cmd.Dir = ws.IOSDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xcodebuild failed: %w", err)
	}

	appPath := filepath.Join(ws.BuildDir, "DerivedData", "Build", "Products", "Debug-iphonesimulator", "Runner.app")
	cmd = exec.Command("xcrun", "simctl", "install", opts.simulator, appPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install app: %w", err)
	}

	cmd = exec.Command("xcrun", "simctl", "launch", opts.simulator, cfg.AppID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to launch app: %w", err)
	}

	fmt.Println()
	fmt.Println("Application running!")
	fmt.Println()

	if !opts.noLogs {
		if err := logIOS(); err != nil {
			return err
		}
	}

	return nil
}

// runIOSDevice builds and runs on a physical iOS device.
func runIOSDevice(ws *workspace.Workspace, cfg *config.Resolved, opts iosRunOptions) error {
	// Check for ios-deploy
	if _, err := exec.LookPath("ios-deploy"); err != nil {
		return fmt.Errorf("ios-deploy not found; install with: brew install ios-deploy")
	}

	buildOpts := iosBuildOptions{release: false, device: true, teamID: opts.teamID}
	if err := buildIOS(ws, buildOpts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Running on iOS Device...")

	xcodeproj := filepath.Join(ws.IOSDir, "Runner.xcodeproj")
	if _, err := os.Stat(xcodeproj); os.IsNotExist(err) {
		return fmt.Errorf("xcode project not found in workspace - create one in %s", ws.IOSDir)
	}

	// Build with xcodebuild for device
	buildArgs := []string{
		"-project", xcodeproj,
		"-scheme", "Runner",
		"-configuration", "Debug",
		"-destination", "generic/platform=iOS",
		"-derivedDataPath", filepath.Join(ws.BuildDir, "DerivedData"),
		"-allowProvisioningUpdates",
	}
	if opts.teamID != "" {
		buildArgs = append(buildArgs, "DEVELOPMENT_TEAM="+opts.teamID)
	}
	buildArgs = append(buildArgs, "build")

	cmd := exec.Command("xcodebuild", buildArgs...)
	cmd.Dir = ws.IOSDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xcodebuild failed: %w", err)
	}

	appPath := filepath.Join(ws.BuildDir, "DerivedData", "Build", "Products", "Debug-iphoneos", "Runner.app")

	// Install and launch using ios-deploy
	fmt.Println("  Installing and launching on device...")
	deployArgs := []string{"--bundle", appPath, "--debug", "--noninteractive"}
	if opts.deviceID != "" {
		deployArgs = append([]string{"--id", opts.deviceID}, deployArgs...)
	}

	cmd = exec.Command("ios-deploy", deployArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ios-deploy failed: %w", err)
	}

	fmt.Println()
	fmt.Println("Application running!")
	fmt.Println()

	return nil
}

func parseXtoolRunArgs(args []string) xtoolRunOptions {
	opts := xtoolRunOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--no-logs":
			opts.noLogs = true
		case "--device":
			// Check if next arg is a UDID (not another flag)
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				opts.deviceID = args[i+1]
				i++
			}
		}
	}
	return opts
}

// runXtool builds and runs on iOS device using xtool (no Xcode required).
func runXtool(ws *workspace.Workspace, cfg *config.Resolved, args []string, noLogs bool) error {
	opts := parseXtoolRunArgs(args)
	if noLogs {
		opts.noLogs = true
	}

	// Build the app first
	buildOpts := xtoolBuildOptions{release: false, device: true}
	if err := buildXtool(ws, buildOpts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Running on iOS Device (via xtool)...")

	// Use xtool dev run from the generated xtool directory
	// This handles signing and deployment using xtool's Apple Developer integration
	xtoolPath, err := exec.LookPath("xtool")
	if err != nil {
		return fmt.Errorf("xtool not found in PATH")
	}

	runArgs := []string{"dev", "run"}
	if opts.deviceID != "" {
		runArgs = append(runArgs, "--device", opts.deviceID)
	}

	cmd := exec.Command(xtoolPath, runArgs...)
	cmd.Dir = ws.XtoolDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xtool dev run failed: %w", err)
	}

	fmt.Println()
	fmt.Println("Application running!")
	fmt.Println()

	// Stream logs if requested
	if !opts.noLogs {
		idevicesyslog, err := exec.LookPath("idevicesyslog")
		if err != nil {
			fmt.Println("Note: idevicesyslog not found, cannot stream logs")
			fmt.Println("Install libimobiledevice for log streaming support")
			return nil
		}

		fmt.Println("Streaming device logs (Ctrl+C to stop)...")
		fmt.Println()

		logArgs := []string{"--match", cfg.AppID}
		if opts.deviceID != "" {
			logArgs = append([]string{"-u", opts.deviceID}, logArgs...)
		}

		cmd = exec.Command(idevicesyslog, logArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			// User likely hit Ctrl+C, which is fine
			return nil
		}
	}

	return nil
}
