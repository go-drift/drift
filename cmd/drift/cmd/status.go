package cmd

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/go-drift/drift/cmd/drift/internal/config"
	"github.com/go-drift/drift/cmd/drift/internal/workspace"
)

func init() {
	RegisterCommand(&Command{
		Name:  "status",
		Short: "Show project status",
		Long: `Show the current status of the Drift project.

Displays which platforms are ejected (user-managed) vs managed (generated
to ~/.drift/build/).

Ejected platforms build in ./platform/<platform>/ and preserve user changes.
Managed platforms regenerate all files on each build.`,
		Usage: "drift status",
		Run:   runStatus,
	})
}

func runStatus(args []string) error {
	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Resolve(root)
	if err != nil {
		return err
	}

	buildRoot, err := workspace.BuildRoot(cfg)
	if err != nil {
		return err
	}

	// Compute hash to match workspace.go path structure
	hash := sha1.Sum([]byte(root))
	shortHash := hex.EncodeToString(hash[:6])

	fmt.Printf("Project: %s (%s)\n", cfg.AppName, cfg.AppID)
	fmt.Println()
	fmt.Println("Platforms:")

	platforms := []string{"ios", "android", "xtool"}

	for _, p := range platforms {
		ejected := workspace.IsEjected(root, p)
		if ejected {
			ejectedDir := filepath.Join(root, "platform", p)
			fmt.Printf("  %-8s ejected  -> %s\n", p+":", ejectedDir)
		} else {
			managedDir := filepath.Join(buildRoot, p, shortHash)
			fmt.Printf("  %-8s managed  -> %s\n", p+":", managedDir)
		}
	}

	return nil
}
