package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-drift/drift/cmd/drift/internal/config"
	"github.com/go-drift/drift/cmd/drift/internal/workspace"
)

func init() {
	RegisterCommand(&Command{
		Name:  "clean",
		Short: "Remove build artifacts",
		Long: `Remove all build artifacts from the project.

This command deletes:
  - Drift build cache for this project
  - Generated Android/iOS workspaces

Use this when you want to do a completely fresh build.`,
		Usage: "drift clean",
		Run:   runClean,
	})
}

func runClean(args []string) error {
	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Resolve(root)
	if err != nil {
		return err
	}

	fmt.Println("Cleaning build artifacts...")

	buildRoot, err := workspace.BuildRoot(cfg)
	if err != nil {
		return err
	}

	if _, err := os.Stat(buildRoot); err == nil {
		fmt.Printf("  Removing %s/\n", buildRoot)
		if err := os.RemoveAll(buildRoot); err != nil {
			fmt.Printf("  Warning: could not remove %s: %v\n", buildRoot, err)
		}
	}

	cacheDir := filepath.Join(root, "build")
	if _, err := os.Stat(cacheDir); err == nil {
		fmt.Printf("  Removing %s/\n", cacheDir)
		if err := os.RemoveAll(cacheDir); err != nil {
			fmt.Printf("  Warning: could not remove %s: %v\n", cacheDir, err)
		}
	}

	fmt.Println()
	fmt.Println("Clean complete!")

	return nil
}
