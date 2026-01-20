package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func init() {
	RegisterCommand(&Command{
		Name:  "devices",
		Short: "List connected devices and simulators",
		Long: `List all connected devices and available simulators.

Shows:
  - Connected Android devices and emulators
  - Connected iOS devices (macOS only, requires ios-deploy)
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

	// List iOS devices and simulators (macOS only)
	if runtime.GOOS == "darwin" {
		fmt.Println("iOS Devices:")
		if err := listIOSDevices(); err != nil {
			fmt.Printf("  (Could not list iOS devices: %v)\n", err)
		}
		fmt.Println()

		fmt.Println("iOS Simulators:")
		if err := listIOSSimulators(); err != nil {
			fmt.Printf("  (Could not list iOS simulators: %v)\n", err)
		}
	}

	return nil
}

func listAndroidDevices() error {
	adb := "adb"
	if sdkRoot := os.Getenv("ANDROID_SDK_ROOT"); sdkRoot != "" {
		adb = filepath.Join(sdkRoot, "platform-tools", "adb")
	} else if androidHome := os.Getenv("ANDROID_HOME"); androidHome != "" {
		adb = filepath.Join(androidHome, "platform-tools", "adb")
	}

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
			if strings.HasPrefix(p, "model:") {
				model = strings.TrimPrefix(p, "model:")
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
	// Try ios-deploy first (preferred, more reliable)
	if _, err := exec.LookPath("ios-deploy"); err == nil {
		return listIOSDevicesWithIOSDeploy()
	}

	// Fall back to xcrun xctrace list devices
	return listIOSDevicesWithXctrace()
}

func listIOSDevicesWithIOSDeploy() error {
	cmd := exec.Command("ios-deploy", "-c", "-t", "1")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		// ios-deploy returns error if no devices found, check output
		output := out.String()
		if strings.Contains(output, "Found") || output == "" {
			fmt.Println("  No devices connected")
			fmt.Println()
			fmt.Println("  To connect a device:")
			fmt.Println("    1. Connect your iOS device via USB")
			fmt.Println("    2. Trust the computer on your device")
			fmt.Println("    3. Ensure device is unlocked")
			return nil
		}
		return err
	}

	// Parse ios-deploy output
	// Format: [....] Found <UDID> (<DeviceName>, <Model>, <Version>, <Arch>) ...
	lines := strings.Split(out.String(), "\n")
	deviceCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "Found") {
			continue
		}

		// Extract device info from line
		// Example: [....] Found 00008030-001234567890 (iPhone, iPhone 14 Pro, 17.0, arm64e) ...
		parts := strings.SplitN(line, "Found ", 2)
		if len(parts) < 2 {
			continue
		}

		rest := parts[1]
		// Find UDID (first space-separated word)
		fields := strings.Fields(rest)
		if len(fields) < 1 {
			continue
		}

		udid := fields[0]

		// Extract device name from parentheses
		deviceName := ""
		if start := strings.Index(rest, "("); start != -1 {
			if end := strings.Index(rest, ")"); end > start {
				info := rest[start+1 : end]
				infoParts := strings.Split(info, ", ")
				if len(infoParts) >= 2 {
					deviceName = infoParts[1] // Model name
				} else if len(infoParts) >= 1 {
					deviceName = infoParts[0] // Device type
				}
			}
		}

		deviceCount++
		if deviceName != "" {
			fmt.Printf("  [%d] %s (%s)\n", deviceCount, deviceName, udid)
		} else {
			fmt.Printf("  [%d] %s\n", deviceCount, udid)
		}
	}

	if deviceCount == 0 {
		fmt.Println("  No devices connected")
		fmt.Println()
		fmt.Println("  To connect a device:")
		fmt.Println("    1. Connect your iOS device via USB")
		fmt.Println("    2. Trust the computer on your device")
		fmt.Println("    3. Ensure device is unlocked")
	} else {
		fmt.Println()
		fmt.Printf("  Run with: drift run ios --device <UDID>\n")
	}

	return nil
}

func listIOSDevicesWithXctrace() error {
	cmd := exec.Command("xcrun", "xctrace", "list", "devices")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return err
	}

	// Parse xctrace output
	// Format: <DeviceName> (<Version>) (<UDID>)
	lines := strings.Split(out.String(), "\n")
	deviceCount := 0
	inDeviceSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip header and simulator section
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

		// Parse device line: DeviceName (Version) (UDID)
		// Find last parentheses for UDID
		lastOpen := strings.LastIndex(line, "(")
		lastClose := strings.LastIndex(line, ")")
		if lastOpen == -1 || lastClose == -1 || lastClose <= lastOpen {
			continue
		}

		udid := line[lastOpen+1 : lastClose]
		// Skip if UDID looks like a version number
		if strings.Count(udid, ".") >= 2 {
			continue
		}

		// Get device name (everything before version)
		rest := strings.TrimSpace(line[:lastOpen])
		// Remove version in parentheses
		if versionEnd := strings.LastIndex(rest, ")"); versionEnd != -1 {
			if versionStart := strings.LastIndex(rest[:versionEnd], "("); versionStart != -1 {
				rest = strings.TrimSpace(rest[:versionStart])
			}
		}

		deviceName := rest

		// Skip entries that look like the host machine (usually Mac)
		if strings.Contains(deviceName, "Mac") {
			continue
		}

		deviceCount++
		fmt.Printf("  [%d] %s (%s)\n", deviceCount, deviceName, udid)
	}

	if deviceCount == 0 {
		fmt.Println("  No devices connected")
		fmt.Println()
		fmt.Println("  To connect a device:")
		fmt.Println("    1. Connect your iOS device via USB")
		fmt.Println("    2. Trust the computer on your device")
		fmt.Println("    3. Ensure device is unlocked")
		fmt.Println()
		fmt.Println("  Tip: Install ios-deploy for better device detection:")
		fmt.Println("    brew install ios-deploy")
	} else {
		fmt.Println()
		fmt.Printf("  Run with: drift run ios --device <UDID>\n")
	}

	return nil
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
				chunk := output[stateIdx : min(stateIdx+500, len(output))]
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
	idx := strings.Index(line, keyPattern)
	if idx == -1 {
		return ""
	}

	rest := line[idx+len(keyPattern):]
	// Find the value
	start := strings.Index(rest, `"`)
	if start == -1 {
		return ""
	}
	rest = rest[start+1:]
	end := strings.Index(rest, `"`)
	if end == -1 {
		return ""
	}
	return rest[:end]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
