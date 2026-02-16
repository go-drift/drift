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

	if err := launchIOSSimulatorApp(cfg.AppID, opts.simulator); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Application running!")
	fmt.Println()

	if opts.watch {
		ctx, cancel := watchContext()
		defer cancel()
		if !opts.noLogs {
			go streamIOSLogs(ctx, cfg.AppID)
		}
		compileCfg := iosCompileConfig{
			projectRoot: ws.Root,
			overlayPath: ws.Overlay,
			libDir:      filepath.Join(ws.IOSDir, "Runner"),
			device:      false,
			arch:        runtime.GOARCH,
			noFetch:     noFetch,
		}
		return watchAndRun(ctx, ws, func() error {
			exec.Command("xcrun", "simctl", "terminate", opts.simulator, cfg.AppID).Run()
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
		return logIOS(cfg.AppID)
	}
	return nil
}

// runIOSDevice builds and runs on a physical iOS device.
func runIOSDevice(ws *workspace.Workspace, cfg *config.Resolved, opts iosRunOptions, noFetch bool) error {
	if _, err := exec.LookPath("ios-deploy"); err != nil {
		return fmt.Errorf("ios-deploy not found; install with: brew install ios-deploy")
	}

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

	fmt.Println("  Installing and launching on device...")
	// In watch mode, use --justlaunch so ios-deploy exits after launching
	if err := iosDeployApp(ws, opts, opts.watch); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Application running!")
	fmt.Println()

	if opts.watch {
		ctx, cancel := watchContext()
		defer cancel()
		if !opts.noLogs {
			go streamIOSLogs(ctx, cfg.AppID)
		}
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
			return iosDeployApp(ws, opts, true)
		})
	}

	return nil
}

// --------------------------------------------------------------------
// iOS helper functions
// --------------------------------------------------------------------

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

func launchIOSSimulatorApp(appID, simulator string) error {
	cmd := exec.Command("xcrun", "simctl", "launch", simulator, appID)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to launch app: %w", err)
	}
	return nil
}

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

// iosDeployApp installs and launches the app on a physical device using
// ios-deploy. When justLaunch is true, ios-deploy exits after launching
// (used in watch mode so the rebuild loop can continue).
func iosDeployApp(ws *workspace.Workspace, opts iosRunOptions, justLaunch bool) error {
	appPath := filepath.Join(ws.BuildDir, "DerivedData", "Build", "Products", "Debug-iphoneos", "Runner.app")
	var deployArgs []string
	if justLaunch {
		deployArgs = []string{"--bundle", appPath, "--justlaunch", "--noninteractive"}
	} else {
		deployArgs = []string{"--bundle", appPath, "--debug", "--noninteractive"}
	}
	if opts.deviceID != "" {
		deployArgs = append([]string{"--id", opts.deviceID}, deployArgs...)
	}

	cmd := exec.Command("ios-deploy", deployArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if !justLaunch {
		cmd.Stdin = os.Stdin
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ios-deploy failed: %w", err)
	}
	return nil
}
