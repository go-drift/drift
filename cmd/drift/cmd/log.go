package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/config"
)

func init() {
	RegisterCommand(&Command{
		Name:  "log",
		Short: "Show application logs",
		Long: `Stream logs from the running application.

For Android, this uses adb logcat filtered to show Drift and Go logs.
For iOS simulator, this shows Console logs from the booted simulator.
For iOS device, this streams syslog from a connected device.
For xtool, this streams syslog from a connected device.

Usage:
  drift log android              # Stream Android logs
  drift log ios                  # Stream iOS simulator logs
  drift log ios --device         # Stream iOS device logs
  drift log ios --device <UDID>  # Stream logs from a specific device
  drift log xtool                # Stream xtool device logs
  drift log xtool --device <UDID>`,
		Usage: "drift log <platform>",
		Run:   runLog,
	})
}

func runLog(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("platform is required (android, ios, or xtool)\n\nUsage: drift log <platform>")
	}

	platform := strings.ToLower(args[0])

	// Load config to get app ID
	root, err := config.FindProjectRoot()
	if err != nil {
		return fmt.Errorf("not in a Drift project (no go.mod found)")
	}
	cfg, err := config.Resolve(root)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	switch platform {
	case "android":
		return logAndroid()
	case "ios":
		return logIOS(cfg, args[1:])
	case "xtool":
		return logXtool(cfg, args[1:])
	default:
		return fmt.Errorf("unknown platform %q (use android, ios, or xtool)", platform)
	}
}

// logAndroid streams logs from Android device.
func logAndroid() error {
	fmt.Println("Streaming Android logs (Ctrl+C to stop)...")
	fmt.Println()
	ctx, cancel := watchContext()
	defer cancel()
	streamAndroidLogs(ctx)
	return nil
}

// logIOS streams logs from iOS simulator or physical device.
func logIOS(cfg *config.Resolved, args []string) error {
	var deviceID string
	device := false
	for i := 0; i < len(args); i++ {
		if args[i] == "--device" {
			device = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				deviceID = args[i+1]
				i++
			}
		}
	}

	ctx, cancel := watchContext()
	defer cancel()

	if device {
		fmt.Println("Streaming iOS device logs (Ctrl+C to stop)...")
		fmt.Println()
		streamDeviceLogs(ctx, "Runner", deviceID)
	} else {
		fmt.Println("Streaming iOS simulator logs (Ctrl+C to stop)...")
		fmt.Println()
		streamIOSSimulatorLogs(ctx, "booted", cfg.AppID)
	}
	return nil
}

// logXtool streams logs from a device built with xtool.
func logXtool(cfg *config.Resolved, args []string) error {
	var deviceID string
	for i := 0; i < len(args); i++ {
		if args[i] == "--device" && i+1 < len(args) {
			deviceID = args[i+1]
			i++
		}
	}
	fmt.Println("Streaming device logs (Ctrl+C to stop)...")
	fmt.Println()
	ctx, cancel := watchContext()
	defer cancel()
	streamDeviceLogs(ctx, cfg.AppName, deviceID)
	return nil
}

// streamIOSSimulatorLogs streams logs from a simulator app, filtered by
// process name and subsystem (bundle ID). simulator should be a simulator
// name or "booted" for the active simulator.
func streamIOSSimulatorLogs(ctx context.Context, simulator, appID string) {
	args := []string{"simctl", "spawn", simulator,
		"log", "stream",
		"--process", "Runner",
		"--level", "debug",
		"--style", "compact",
	}
	if appID != "" && !strings.ContainsAny(appID, `"'\`) {
		args = append(args, "--predicate", fmt.Sprintf(`subsystem == "%s"`, appID))
	}
	cmd := exec.CommandContext(ctx, "xcrun", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}
