package cmd

import (
	"bytes"
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
	if id == "" {
		switch len(devices) {
		case 0:
			return ios.DeviceEntry{}, fmt.Errorf("no connected iOS devices found\nConnect a device via USB, or specify one with --device <name-or-udid>")
		case 1:
			name := deviceName(devices[0])
			fmt.Printf("  Auto-detected device: %s (%s)\n", name, devices[0].Properties.SerialNumber)
			return devices[0], nil
		default:
			return ios.DeviceEntry{}, fmt.Errorf("multiple iOS devices connected, specify one with --device <name-or-udid>:\n%s", formatDeviceList(devices))
		}
	}

	// Match by UDID first (case-sensitive).
	for _, d := range devices {
		if d.Properties.SerialNumber == id {
			return d, nil
		}
	}

	// Match by device name (case-insensitive).
	for _, d := range devices {
		if strings.EqualFold(deviceName(d), id) {
			return d, nil
		}
	}

	listing := formatDeviceList(devices)
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

// formatDeviceList formats a list of devices for display in error messages.
func formatDeviceList(devices []ios.DeviceEntry) string {
	var lines []string
	for _, d := range devices {
		name := deviceName(d)
		udid := d.Properties.SerialNumber
		if name != udid {
			lines = append(lines, fmt.Sprintf("  %s (%s)", name, udid))
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

	// Simple parsing - look for device names and states
	output := out.String()

	// Find booted devices first
	bootedCount := 0
	fmt.Println("  Booted:")

	// Parse the output looking for booted devices
	lines := strings.Split(output, "\n")
	inDevices := false
	currentRuntime := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Track runtime sections
		if strings.Contains(line, "iOS") && strings.Contains(line, ":") {
			currentRuntime = strings.Trim(strings.TrimSuffix(line, ":"), `"`)
			inDevices = true
			continue
		}

		if inDevices && strings.Contains(line, `"name"`) {
			// Extract device name
			name := extractJSONString(line, "name")
			// Look ahead for state
			stateIdx := strings.Index(output, line)
			if stateIdx != -1 {
				chunk := output[stateIdx:min(stateIdx+500, len(output))]
				if strings.Contains(chunk, `"state" : "Booted"`) {
					bootedCount++
					fmt.Printf("    [%d] %s (%s)\n", bootedCount, name, currentRuntime)
				}
			}
		}
	}

	if bootedCount == 0 {
		fmt.Println("    (none)")
	}

	fmt.Println()
	fmt.Println("  Available (run with 'drift run ios --simulator \"<name>\"'):")

	// List some common available simulators
	availableCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, `"name"`) {
			name := extractJSONString(line, "name")
			if strings.Contains(name, "iPhone") && availableCount < 5 {
				availableCount++
				fmt.Printf("    • %s\n", name)
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

func extractJSONString(line, key string) string {
	// Simple extraction: "key" : "value"
	keyPattern := fmt.Sprintf(`"%s"`, key)
	_, after, ok := strings.Cut(line, keyPattern)
	if !ok {
		return ""
	}

	rest := after
	// Find the value
	start := strings.Index(rest, `"`)
	if start == -1 {
		return ""
	}
	rest = rest[start+1:]
	before, _, ok := strings.Cut(rest, `"`)
	if !ok {
		return ""
	}
	return before
}
