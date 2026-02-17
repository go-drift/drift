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

type iosRunOptions struct {
	device    bool
	deviceID  string
	simulator string
	teamID    string
	noLogs    bool
	watch     bool
}

// parseIOSRunArgs parses iOS-specific flags from the argument list and returns
// the resolved options.
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

// runIOS builds and runs on iOS simulator or physical device.
func runIOS(ws *workspace.Workspace, cfg *config.Resolved, args []string, opts runOptions) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("iOS development requires macOS")
	}

	iosOpts := parseIOSRunArgs(args)
	if opts.noLogs {
		iosOpts.noLogs = true
	}
	iosOpts.watch = opts.watch

	if iosOpts.device {
		return runIOSDevice(ws, cfg, iosOpts, opts.noFetch)
	}
	return runIOSSimulator(ws, cfg, iosOpts, opts.noFetch)
}

// runIOSSimulator builds and runs on iOS simulator.
func runIOSSimulator(ws *workspace.Workspace, cfg *config.Resolved, opts iosRunOptions, noFetch bool) error {
	buildOpts := iosBuildOptions{buildOptions: buildOptions{noFetch: noFetch}, release: false, device: false}
	if err := buildIOS(ws, buildOpts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Running on iOS Simulator...")

	if err := bootSimulator(opts.simulator); err != nil {
		return err
	}

	if err := exec.Command("open", "-a", "Simulator").Run(); err != nil {
		return fmt.Errorf("failed to open Simulator app: %w", err)
	}

	if _, err := os.Stat(filepath.Join(ws.IOSDir, "Runner.xcodeproj")); os.IsNotExist(err) {
		return fmt.Errorf("xcode project not found in workspace - create one in %s", ws.IOSDir)
	}

	if err := xcodebuildForSimulator(ws, opts.simulator); err != nil {
		return err
	}

	if err := installIOSSimulatorApp(ws, opts.simulator); err != nil {
		return err
	}

	ctx, cancel := watchContext()
	defer cancel()

	// Start log streaming before launch so startup logs are captured.
	if !opts.noLogs {
		go streamIOSSimulatorLogs(ctx, opts.simulator, cfg.AppID)
	}

	if err := launchIOSSimulatorApp(cfg.AppID, opts.simulator); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Application running!")
	fmt.Println()

	if opts.watch {
		compileCfg := iosCompileConfig{
			projectRoot: ws.Root,
			overlayPath: ws.Overlay,
			libDir:      filepath.Join(ws.IOSDir, "Runner"),
			device:      false,
			arch:        runtime.GOARCH,
			noFetch:     noFetch,
		}
		return watchAndRun(ctx, ws, func() error {
			if err := ws.Refresh(); err != nil {
				return err
			}
			if err := compileGoForIOS(compileCfg); err != nil {
				return err
			}
			if err := xcodebuildForSimulator(ws, opts.simulator); err != nil {
				return err
			}
			if err := installIOSSimulatorApp(ws, opts.simulator); err != nil {
				return err
			}
			return launchIOSSimulatorApp(cfg.AppID, opts.simulator)
		})
	}

	if !opts.noLogs {
		// Block until Ctrl+C; log streaming goroutine handles output.
		<-ctx.Done()
	}
	return nil
}

// runIOSDevice builds and runs on a physical iOS device.
func runIOSDevice(ws *workspace.Workspace, cfg *config.Resolved, opts iosRunOptions, noFetch bool) error {
	if _, err := exec.LookPath("xcrun"); err != nil {
		return fmt.Errorf("xcrun not found; make sure Xcode command line tools are installed")
	}

	// Resolve the device identifier once for the session. devicectl needs
	// the device name; go-ios (log streaming) needs the xctrace UDID.
	resolvedDevice, err := resolveIOSDevice(opts.deviceID)
	if err != nil {
		return err
	}
	opts.deviceID = resolvedDevice.name

	buildOpts := iosBuildOptions{buildOptions: buildOptions{noFetch: noFetch}, release: false, device: true, teamID: opts.teamID}
	if err := buildIOS(ws, buildOpts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Running on iOS Device...")

	if _, err := os.Stat(filepath.Join(ws.IOSDir, "Runner.xcodeproj")); os.IsNotExist(err) {
		return fmt.Errorf("xcode project not found in workspace - create one in %s", ws.IOSDir)
	}

	if err := xcodebuildForDevice(ws, opts); err != nil {
		return err
	}

	fmt.Println("  Installing on device...")
	if err := devicectlInstall(ws, opts); err != nil {
		return err
	}

	ctx, cancel := watchContext()
	defer cancel()

	// Start log streaming before launch so startup logs are captured.
	if !opts.noLogs {
		if resolvedDevice.udid != "" {
			go streamDeviceLogs(ctx, "Runner", resolvedDevice.udid)
		} else {
			fmt.Fprintln(os.Stderr, "Note: log streaming unavailable (device UDID not resolved)")
		}
	}

	fmt.Println("  Launching on device...")
	if err := devicectlLaunch(cfg.AppID, opts.deviceID); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Application running!")
	fmt.Println()

	if opts.watch {
		compileCfg := iosCompileConfig{
			projectRoot: ws.Root,
			overlayPath: ws.Overlay,
			libDir:      filepath.Join(ws.IOSDir, "Runner"),
			device:      true,
			arch:        "arm64",
			noFetch:     noFetch,
		}
		return watchAndRun(ctx, ws, func() error {
			if err := ws.Refresh(); err != nil {
				return err
			}
			if err := compileGoForIOS(compileCfg); err != nil {
				return err
			}
			if err := xcodebuildForDevice(ws, opts); err != nil {
				return err
			}
			if err := devicectlInstall(ws, opts); err != nil {
				return err
			}
			return devicectlLaunch(cfg.AppID, opts.deviceID)
		})
	}

	if !opts.noLogs {
		// Block until Ctrl+C; log streaming goroutine handles output.
		<-ctx.Done()
	}
	return nil
}

// --------------------------------------------------------------------
// iOS helper functions
// --------------------------------------------------------------------

// bootSimulator boots the named iOS Simulator. If the simulator is already
// booted (exit code 149), the error is silently ignored.
func bootSimulator(name string) error {
	fmt.Printf("  Booting %s...\n", name)
	cmd := exec.Command("xcrun", "simctl", "boot", name)
	if err := cmd.Run(); err != nil {
		// Exit code 149 means simulator is already booted
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 149 {
			// Already booted, continue
		} else {
			return fmt.Errorf("failed to boot simulator %s: %w", name, err)
		}
	}
	return nil
}

// xcodebuildForSimulator runs xcodebuild targeting the named iOS Simulator.
func xcodebuildForSimulator(ws *workspace.Workspace, simulator string) error {
	xcodeproj := filepath.Join(ws.IOSDir, "Runner.xcodeproj")
	buildArgs := []string{
		"-project", xcodeproj,
		"-scheme", "Runner",
		"-configuration", "Debug",
		"-destination", fmt.Sprintf("platform=iOS Simulator,name=%s", simulator),
		"-derivedDataPath", filepath.Join(ws.BuildDir, "DerivedData"),
	}
	buildArgs = append(buildArgs, simulatorArchBuildSettings()...)
	buildArgs = append(buildArgs, "build")
	cmd := exec.Command("xcodebuild", buildArgs...)
	cmd.Dir = ws.IOSDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("xcodebuild failed: %w", err)
	}
	return nil
}

// installIOSSimulatorApp installs the built .app bundle into the named iOS
// Simulator using simctl.
func installIOSSimulatorApp(ws *workspace.Workspace, simulator string) error {
	appPath := filepath.Join(ws.BuildDir, "DerivedData", "Build", "Products", "Debug-iphonesimulator", "Runner.app")
	cmd := exec.Command("xcrun", "simctl", "install", simulator, appPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install app: %w", err)
	}
	return nil
}

// launchIOSSimulatorApp launches the app by bundle ID in the named iOS
// Simulator using simctl.
func launchIOSSimulatorApp(appID, simulator string) error {
	cmd := exec.Command("xcrun", "simctl", "launch", simulator, appID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to launch app: %w", err)
	}
	return nil
}

// xcodebuildForDevice runs xcodebuild targeting a physical iOS device.
func xcodebuildForDevice(ws *workspace.Workspace, opts iosRunOptions) error {
	xcodeproj := filepath.Join(ws.IOSDir, "Runner.xcodeproj")
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
	return nil
}

// devicectlInstall installs the built .app bundle on a physical iOS device
// using xcrun devicectl (requires Xcode 15+).
func devicectlInstall(ws *workspace.Workspace, opts iosRunOptions) error {
	appPath := filepath.Join(ws.BuildDir, "DerivedData", "Build", "Products", "Debug-iphoneos", "Runner.app")
	args := []string{"devicectl", "device", "install", "app", "--device", opts.deviceID, appPath}
	cmd := exec.Command("xcrun", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("devicectl install failed: %w\nMake sure Xcode 15+ is installed and the device is connected", err)
	}
	return nil
}

// resolveIOSDevice resolves a device identifier (name, UDID, or empty for
// auto-detect) into an iosDevice with both name and UDID populated.
// devicectl needs the name; go-ios needs the UDID.
func resolveIOSDevice(id string) (iosDevice, error) {
	devices, err := connectedIOSDevices()
	if err != nil {
		return iosDevice{}, fmt.Errorf("failed to list devices: %w", err)
	}

	if id == "" {
		switch len(devices) {
		case 0:
			return iosDevice{}, fmt.Errorf("no connected iOS devices found\nConnect a device via USB, or specify a device with --device <name-or-udid>")
		case 1:
			fmt.Printf("  Auto-detected device: %s (%s)\n", devices[0].name, devices[0].udid)
			return devices[0], nil
		default:
			var lines []string
			for _, d := range devices {
				lines = append(lines, fmt.Sprintf("  %s (%s)", d.name, d.udid))
			}
			return iosDevice{}, fmt.Errorf("multiple iOS devices connected, specify one with --device <name-or-udid>:\n%s", strings.Join(lines, "\n"))
		}
	}

	// Match by name (case-insensitive) or UDID (case-sensitive).
	for _, d := range devices {
		if strings.EqualFold(d.name, id) || d.udid == id {
			return d, nil
		}
	}

	// Not found in xctrace listing.
	if looksLikeUDID(id) {
		var lines []string
		for _, d := range devices {
			lines = append(lines, fmt.Sprintf("  %s (%s)", d.name, d.udid))
		}
		listing := "(none)"
		if len(lines) > 0 {
			listing = strings.Join(lines, "\n")
		}
		return iosDevice{}, fmt.Errorf("device with UDID %s not found\nConnected devices:\n%s", id, listing)
	}

	// Looks like a name; let devicectl attempt its own resolution.
	// Log streaming will be unavailable since we don't have a UDID.
	return iosDevice{name: id, udid: ""}, nil
}

// looksLikeUDID returns true if s resembles an iOS device UDID (hex digits
// and dashes, at least 20 characters, no spaces).
func looksLikeUDID(s string) bool {
	if strings.ContainsRune(s, ' ') {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') || r == '-') {
			return false
		}
	}
	return len(s) >= 20
}

// devicectlLaunch launches the app by bundle ID on a physical iOS device
// using xcrun devicectl (requires Xcode 15+).
func devicectlLaunch(appID, deviceID string) error {
	cmd := exec.Command("xcrun", "devicectl", "device", "process", "launch", "--device", deviceID, appID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("devicectl launch failed: %w\nMake sure Xcode 15+ is installed and the device is connected", err)
	}
	return nil
}
