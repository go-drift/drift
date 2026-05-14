package cmd

import (
	"fmt"
	"os"

	"github.com/go-drift/drift/cmd/drift/internal/config"
	driftpluginCLI "github.com/go-drift/drift/cmd/drift/internal/plugin"
)

func init() {
	RegisterCommand(&Command{
		Name:  "plugin",
		Short: "Manage Drift plugins",
		Long: `Drift plugin subcommands.

  sync   Regenerate the plugin bridge, validate drift.yaml against schemas,
         and optionally run go mod tidy.
  list   Print installed plugins, versions, and status.`,
		Usage: "drift plugin <subcommand> [flags]",
		Run:   runPlugin,
	})
}

func runPlugin(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("plugin subcommand is required (sync or list)\n\nUsage: drift plugin <subcommand>")
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "sync":
		return runPluginSync(rest)
	case "list":
		return runPluginList(rest)
	default:
		return fmt.Errorf("unknown plugin subcommand %q (use sync or list)", sub)
	}
}

func runPluginSync(args []string) error {
	opts := driftpluginCLI.SyncOptions{CLIVersion: Version}
	for _, a := range args {
		switch a {
		case "--tidy":
			opts.Tidy = true
		default:
			return fmt.Errorf("unknown flag %q (only --tidy is supported)", a)
		}
	}

	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}
	opts.ProjectRoot = root

	res, err := driftpluginCLI.Sync(opts)
	if err != nil {
		return err
	}
	if res == nil {
		return nil
	}
	for _, d := range res.Diagnostics {
		fmt.Fprintln(os.Stderr, d.String())
	}
	if len(res.Plugins) == 0 {
		fmt.Println("No plugins configured in drift.yaml.")
		return nil
	}
	if len(res.Diagnostics) > 0 {
		// CI relies on a nonzero exit to gate merges; a successful Sync with
		// validation errors would let invalid drift.yaml ship.
		return fmt.Errorf("plugin schema validation failed: %d diagnostic(s)", len(res.Diagnostics))
	}
	fmt.Printf("Synced %d plugin(s).\n", len(res.Plugins))
	return nil
}

func runPluginList(args []string) error {
	resolve := false
	jsonOut := false
	for _, a := range args {
		switch {
		case a == "--resolve":
			resolve = true
		case a == "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q (use --resolve or --json)", a)
		}
	}

	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}

	res, err := driftpluginCLI.List(root, Version, resolve)
	if err != nil {
		return err
	}

	if len(res.Entries) == 0 {
		if jsonOut {
			fmt.Println("[]")
		} else {
			fmt.Println("No plugins configured in drift.yaml.")
		}
		return nil
	}

	if jsonOut {
		data, err := driftpluginCLI.FormatListJSON(res)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	out := driftpluginCLI.FormatListTable(res, resolve)
	fmt.Print(out)
	return nil
}
