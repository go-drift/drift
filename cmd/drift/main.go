// Package main is the entry point for the Drift CLI tool.
//
// The Drift CLI provides commands for building and running Drift
// mobile applications. It manages the build toolchain for both
// iOS and Android platforms.
//
// Usage:
//
//	drift build <platform>  Build for iOS or Android
//	drift run <platform>    Build and run on a device/simulator
package main

import (
	"os"

	"github.com/go-drift/drift/cmd/drift/cmd"
	"github.com/go-drift/drift/cmd/drift/internal/errors"
)

func main() {
	defer errors.RecoverPanic()
	if err := cmd.Execute(); err != nil {
		errors.PrintError(err)
		os.Exit(1)
	}
}
