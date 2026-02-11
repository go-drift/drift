package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-drift/drift/cmd/drift/internal/workspace"
)

// watchAndRun watches for Go file changes in the project and calls rebuild
// on each change. It debounces rapid successive events (e.g. editor
// save-all) into a single rebuild.
func watchAndRun(ctx context.Context, ws *workspace.Workspace, rebuild func() error) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	defer watcher.Close()

	if err := addWatchDirs(watcher, ws.Root); err != nil {
		return fmt.Errorf("failed to watch project directories: %w", err)
	}

	fmt.Println("Watching for changes... (Ctrl+C to stop)")
	fmt.Println()

	const debounceDelay = 500 * time.Millisecond
	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if !isRelevantChange(event) {
				continue
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(debounceDelay)
			timerC = timer.C

		case <-timerC:
			timerC = nil
			fmt.Println()
			fmt.Println("Rebuilding...")
			if err := rebuild(); err != nil {
				fmt.Fprintf(os.Stderr, "\nRebuild failed: %v\n", err)
			}
			fmt.Println()
			fmt.Println("Watching for changes...")

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "Watch error: %v\n", err)
		}
	}
}

// addWatchDirs recursively adds project directories to the watcher,
// skipping hidden dirs, vendor, platform scaffolds, and third_party.
func addWatchDirs(watcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible entries
		}
		if !info.IsDir() {
			return nil
		}
		name := info.Name()
		if name != "." && strings.HasPrefix(name, ".") {
			return filepath.SkipDir
		}
		switch name {
		case "vendor", "platform", "third_party":
			return filepath.SkipDir
		}
		return watcher.Add(path)
	})
}

// isRelevantChange returns true for write/create/remove/rename events on
// .go files or the drift config file.
func isRelevantChange(event fsnotify.Event) bool {
	if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return false
	}
	base := filepath.Base(event.Name)
	return strings.HasSuffix(base, ".go") || base == "drift.yaml" || base == "drift.yml"
}

// findADB locates the adb binary, checking ANDROID_SDK_ROOT and
// ANDROID_HOME before falling back to bare "adb" on PATH.
func findADB() string {
	if sdkRoot := os.Getenv("ANDROID_SDK_ROOT"); sdkRoot != "" {
		return filepath.Join(sdkRoot, "platform-tools", "adb")
	}
	if androidHome := os.Getenv("ANDROID_HOME"); androidHome != "" {
		return filepath.Join(androidHome, "platform-tools", "adb")
	}
	return "adb"
}

// streamAndroidLogs streams tag-filtered logcat output until ctx is
// cancelled. Tag-based filtering survives app restarts, unlike PID-based.
// Intended to run as a goroutine.
func streamAndroidLogs(ctx context.Context, appID string) {
	adb := findADB()

	// Clear stale logs so the stream starts fresh
	exec.Command(adb, "logcat", "-c").Run()

	cmd := exec.CommandContext(ctx, adb, "logcat", "-v", "time",
		"DriftJNI:*",
		"DriftAccessibility:*",
		"DriftDeepLink:*",
		"DriftRenderer:*",
		"DriftSurfaceView:*",
		"DriftBackground:*",
		"DriftPush:*",
		"DriftSkia:*",
		"PlatformChannel:*",
		"Go:*",
		"AndroidRuntime:E",
		"*:S",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run() // exits when ctx is cancelled
}

// streamIOSLogs streams os_log output filtered by subsystem (bundle ID)
// until ctx is cancelled. Subsystem filtering survives app restarts.
// Intended to run as a goroutine.
func streamIOSLogs(ctx context.Context, appID string) {
	predicate := fmt.Sprintf(`subsystem == "%s"`, appID)
	cmd := exec.CommandContext(ctx, "log", "stream",
		"--predicate", predicate,
		"--level", "debug",
		"--style", "compact",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run() // exits when ctx is cancelled
}

// watchContext creates a cancellable context that is cancelled on
// SIGINT or SIGTERM.
func watchContext() (context.Context, context.CancelFunc) {
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
