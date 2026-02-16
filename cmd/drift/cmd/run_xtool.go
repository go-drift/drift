package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

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

// runXtool builds and runs on iOS device using xtool (no Xcode required).
func runXtool(ws *workspace.Workspace, cfg *config.Resolved, args []string, opts runOptions) error {
	xtoolOpts := parseXtoolRunArgs(args)
	if opts.noLogs {
		xtoolOpts.noLogs = true
	}
	xtoolOpts.watch = opts.watch

	// Build the app first
	buildOpts := xtoolBuildOptions{buildOptions: buildOptions{noFetch: opts.noFetch}, release: false, device: true}
	if err := buildXtool(ws, buildOpts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Running on iOS Device (via xtool)...")

	if err := xtoolDevRun(ws, xtoolOpts); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("App installed. Close and relaunch the app on your device to reload.")
	fmt.Println()

	if xtoolOpts.watch {
		ctx, cancel := watchContext()
		defer cancel()
		return watchAndRun(ctx, ws, func() error {
			if err := ws.Refresh(); err != nil {
				return err
			}
			if err := buildXtool(ws, buildOpts); err != nil {
				return err
			}
			fmt.Println()
			fmt.Println("Close the app on your device to install the update...")
			return xtoolDevRun(ws, xtoolOpts)
		})
	}

	// Stream logs if requested
	if !xtoolOpts.noLogs {
		idevicesyslog, err := exec.LookPath("idevicesyslog")
		if err != nil {
			fmt.Println("Note: idevicesyslog not found, cannot stream logs")
			fmt.Println("Install libimobiledevice for log streaming support")
			return nil
		}

		fmt.Println("Streaming device logs (Ctrl+C to stop)...")
		fmt.Println()

		// Use --process to filter by app name (matches the executable name)
		logArgs := []string{"--process", cfg.AppName}
		if xtoolOpts.deviceID != "" {
			logArgs = append([]string{"-u", xtoolOpts.deviceID}, logArgs...)
		}

		cmd := exec.Command(idevicesyslog, logArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			// User likely hit Ctrl+C, which is fine
			return nil
		}
	}

	return nil
}

// xtoolDevRun installs and launches the app on a connected iOS device using
// the xtool dev run command.
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
