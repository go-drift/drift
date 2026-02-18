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
  - Connected iOS devices (via Xcode on macOS, via usbmuxd on Linux)
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
	if runtime.GOOS == "darwin" {
		if err := listIOSDevices(); err != nil {
			fmt.Printf("  (Could not list iOS devices: %v)\n", err)
		}
	} else {
		if err := listIOSDevicesGoIOS(); err != nil {
			fmt.Printf("  (Could not list iOS devices: %v)\n", err)
		}
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
	}

	return nil
}

func listIOSDevices() error {
	devices, err := connectedIOSDevices()
	if err != nil {
		return err
	}

	if len(devices) == 0 {
		fmt.Println("  No devices connected")
		fmt.Println()
		fmt.Println("  To connect a device:")
		fmt.Println("    1. Connect your iOS device via USB")
		fmt.Println("    2. Trust the computer on your device")
		fmt.Println("    3. Ensure device is unlocked")
	} else {
		for i, d := range devices {
			fmt.Printf("  [%d] %s (%s)\n", i+1, d.name, d.udid)
		}
		fmt.Println()
		fmt.Printf("  Run with: drift run ios --device <UDID>\n")
	}

	return nil
}

// listIOSDevicesGoIOS lists connected iOS devices using the go-ios library.
// Used on Linux where xcrun is not available.
func listIOSDevicesGoIOS() error {
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
	fmt.Printf("  Run with: drift run xtool --device <UDID>\n")
	return nil
}

// iosDevice holds the name and UDID of a connected iOS device.
type iosDevice struct {
	name string
	udid string
}

// connectedIOSDevices returns the list of physical iOS devices reported by
// xcrun xctrace list devices, excluding simulators and the host Mac.
func connectedIOSDevices() ([]iosDevice, error) {
	cmd := exec.Command("xcrun", "xctrace", "list", "devices")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var devices []iosDevice
	lines := strings.Split(out.String(), "\n")
	inDeviceSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "== Devices ==" {
			inDeviceSection = true
			continue
		}
		if line == "== Simulators ==" {
			break
		}
		if !inDeviceSection || line == "" {
			continue
		}

		// Format: DeviceName (Version) (UDID)
		lastOpen := strings.LastIndex(line, "(")
		lastClose := strings.LastIndex(line, ")")
		if lastOpen == -1 || lastClose == -1 || lastClose <= lastOpen {
			continue
		}

		udid := line[lastOpen+1 : lastClose]
		if strings.Count(udid, ".") >= 2 {
			continue
		}

		// The text before the UDID group. iOS devices include a version:
		//   "Tobys iPhone (26.0.1)" -> has parens -> iOS device
		//   "Toby's MacBook Pro"    -> no parens -> host Mac, skip
		rest := strings.TrimSpace(line[:lastOpen])
		hasVersion := false
		if versionEnd := strings.LastIndex(rest, ")"); versionEnd != -1 {
			if versionStart := strings.LastIndex(rest[:versionEnd], "("); versionStart != -1 {
				rest = strings.TrimSpace(rest[:versionStart])
				hasVersion = true
			}
		}

		if !hasVersion {
			continue
		}

		devices = append(devices, iosDevice{name: rest, udid: udid})
	}

	return devices, nil
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
