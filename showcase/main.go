package main

import "github.com/go-drift/drift/pkg/drift"

func main() {
	drift.NewApp(App()).Run()
}
