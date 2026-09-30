// Firebase iOS spike app. Renders one line of text; the point is the build.
package main

import (
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/widgets"
)

func main() {
	drift.App{Root: widgets.Centered(widgets.Text{Content: "firebase spike"})}.Run()
}
