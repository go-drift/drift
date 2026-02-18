package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	ios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/syslog"
)

// streamAndroidLogs streams tag-filtered logcat output until ctx is
// cancelled. Tag-based filtering survives app restarts, unlike PID-based.
// When serial is non-empty, targets that specific device via `-s`.
// Intended to run as a goroutine.
func streamAndroidLogs(ctx context.Context, serial string) {
	adb := findADB()

	// Clear stale logs so the stream starts fresh
	adbCommand(adb, serial, "logcat", "-c").Run()

	logcatArgs := []string{"logcat", "-v", "time",
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
	}
	if serial != "" {
		logcatArgs = append([]string{"-s", serial}, logcatArgs...)
	}
	cmd := exec.CommandContext(ctx, adb, logcatArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run() // exits when ctx is cancelled
}

// streamDeviceLogs streams physical-device logs filtered by process name until
// ctx is cancelled. Uses go-ios to connect to the syslog relay service
// directly, so no external tools (like libimobiledevice) are required.
// processName should be "Runner" for xcodeproj builds or the app name for xtool.
// Intended to run as a goroutine.
func streamDeviceLogs(ctx context.Context, processName string, device ios.DeviceEntry) {
	conn, err := syslog.New(device)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not start syslog stream: %v\n", err)
		return
	}
	defer conn.Close()

	type logMsg struct {
		raw string
		err error
	}
	ch := make(chan logMsg, 1)

	go func() {
		for {
			msg, err := conn.ReadLogMessage()
			select {
			case ch <- logMsg{msg, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	parse := syslog.Parser()
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-ch:
			if m.err != nil {
				fmt.Fprintf(os.Stderr, "Error: syslog read failed: %v\n", m.err)
				return
			}
			entry, err := parse(m.raw)
			if err != nil {
				continue
			}
			if entry.Process == processName {
				fmt.Println(entry.Message)
			}
		}
	}
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

// signalContext creates a cancellable context that is cancelled on
// SIGINT or SIGTERM.
func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigChan:
			cancel()
		case <-ctx.Done():
		}
		signal.Stop(sigChan)
	}()
	return ctx, cancel
}
