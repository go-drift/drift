package cmd

import (
	"fmt"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/config"
	"github.com/go-drift/drift/cmd/drift/internal/workspace"
)

func init() {
	RegisterCommand(&Command{
		Name:  "run",
		Short: "Build and run on a device or simulator",
		Long: `Build the application and run it on a connected device or simulator.

Supported platforms:
  android   Run on Android device or emulator
  ios       Run on iOS device or simulator (requires macOS)
  xtool     Run on iOS device using xtool (Linux/macOS, no Xcode required)

The command will:
  1. Build the application (debug mode)
  2. Install it on the target device
  3. Launch the application

Flags:
  --watch            Watch for file changes and rebuild automatically
  --no-logs          Launch without streaming logs
  --no-fetch         Disable auto-download of missing Skia libraries
  --device [UDID]    Run on a physical iOS device (optional: specify UDID)
  --simulator NAME   Run on a specific iOS simulator (default: iPhone 15)
  --team-id TEAM_ID  Apple Developer Team ID for code signing (required for --device)

For Android, you can specify a device with ADB:
  ANDROID_SERIAL=<device-id> drift run android

For iOS simulators:
  drift run ios --simulator "iPhone 15"

For physical iOS devices:
  drift run ios --device --team-id ABC123XYZ
  drift run ios --device 00008030-... --team-id ABC123XYZ

For xtool (Linux/macOS):
  drift run xtool                     Run on connected device
  drift run xtool --device UDID       Run on specific device

Note: Physical device deployment requires ios-deploy (brew install ios-deploy)
      or ideviceinstaller (part of libimobiledevice)`,
		Usage: "drift run <platform> [--watch] [--no-logs] [--no-fetch] [--device [UDID]] [--simulator NAME] [--team-id TEAM_ID]",
		Run:   runRun,
	})
}

type runOptions struct {
	noLogs  bool
	noFetch bool
	watch   bool
}

func runRun(args []string) error {
	platformArgs, opts := parseRunArgs(args)
	if len(platformArgs) == 0 {
		return fmt.Errorf("platform is required (android, ios, or xtool)\n\nUsage: drift run <platform> [--no-logs]")
	}

	platform := strings.ToLower(platformArgs[0])

	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Resolve(root)
	if err != nil {
		return err
	}

	ws, err := workspace.Prepare(root, cfg, platform)
	if err != nil {
		return err
	}

	switch platform {
	case "android":
		return runAndroid(ws, cfg, opts)
	case "ios":
		return runIOS(ws, cfg, platformArgs[1:], opts)
	case "xtool":
		return runXtool(ws, cfg, platformArgs[1:], opts)
	default:
		return fmt.Errorf("unknown platform %q (use android, ios, or xtool)", platform)
	}
}

func parseRunArgs(args []string) ([]string, runOptions) {
	opts := runOptions{}
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--no-logs":
			opts.noLogs = true
		case "--no-fetch":
			opts.noFetch = true
		case "--watch":
			opts.watch = true
		default:
			filtered = append(filtered, arg)
		}
	}
	return filtered, opts
}
