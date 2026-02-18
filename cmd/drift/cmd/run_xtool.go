package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"

	ios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/instruments"
	"github.com/danielpaulus/go-ios/ios/tunnel"
	"github.com/go-drift/drift/cmd/drift/internal/config"
	"github.com/go-drift/drift/cmd/drift/internal/workspace"
)

type xtoolRunOptions struct {
	deviceID string
	noLogs   bool
	watch    bool
}

// parseXtoolRunArgs parses xtool-specific flags from the argument list and
// returns the resolved options.
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

// startTunnelAndGetDevice establishes an inline userspace tunnel to an iOS 17.4+
// device and returns an enriched DeviceEntry with RSD tunnel info. The returned
// close function must be called to tear down the tunnel when done. For pre-17.4
// devices (where CoreDeviceProxy is unavailable), the tunnel step fails and the
// plain usbmuxd device entry is returned with a nil closer.
func startTunnelAndGetDevice(deviceID string) (ios.DeviceEntry, func() error, error) {
	device, err := ios.GetDevice(deviceID)
	if err != nil {
		return ios.DeviceEntry{}, nil, fmt.Errorf("no iOS device found. Is a device connected and trusted? (usbmuxd: %v)", err)
	}

	// Pick a free port for the userspace TUN listener.
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Note: could not allocate tunnel port: %v\n", err)
		return device, nil, nil
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	tun, err := tunnel.ConnectUserSpaceTunnelLockdown(device, port)
	if err != nil {
		// Pre-17.4 device or CoreDeviceProxy unavailable: fall back to plain device.
		fmt.Fprintf(os.Stderr, "Note: tunnel setup skipped (pre-17.4 device?): %v\n", err)
		return device, nil, nil
	}

	udid := device.Properties.SerialNumber

	// Set userspace TUN fields on the device before the RSD connection so that
	// ConnectTUNDevice routes through the local TCP proxy instead of trying to
	// reach the IPv6 tunnel address directly.
	device.UserspaceTUN = true
	device.UserspaceTUNHost = "127.0.0.1"
	device.UserspaceTUNPort = port

	rsdService, err := ios.NewWithAddrPortDevice(tun.Address, tun.RsdPort, device)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Note: RSD connection failed, falling back to plain device: %v\n", err)
		tun.Close()
		return device, nil, nil
	}

	rsdProvider, err := rsdService.Handshake()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Note: RSD handshake failed, falling back to plain device: %v\n", err)
		rsdService.Close()
		tun.Close()
		return device, nil, nil
	}
	rsdService.Close()

	enriched, err := ios.GetDeviceWithAddress(udid, tun.Address, rsdProvider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Note: could not enrich device with tunnel info, falling back to plain device: %v\n", err)
		tun.Close()
		return device, nil, nil
	}
	enriched.UserspaceTUN = true
	enriched.UserspaceTUNHost = "127.0.0.1"
	enriched.UserspaceTUNPort = port

	return enriched, tun.Close, nil
}

// runXtool builds and runs on iOS device using xtool (no Xcode required).
func runXtool(ws *workspace.Workspace, cfg *config.Resolved, args []string, opts runOptions) error {
	xtoolOpts := parseXtoolRunArgs(args)
	if opts.noLogs {
		xtoolOpts.noLogs = true
	}
	xtoolOpts.watch = opts.watch

	// Start the tunnel once for the entire session.
	device, closeTunnel, err := startTunnelAndGetDevice(xtoolOpts.deviceID)
	if err != nil {
		return err
	}
	if closeTunnel != nil {
		defer closeTunnel()
	}

	// Build the app first
	buildOpts := xtoolBuildOptions{buildOptions: buildOptions{noFetch: opts.noFetch}, release: false, device: true}
	if err := buildXtool(ws, buildOpts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Running on iOS Device (via xtool)...")

	killRunningApp(device, cfg.AppName)
	if err := xtoolDevRun(ws, xtoolOpts); err != nil {
		return err
	}

	// Resolve the actual on-device bundle ID. xtool signs with a team ID prefix
	// (e.g. "ABCDE12345.com.example.app"), so the config bundle ID won't match.
	baseDevice, err := ios.GetDevice(xtoolOpts.deviceID)
	if err != nil {
		return fmt.Errorf("could not reconnect to device: %w", err)
	}
	installedBundleID, err := resolveInstalledBundleID(baseDevice, cfg.AppID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: could not resolve installed bundle ID: %v\n", err)
		installedBundleID = cfg.AppID
	}

	fmt.Println("Launching app (ensure the device screen is unlocked)...")
	if err := launchAppOnDevice(device, installedBundleID); err != nil {
		if strings.Contains(err.Error(), "Timed out") {
			fmt.Println("Launch timed out. The app may have started; if not, unlock the device and open it manually.")
		} else {
			fmt.Fprintf(os.Stderr, "\nWarning: could not launch app automatically: %v\n", err)
			fmt.Println("Open the app manually on your device.")
		}
	} else {
		fmt.Println("App launched.")
	}
	fmt.Println()

	ctx, cancel := signalContext()
	defer cancel()

	if !xtoolOpts.noLogs {
		go streamDeviceLogs(ctx, cfg.AppName, xtoolOpts.deviceID)
	}

	if xtoolOpts.watch {
		return watchAndRun(ctx, ws, func() error {
			killRunningApp(device, cfg.AppName)
			if err := ws.Refresh(); err != nil {
				return err
			}
			if err := buildXtool(ws, buildOpts); err != nil {
				return err
			}
			if err := xtoolDevRun(ws, xtoolOpts); err != nil {
				return err
			}
			if err := launchAppOnDevice(device, installedBundleID); err != nil {
				if strings.Contains(err.Error(), "Timed out") {
					fmt.Println("Relaunch timed out. The app may have started; if not, unlock the device and open it manually.")
				} else {
					fmt.Fprintf(os.Stderr, "\nWarning: could not relaunch app: %v\n", err)
					fmt.Println("Open the app manually on your device.")
				}
			} else {
				fmt.Println("App relaunched.")
			}
			return nil
		})
	}

	if xtoolOpts.noLogs {
		return nil
	}
	<-ctx.Done()
	return nil
}

// resolveInstalledBundleID queries the device's installation proxy to find the
// actual bundle ID of an installed app. xtool signs apps with a team ID prefix
// (e.g. "ABCDE12345.com.example.app"), so the on-device bundle ID differs from
// the project config. This function finds the installed app whose bundle ID
// ends with the given suffix. It uses the base usbmuxd device (no tunnel needed).
func resolveInstalledBundleID(device ios.DeviceEntry, bundleIDSuffix string) (string, error) {
	conn, err := installationproxy.New(device)
	if err != nil {
		return "", fmt.Errorf("could not connect to installation proxy: %w", err)
	}
	defer conn.Close()
	apps, err := conn.BrowseUserApps()
	if err != nil {
		return "", fmt.Errorf("could not list installed apps: %w", err)
	}
	for _, app := range apps {
		bid := app.CFBundleIdentifier()
		if strings.HasSuffix(bid, "."+bundleIDSuffix) || bid == bundleIDSuffix {
			return bid, nil
		}
	}
	return "", fmt.Errorf("app %q not found on device", bundleIDSuffix)
}

// launchAppOnDevice launches the app on a connected iOS device using the
// instruments process control service. The device entry may be enriched with
// tunnel info so instruments can reach the service through the userspace TUN
// proxy on iOS 17+.
func launchAppOnDevice(device ios.DeviceEntry, bundleID string) error {
	pc, err := instruments.NewProcessControl(device)
	if err != nil {
		return fmt.Errorf("could not connect to instruments service: %w", err)
	}
	defer pc.Close()
	_, err = pc.LaunchApp(bundleID, map[string]any{"KillExisting": uint64(1)})
	if err != nil {
		return fmt.Errorf("instruments launch failed: %w", err)
	}
	return nil
}

// killRunningApp finds and kills a running app by name on a connected iOS device.
// Uses instruments to list processes and kill by PID. Best-effort; errors are
// logged to stderr but not returned since this runs before reinstall.
func killRunningApp(device ios.DeviceEntry, appName string) {
	info, err := instruments.NewDeviceInfoService(device)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Note: could not list processes for kill: %v\n", err)
		return
	}
	defer info.Close()

	procs, err := info.ProcessList()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Note: could not list processes for kill: %v\n", err)
		return
	}

	for _, p := range procs {
		if p.IsApplication && p.Name == appName {
			pc, err := instruments.NewProcessControl(device)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Note: could not connect to instruments for kill: %v\n", err)
				return
			}
			if err := pc.KillProcess(p.Pid); err != nil {
				fmt.Fprintf(os.Stderr, "Note: could not kill running app: %v\n", err)
			}
			pc.Close()
			return
		}
	}
}

// xtoolDevRun installs the app on a connected iOS device using the xtool dev
// run command. Launching is handled separately by launchAppOnDevice.
func xtoolDevRun(ws *workspace.Workspace, opts xtoolRunOptions) error {
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
	return nil
}
