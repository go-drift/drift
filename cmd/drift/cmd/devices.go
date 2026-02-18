package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	ios "github.com/danielpaulus/go-ios/ios"
)

func init() {
	RegisterCommand(&Command{
		Name:  "devices",
		Short: "List connected devices and simulators",
		Long: `List all connected devices and available simulators.

Shows:
  - Connected Android devices and emulators
  - Connected iOS devices (via usbmuxd)
  - Available iOS simulators (macOS only)

Use this to find device identifiers for running apps on specific devices.`,
		Usage: "drift devices",
		Run:   runDevices,
	})
}

func runDevices(args []string) error {
	fmt.Println("Connected devices and simulators:")
	fmt.Println()

	// List Android devices
	fmt.Println("Android devices:")
	if err := listAndroidDevices(); err != nil {
		fmt.Printf("  (Could not list Android devices: %v)\n", err)
	}
	fmt.Println()

	fmt.Println("iOS Devices:")
	if err := listIOSDevices(); err != nil {
		fmt.Printf("  (Could not list iOS devices: %v)\n", err)
	}
	fmt.Println()

	if runtime.GOOS == "darwin" {
		fmt.Println("iOS Simulators:")
		if err := listIOSSimulators(); err != nil {
			fmt.Printf("  (Could not list iOS simulators: %v)\n", err)
		}
	}

	return nil
}

func listAndroidDevices() error {
	adb := findADB()

	cmd := exec.Command(adb, "devices", "-l")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return err
	}

	lines := strings.Split(out.String(), "\n")
	deviceCount := 0
	for _, line := range lines[1:] { // Skip header
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse device line: <serial> <state> <info...>
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		serial := parts[0]
		state := parts[1]

		// Extract model if available
		model := ""
		for _, p := range parts[2:] {
			if after, ok := strings.CutPrefix(p, "model:"); ok {
				model = after
				break
			}
		}

		if state == "device" {
			deviceCount++
			if model != "" {
				fmt.Printf("  [%d] %s (%s)\n", deviceCount, model, serial)
			} else {
				fmt.Printf("  [%d] %s\n", deviceCount, serial)
			}
		} else if state == "unauthorized" {
			fmt.Printf("  [!] %s (unauthorized - check device for prompt)\n", serial)
		} else if state == "offline" {
			fmt.Printf("  [!] %s (offline)\n", serial)
		}
	}

	if deviceCount == 0 {
		fmt.Println("  No devices connected")
		fmt.Println()
		fmt.Println("  To connect a device:")
		fmt.Println("    1. Enable USB debugging on your Android device")
		fmt.Println("    2. Connect via USB")
		fmt.Println("    3. Authorize the connection on your device")
		fmt.Println()
		fmt.Println("  To start an emulator:")
		fmt.Println("    emulator -avd <avd-name>")
	} else {
		fmt.Println()
		fmt.Println("  Run with: drift run android --device <name or serial>")
	}

	return nil
}

// androidDevice holds the parsed fields from an `adb devices -l` line.
type androidDevice struct {
	serial string
	model  string
}

// resolveAndroidDevice resolves a device identifier (name, serial, or empty
// for auto-detect) into an adb serial string. Runs `adb devices -l` and
// matches by serial (exact) or model name (case-insensitive).
func resolveAndroidDevice(adb, id string) (string, error) {
	cmd := exec.Command(adb, "devices", "-l")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to list Android devices: %w", err)
	}

	var devices []androidDevice
	for _, line := range strings.Split(out.String(), "\n")[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 || parts[1] != "device" {
			continue
		}
		d := androidDevice{serial: parts[0]}
		for _, p := range parts[2:] {
			if after, ok := strings.CutPrefix(p, "model:"); ok {
				d.model = after
				break
			}
		}
		devices = append(devices, d)
	}

	if id == "" {
		switch len(devices) {
		case 0:
			return "", fmt.Errorf("no connected Android devices found\nConnect a device via USB, or specify one with --device <name or serial>")
		case 1:
			d := devices[0]
			if d.model != "" {
				fmt.Printf("  Auto-detected device: %s (%s)\n", d.model, d.serial)
			} else {
				fmt.Printf("  Auto-detected device: %s\n", d.serial)
			}
			return d.serial, nil
		default:
			return "", fmt.Errorf("multiple Android devices connected, specify one with --device <name or serial>:\n%s", formatAndroidDeviceList(devices))
		}
	}

	// Exact serial match.
	for _, d := range devices {
		if d.serial == id {
			return d.serial, nil
		}
	}

	// Case-insensitive model match.
	for _, d := range devices {
		if d.model != "" && strings.EqualFold(d.model, id) {
			return d.serial, nil
		}
	}

	listing := formatAndroidDeviceList(devices)
	if len(devices) == 0 {
		listing = "  (none)"
	}
	return "", fmt.Errorf("device %q not found\nConnected devices:\n%s", id, listing)
}

func formatAndroidDeviceList(devices []androidDevice) string {
	var lines []string
	for _, d := range devices {
		if d.model != "" {
			lines = append(lines, fmt.Sprintf("  %s (%s)", d.model, d.serial))
		} else {
			lines = append(lines, fmt.Sprintf("  %s", d.serial))
		}
	}
	return strings.Join(lines, "\n")
}

// listIOSDevices lists connected iOS devices using the go-ios library.
func listIOSDevices() error {
	deviceList, err := ios.ListDevices()
	if err != nil {
		return err
	}
	if len(deviceList.DeviceList) == 0 {
		fmt.Println("  No devices connected")
		fmt.Println()
		fmt.Println("  To connect a device:")
		fmt.Println("    1. Connect your iOS device via USB")
		fmt.Println("    2. Trust the computer on your device")
		fmt.Println("    3. Ensure usbmuxd is running")
		return nil
	}
	for i, d := range deviceList.DeviceList {
		udid := d.Properties.SerialNumber
		vals, err := ios.GetValues(d)
		if err == nil && vals.Value.DeviceName != "" {
			fmt.Printf("  [%d] %s (%s, %s %s)\n", i+1,
				vals.Value.DeviceName, vals.Value.ProductType,
				vals.Value.ProductName, vals.Value.ProductVersion)
			fmt.Printf("      UDID: %s\n", udid)
		} else {
			fmt.Printf("  [%d] %s\n", i+1, udid)
		}
	}
	fmt.Println()
	if runtime.GOOS == "darwin" {
		fmt.Println("  Run with: drift run ios --device <name or UDID>")
	} else {
		fmt.Println("  Run with: drift run xtool --device <name or UDID>")
	}
	return nil
}

// resolveDevice resolves a device identifier (name, UDID, or empty for
// auto-detect) into an ios.DeviceEntry. Uses go-ios to enumerate connected
// devices and match by UDID or device name (case-insensitive).
func resolveDevice(id string) (ios.DeviceEntry, error) {
	deviceList, err := ios.ListDevices()
	if err != nil {
		return ios.DeviceEntry{}, fmt.Errorf("could not list iOS devices: %w", err)
	}

	devices := deviceList.DeviceList

	// Resolve names once upfront to avoid redundant USB round-trips.
	names := make([]string, len(devices))
	for i, d := range devices {
		names[i] = deviceName(d)
	}

	if id == "" {
		switch len(devices) {
		case 0:
			return ios.DeviceEntry{}, fmt.Errorf("no connected iOS devices found\nConnect a device via USB, or specify one with --device <name-or-udid>")
		case 1:
			fmt.Printf("  Auto-detected device: %s (%s)\n", names[0], devices[0].Properties.SerialNumber)
			return devices[0], nil
		default:
			return ios.DeviceEntry{}, fmt.Errorf("multiple iOS devices connected, specify one with --device <name-or-udid>:\n%s", formatDeviceListCached(devices, names))
		}
	}

	// Match by UDID first (case-sensitive).
	for _, d := range devices {
		if d.Properties.SerialNumber == id {
			return d, nil
		}
	}

	// Match by device name (case-insensitive).
	for i, d := range devices {
		if strings.EqualFold(names[i], id) {
			return d, nil
		}
	}

	listing := formatDeviceListCached(devices, names)
	if len(devices) == 0 {
		listing = "  (none)"
	}
	return ios.DeviceEntry{}, fmt.Errorf("device %q not found\nConnected devices:\n%s", id, listing)
}

// deviceName returns the user-visible name of a device, falling back to the
// UDID if the name cannot be retrieved.
func deviceName(d ios.DeviceEntry) string {
	vals, err := ios.GetValues(d)
	if err == nil && vals.Value.DeviceName != "" {
		return vals.Value.DeviceName
	}
	return d.Properties.SerialNumber
}

// formatDeviceListCached formats a list of devices using pre-resolved names.
func formatDeviceListCached(devices []ios.DeviceEntry, names []string) string {
	var lines []string
	for i, d := range devices {
		udid := d.Properties.SerialNumber
		if names[i] != udid {
			lines = append(lines, fmt.Sprintf("  %s (%s)", names[i], udid))
		} else {
			lines = append(lines, fmt.Sprintf("  %s", udid))
		}
	}
	return strings.Join(lines, "\n")
}

func listIOSSimulators() error {
	cmd := exec.Command("xcrun", "simctl", "list", "devices", "available", "--json")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return err
	}

	var result struct {
		Devices map[string][]struct {
			Name  string `json:"name"`
			State string `json:"state"`
			UDID  string `json:"udid"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return fmt.Errorf("failed to parse simctl output: %w", err)
	}

	bootedCount := 0
	fmt.Println("  Booted:")
	for runtime, devices := range result.Devices {
		if !strings.Contains(runtime, "iOS") {
			continue
		}
		// Extract a readable runtime name (e.g. "iOS 17.2") from the identifier.
		runtimeName := runtime
		if i := strings.LastIndex(runtime, "SimRuntime."); i != -1 {
			runtimeName = strings.ReplaceAll(runtime[i+len("SimRuntime."):], "-", " ")
			// Collapse "iOS 17 2" into "iOS 17.2": put a dot before the last segment.
			if parts := strings.SplitN(runtimeName, " ", 3); len(parts) == 3 {
				runtimeName = parts[0] + " " + parts[1] + "." + parts[2]
			}
		}
		for _, d := range devices {
			if d.State == "Booted" {
				bootedCount++
				fmt.Printf("    [%d] %s (%s)\n", bootedCount, d.Name, runtimeName)
			}
		}
	}
	if bootedCount == 0 {
		fmt.Println("    (none)")
	}

	fmt.Println()
	fmt.Println("  Available (run with 'drift run ios --simulator \"<name>\"'):")

	availableCount := 0
	for _, devices := range result.Devices {
		for _, d := range devices {
			if strings.Contains(d.Name, "iPhone") && availableCount < 5 {
				availableCount++
				fmt.Printf("    • %s\n", d.Name)
			}
		}
	}
	if availableCount == 0 {
		fmt.Println("    (none)")
	} else {
		fmt.Println("    ...")
	}

	return nil
}
