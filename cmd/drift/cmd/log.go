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
		return logAndroid()
	case "ios":
		return logIOS(cfg.AppID)
	default:
		return fmt.Errorf("unknown platform %q (use android or ios)", platform)
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

// logIOS streams logs from iOS simulator.
func logIOS(appID string) error {
	fmt.Println("Streaming iOS simulator logs (Ctrl+C to stop)...")
	fmt.Println()
	ctx, cancel := watchContext()
	defer cancel()
	streamIOSSimulatorLogs(ctx, "booted", appID)
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
