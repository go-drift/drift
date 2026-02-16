// Package cmd implements the Drift CLI commands.
//
// The command structure follows standard Go CLI patterns with a root command
// that dispatches to subcommands (build, run, clean, devices, log).
package cmd

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/cache"
)

// Version information set at build time or read from embedded build info.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func init() {
	// Read version from Go's embedded build info (set by go install @version)
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
		return
	}
	var revision string
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			revision = setting.Value
			break
		}
	}
	if revision != "" {
		if len(revision) > 12 {
			revision = revision[:12]
		}
		Version = "dev+" + revision
	}
}

// Command represents a CLI command.
type Command struct {
	Name        string
	Short       string
	Long        string
	Usage       string
	Run         func(args []string) error
	SubCommands []*Command
}

var rootCmd = &Command{
	Name:  "drift",
	Short: "Drift - modern mobile UI architecture, but Go",
	Long: `Drift is a cross-platform mobile UI framework that brings proven
architectural patterns to Go developers. Write your app logic
and UI in Go, deploy to iOS and Android with native performance.

Use "drift <command> --help" for more information about a command.`,
	Usage: "drift <command> [flags]",
}

// Commands registered with the CLI.
var commands = make(map[string]*Command)

// RegisterCommand adds a command to the CLI.
func RegisterCommand(cmd *Command) {
	commands[cmd.Name] = cmd
	rootCmd.SubCommands = append(rootCmd.SubCommands, cmd)
}

// Execute runs the CLI with the given arguments.
func Execute() error {
	cache.SetGlobal(Version)

	args := os.Args[1:]

	// Handle no arguments
	if len(args) == 0 {
		printHelp(rootCmd)
		return nil
	}

	// Handle global flags and extract --cache-dir
	var filteredArgs []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help", "help":
			if len(filteredArgs) == 0 {
				printHelp(rootCmd)
				return nil
			}
			filteredArgs = append(filteredArgs, arg)
		case "-v", "--version", "version":
			if len(filteredArgs) == 0 {
				fmt.Printf("Drift CLI version %s (built %s)\n", Version, BuildTime)
				return nil
			}
			filteredArgs = append(filteredArgs, arg)
		case "--cache-dir":
			if i+1 < len(args) {
				cache.SetCacheDir(args[i+1])
				i++
			} else {
				return fmt.Errorf("--cache-dir requires a directory path")
			}
		default:
			if after, ok := strings.CutPrefix(arg, "--cache-dir="); ok {
				cache.SetCacheDir(after)
				continue
			}
			filteredArgs = append(filteredArgs, arg)
		}
	}
	args = filteredArgs

	if len(args) == 0 {
		printHelp(rootCmd)
		return nil
	}

	// Find and execute the command
	cmdName := args[0]
	cmd, ok := commands[cmdName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: unknown command %q\n\n", cmdName)
		printHelp(rootCmd)
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	// Check for help flag on subcommand
	cmdArgs := args[1:]
	for _, arg := range cmdArgs {
		if arg == "-h" || arg == "--help" || arg == "help" {
			printCommandHelp(cmd)
			return nil
		}
	}

	return cmd.Run(cmdArgs)
}

func printHelp(cmd *Command) {
	fmt.Println(cmd.Long)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s\n", cmd.Usage)
	fmt.Println()
	fmt.Println("Commands:")
	for _, sub := range cmd.SubCommands {
		fmt.Printf("  %-14s %s\n", sub.Name, sub.Short)
	}
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -h, --help           Show help for a command")
	fmt.Println("  -v, --version        Show version information")
	fmt.Println("  --cache-dir DIR      Override cache directory (default: ~/.drift)")
	fmt.Println()
	fmt.Println("Environment:")
	fmt.Println("  DRIFT_CACHE_DIR      Cache directory override (lower priority than --cache-dir)")
	fmt.Println("  DRIFT_SKIA_DIR       Skia library base directory override (expects {platform}/{arch} structure)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  drift build android       Build for Android")
	fmt.Println("  drift run ios             Build and run on iOS simulator")
	fmt.Println("  drift clean               Remove generated build artifacts")
}

func printCommandHelp(cmd *Command) {
	fmt.Println(cmd.Long)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s\n", cmd.Usage)
}
