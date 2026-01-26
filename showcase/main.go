package main

import (
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/engine"
)

func main() {
	app := drift.NewApp(App())
	app.Diagnostics = engine.DefaultDiagnosticsConfig()
	app.Run()
}
