package main

import (
	"context"
	"fmt"

	"github.com/go-drift/drift/pkg/drift"
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
	app.OnInit = func(ctx context.Context) error {
		fmt.Println("App OnInit")
		return nil
	}
	app.OnDispose = func() {
		fmt.Println("App OnDispose")
	}
	app.Run()
}
