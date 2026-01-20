package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
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

	switch platform {
	case "android":
		return logAndroid()
	case "ios":
		return logIOS()
	default:
		return fmt.Errorf("unknown platform %q (use android or ios)", platform)
	}
}

// logAndroid streams logs from Android device.
func logAndroid() error {
	fmt.Println("Streaming Android logs (Ctrl+C to stop)...")
	fmt.Println()

	adb := findADBForLog()

	// Clear existing logs first
	clearCmd := exec.Command(adb, "logcat", "-c")
	if err := clearCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to clear logcat: %v\n", err)
	}

	// Stream logs filtered for Drift
	cmd := exec.Command(adb, "logcat",
		"-v", "time",
		"DriftJNI:*",
		"DriftDeepLink:*",
		"Go:*",
		"drift:*",
		"AndroidRuntime:E",
		"*:S", // Silence other tags
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
		// Check if killed by signal
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == -1 {
			fmt.Println("\nLog streaming stopped.")
			return nil
		}
		return fmt.Errorf("logcat failed: %w", err)
	}

	return nil
}

// logIOS streams logs from iOS simulator.
func logIOS() error {
	fmt.Println("Streaming iOS simulator logs (Ctrl+C to stop)...")
	fmt.Println()

	// Get predicate for filtering
	// This filters for our app's process
	cmd := exec.Command("log", "stream",
		"--predicate", `subsystem CONTAINS "drift" OR process CONTAINS "Runner"`,
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
