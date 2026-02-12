package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/go-drift/drift/cmd/drift/internal/config"
)

func init() {
	RegisterCommand(&Command{
		Name:  "log",
		Short: "Show application logs",
		Long: `Stream logs from the running application.

For Android, this uses adb logcat filtered to show Drift and Go logs.
For iOS, this shows Console logs from the simulator.

Usage:
  drift log android   # Stream Android logs
  drift log ios       # Stream iOS simulator logs`,
		Usage: "drift log <platform>",
		Run:   runLog,
	})
}

func runLog(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("platform is required (android or ios)\n\nUsage: drift log <platform>")
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
		return logAndroid(cfg.AppID)
	case "ios":
		return logIOS(cfg.AppID)
	default:
		return fmt.Errorf("unknown platform %q (use android or ios)", platform)
	}
}

// logAndroid streams logs from Android device.
func logAndroid(appID string) error {
	fmt.Println("Streaming Android logs (Ctrl+C to stop)...")
	fmt.Println()

	adb := findADBForLog()

	// Clear existing logs first
	clearCmd := exec.Command(adb, "logcat", "-c")
	if err := clearCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to clear logcat: %v\n", err)
	}

	// Try to get PID of the app (with fallbacks for older devices)
	pid := getAppPID(adb, appID)

	var cmd *exec.Cmd
	if pid != "" {
		// Test if --pid is supported (fails on old adb/devices)
		testCmd := exec.Command(adb, "logcat", "-d", "--pid", pid)
		if err := testCmd.Run(); err == nil {
			// PID-based filtering (preferred) - app logs only
			cmd = exec.Command(adb, "logcat", "-v", "time", "--pid", pid)
		} else {
			pid = "" // Force fallback
		}
	}

	if pid == "" {
		// Fallback: tag-based filtering (includes crash logs)
		fmt.Fprintf(os.Stderr, "Note: using tag-based filtering (includes crash logs)\n")
		cmd = exec.Command(adb, "logcat", "-v", "time",
			"DriftJNI:*",
			"DriftAccessibility:*",
			"DriftDeepLink:*",
			"SkiaHostView:*",
			"DriftBackground:*",
			"DriftPush:*",
			"DriftSkia:*",
			"PlatformChannel:*",
			"Go:*",
			"AndroidRuntime:E",
			"*:S",
		)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		cmd.Process.Kill()
	}()

	if err := cmd.Run(); err != nil {
		// Check if killed by signal
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == -1 {
			fmt.Println("\nLog streaming stopped.")
			return nil
		}
		return fmt.Errorf("logcat failed: %w", err)
	}

	return nil
}

// getAppPID tries multiple methods to get the app's PID.
func getAppPID(adb, appID string) string {
	// Try pidof -s first (single PID, most common)
	if out, err := exec.Command(adb, "shell", "pidof", "-s", appID).Output(); err == nil {
		if pid := strings.TrimSpace(string(out)); pid != "" {
			return pid
		}
	}

	// Fallback: pidof without -s (some devices)
	if out, err := exec.Command(adb, "shell", "pidof", appID).Output(); err == nil {
		if pid := strings.TrimSpace(string(out)); pid != "" {
			// May return multiple PIDs, take first
			fields := strings.Fields(pid)
			if len(fields) > 0 {
				return fields[0]
			}
		}
	}

	return "" // PID unavailable
}

// logIOS streams logs from iOS simulator.
func logIOS(appID string) error {
	fmt.Println("Streaming iOS simulator logs (Ctrl+C to stop)...")
	fmt.Println()

	// Filter by app's bundle ID (which is used as the os_log subsystem)
	// This captures DriftLog.* calls; NSLog and third-party logs are excluded to reduce noise
	predicate := fmt.Sprintf(`subsystem == "%s"`, appID)

	cmd := exec.Command("log", "stream",
		"--predicate", predicate,
		"--level", "debug",
		"--style", "compact",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		cmd.Process.Kill()
	}()

	if err := cmd.Run(); err != nil {
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == -1 {
			fmt.Println("\nLog streaming stopped.")
			return nil
		}
		return fmt.Errorf("log stream failed: %w", err)
	}

	return nil
}

// findADB locates the adb executable (duplicated for simplicity).
func findADBForLog() string {
	if sdkRoot := os.Getenv("ANDROID_SDK_ROOT"); sdkRoot != "" {
		return filepath.Join(sdkRoot, "platform-tools", "adb")
	}
	if androidHome := os.Getenv("ANDROID_HOME"); androidHome != "" {
		return filepath.Join(androidHome, "platform-tools", "adb")
	}
	return "adb"
}
