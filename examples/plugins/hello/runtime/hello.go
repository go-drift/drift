// Package runtime is the hello plugin's Go API. The plugin must be in the
// app's drift.yaml (github.com/go-drift/drift/examples/plugins/hello/plugin).
package runtime

import (
	"context"
	"fmt"

	"github.com/go-drift/drift/pkg/platform"
)

// channel matches the name the native halves register
// (plugin/ios/HelloPlugin.swift, plugin/android/HelloPlugin.kt).
var channel = platform.NewMethodChannel("example/hello")

// Greeting returns the greeting configured in drift.yaml. It waits for the
// native side: call it from a goroutine, not from a widget callback.
func Greeting(ctx context.Context) (string, error) {
	res, err := channel.Invoke(ctx, "greeting", nil)
	if err != nil {
		return "", fmt.Errorf("hello greeting: %w", err)
	}
	s, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("hello greeting: got %T, want string", res)
	}
	return s, nil
}
