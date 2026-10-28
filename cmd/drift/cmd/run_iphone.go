package cmd

import (
	"bytes"
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
	simulator string // name or UDID; empty picks one (resolveIOSSimulator)
	teamID    string
	noLogs    bool
	watch     bool
}

// parseIOSRunArgs parses iOS-specific flags from the argument list and returns
// the resolved options.
func parseIOSRunArgs(args []string) iosRunOptions {
	var opts iosRunOptions
	id, present := parseDeviceFlag(args)
	if present {
		opts.device = true
		opts.deviceID = id
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
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
	sims, err := loadIOSSimulators()
	if err != nil {
		return err
	}
	sim, err := resolveIOSSimulator(sims, opts.simulator)
	if err != nil {
		return err
	}

	buildOpts := iosBuildOptions{buildOptions: buildOptions{noFetch: noFetch}, release: false, device: false}
	if err := buildIOS(ws, buildOpts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Running on iOS Simulator...")

	if err := bootSimulator(sim); err != nil {
		return err
	}

	showSimulator()

	if _, err := os.Stat(filepath.Join(ws.IOSDir, "Runner.xcodeproj")); os.IsNotExist(err) {
		return fmt.Errorf("xcode project not found in workspace - create one in %s", ws.IOSDir)
	}

	if err := xcodebuildForSimulator(ws, sim.UDID); err != nil {
		return err
	}

	if err := installIOSSimulatorApp(ws, sim.UDID); err != nil {
		return err
	}

	ctx, cancel := signalContext()
	defer cancel()

	// Start log streaming before launch so startup logs are captured.
	if !opts.noLogs {
		go streamIOSSimulatorLogs(ctx, sim.UDID, cfg.AppID)
	}

	if err := launchIOSSimulatorApp(cfg.AppID, sim.UDID); err != nil {
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
			exec.Command("xcrun", "simctl", "terminate", sim.UDID, cfg.AppID).Run()
			if err := ws.Refresh(); err != nil {
				return err
			}
			if err := compileGoForIOS(compileCfg); err != nil {
				return err
			}
			if err := xcodebuildForSimulator(ws, sim.UDID); err != nil {
				return err
			}
			if err := installIOSSimulatorApp(ws, sim.UDID); err != nil {
				return err
			}
			return launchIOSSimulatorApp(cfg.AppID, sim.UDID)
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

	// Resolve the device identifier once for the session.
	resolved, err := resolveDevice(opts.deviceID)
	if err != nil {
		return err
	}
	opts.deviceID = resolved.Properties.SerialNumber

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

	ctx, cancel := signalContext()
	defer cancel()

	// Start log streaming before launch so startup logs are captured.
	if !opts.noLogs {
		go streamDeviceLogs(ctx, "Runner", resolved)
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
			exec.Command("xcrun", "devicectl", "device", "process", "terminate", "--device", opts.deviceID, cfg.AppID).Run()
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

// bootSimulator boots the iOS Simulator. If the simulator is already
// booted (exit code 149), the error is silently ignored.
func bootSimulator(sim iosSimulator) error {
	fmt.Printf("  Booting %s...\n", sim)
	cmd := exec.Command("xcrun", "simctl", "boot", sim.UDID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Exit code 149 means simulator is already booted
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 149 {
			// Already booted, continue
		} else {
			return fmt.Errorf("failed to boot simulator %s: %w\n%s", sim, err, strings.TrimSpace(stderr.String()))
		}
	}
	return nil
}

// simulatorViewerBundleIDs lists the apps that display booted simulators,
// newest first: Xcode 27 replaced Simulator.app with Device Hub.
var simulatorViewerBundleIDs = []string{
	"com.apple.dt.Devices",      // Device Hub (Xcode 27+)
	"com.apple.iphonesimulator", // Simulator.app (Xcode 26 and earlier)
}

// showSimulator brings the simulator window to the front. The window is a
// convenience: simctl installs and launches without it, so failing to open
// it is a warning, not an error.
func showSimulator() {
	for _, id := range simulatorViewerBundleIDs {
		if exec.Command("open", "-b", id).Run() == nil {
			return
		}
	}
	fmt.Println("  Warning: could not open Device Hub or Simulator; continuing without a simulator window")
}

// xcodebuildForSimulator runs xcodebuild targeting the iOS Simulator with
// the given UDID.
func xcodebuildForSimulator(ws *workspace.Workspace, udid string) error {
	xcodeproj := filepath.Join(ws.IOSDir, "Runner.xcodeproj")
	buildArgs := []string{
		"-project", xcodeproj,
		"-scheme", "Runner",
		"-configuration", "Debug",
		"-destination", "platform=iOS Simulator,id=" + udid,
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

// installIOSSimulatorApp installs the built .app bundle into the iOS
// Simulator with the given UDID using simctl.
func installIOSSimulatorApp(ws *workspace.Workspace, udid string) error {
	appPath := filepath.Join(ws.BuildDir, "DerivedData", "Build", "Products", "Debug-iphonesimulator", "Runner.app")
	cmd := exec.Command("xcrun", "simctl", "install", udid, appPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install app: %w", err)
	}
	return nil
}

// launchIOSSimulatorApp launches the app by bundle ID in the iOS Simulator
// with the given UDID using simctl.
func launchIOSSimulatorApp(appID, udid string) error {
	cmd := exec.Command("xcrun", "simctl", "launch", udid, appID)
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
