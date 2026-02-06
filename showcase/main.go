package main

import (
	"time"

	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/engine"
)

func main() {
	app := drift.NewApp(App())
	// app.Diagnostics = &engine.DiagnosticsConfig{
	// 	ShowFPS:          false,
	// 	ShowFrameGraph:   false,
	// 	TargetFrameTime:  16667 * time.Microsecond, // ~16.67ms for 60fps
	// 	ShowLayoutBounds: false,
	// 	DebugServerPort:  9999,
	// }
	app.Run()
}
