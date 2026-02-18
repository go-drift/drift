package cmd

import (
	"fmt"
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
  drift log android --device ID  # Stream logs from a specific Android device
  drift log ios                  # Stream iOS simulator logs
  drift log ios --device         # Stream iOS device logs
  drift log ios --device <UDID>  # Stream logs from a specific device
  drift log xtool                # Stream xtool device logs
  drift log xtool --device <UDID or name>`,
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
		return logAndroid(args[1:])
	case "ios":
		return logIOS(cfg, args[1:])
	case "xtool":
		return logXtool(cfg, args[1:])
	default:
		return fmt.Errorf("unknown platform %q (use android, ios, or xtool)", platform)
	}
}

// logAndroid streams logs from Android device.
func logAndroid(args []string) error {
	adb := findADB()
	deviceID, _ := parseDeviceFlag(args)
	serial, err := resolveAndroidDevice(adb, deviceID)
	if err != nil {
		return err
	}

	fmt.Println("Streaming Android logs (Ctrl+C to stop)...")
	fmt.Println()
	ctx, cancel := signalContext()
	defer cancel()
	streamAndroidLogs(ctx, serial)
	return nil
}

// logIOS streams logs from iOS simulator or physical device.
func logIOS(cfg *config.Resolved, args []string) error {
	deviceID, device := parseDeviceFlag(args)

	ctx, cancel := signalContext()
	defer cancel()

	if device {
		resolved, err := resolveDevice(deviceID)
		if err != nil {
			return err
		}
		fmt.Println("Streaming iOS device logs (Ctrl+C to stop)...")
		fmt.Println()
		streamDeviceLogs(ctx, "Runner", resolved)
	} else {
		fmt.Println("Streaming iOS simulator logs (Ctrl+C to stop)...")
		fmt.Println()
		streamIOSSimulatorLogs(ctx, "booted", cfg.AppID)
	}
	return nil
}

// logXtool streams logs from a device built with xtool.
func logXtool(cfg *config.Resolved, args []string) error {
	deviceID, _ := parseDeviceFlag(args)

	resolved, err := resolveDevice(deviceID)
	if err != nil {
		return err
	}

	fmt.Println("Streaming device logs (Ctrl+C to stop)...")
	fmt.Println()
	ctx, cancel := signalContext()
	defer cancel()
	streamDeviceLogs(ctx, cfg.AppName, resolved)
	return nil
}
