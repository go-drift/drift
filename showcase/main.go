package main

import (
	"github.com/go-drift/drift/pkg/drift"
)

func main() {
	app := drift.NewApp(App())
	// diagnostics := engine.DefaultDiagnosticsConfig()
	// diagnostics.ShowLayoutBounds = false
	// diagnostics.DebugServerPort = 9999 // Enable debug server: curl localhost:9999/tree | jq .
	// app.Diagnostics = diagnostics
	app.Run()
}
