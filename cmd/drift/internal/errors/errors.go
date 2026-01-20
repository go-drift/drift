// Package errors provides error formatting and display for the Drift CLI.
package errors

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// Colors for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

// isTTY returns true if stdout is a terminal.
func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// color returns the color code if stdout is a terminal, empty string otherwise.
func color(c string) string {
	if isTTY() {
		return c
	}
	return ""
}

// FormatError formats an error for display.
func FormatError(err error) string {
	if err == nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(color(colorRed))
	sb.WriteString("Error: ")
	sb.WriteString(color(colorReset))
	sb.WriteString(err.Error())
	sb.WriteString("\n")

	return sb.String()
}

// PrintError prints an error to stderr.
func PrintError(err error) {
	fmt.Fprint(os.Stderr, FormatError(err))
}

// PrintErrorf prints a formatted error to stderr.
func PrintErrorf(format string, args ...any) {
	PrintError(fmt.Errorf(format, args...))
}

// Warning prints a warning message.
func Warning(msg string) {
	fmt.Fprintf(os.Stderr, "%sWarning:%s %s\n", color(colorYellow), color(colorReset), msg)
}

// Warningf prints a formatted warning message.
func Warningf(format string, args ...any) {
	Warning(fmt.Sprintf(format, args...))
}

// BuildError represents a build error with context.
type BuildError struct {
	Phase   string // e.g., "Go compilation", "Gradle build"
	Command string // The command that failed
	Output  string // Command output
	Err     error  // Underlying error
}

func (e *BuildError) Error() string {
	return fmt.Sprintf("%s failed: %v", e.Phase, e.Err)
}

// FormatBuildError formats a build error with context.
func FormatBuildError(err *BuildError) string {
	var sb strings.Builder

	sb.WriteString(color(colorRed))
	sb.WriteString("Build Failed: ")
	sb.WriteString(color(colorReset))
	sb.WriteString(err.Phase)
	sb.WriteString("\n\n")

	if err.Command != "" {
		sb.WriteString(color(colorGray))
		sb.WriteString("Command: ")
		sb.WriteString(color(colorReset))
		sb.WriteString(err.Command)
		sb.WriteString("\n\n")
	}

	if err.Output != "" {
		sb.WriteString(color(colorCyan))
		sb.WriteString("Output:\n")
		sb.WriteString(color(colorReset))
		// Indent output
		lines := strings.Split(err.Output, "\n")
		for _, line := range lines {
			sb.WriteString("  ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if err.Err != nil {
		sb.WriteString(color(colorRed))
		sb.WriteString("Error: ")
		sb.WriteString(color(colorReset))
		sb.WriteString(err.Err.Error())
		sb.WriteString("\n")
	}

	return sb.String()
}

// RecoverPanic handles panics in the CLI.
func RecoverPanic() {
	if r := recover(); r != nil {
		// Get stack trace
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		stack := string(buf[:n])

		fmt.Fprintf(os.Stderr, "\n%sPanic:%s %v\n\n", color(colorRed), color(colorReset), r)
		fmt.Fprintf(os.Stderr, "%sStack trace:%s\n", color(colorGray), color(colorReset))

		// Format stack trace
		lines := strings.Split(stack, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "goroutine") {
				continue // Skip goroutine header
			}
			fmt.Fprintf(os.Stderr, "  %s\n", line)
		}

		fmt.Fprintf(os.Stderr, "\n%sThis is a bug in the Drift CLI.%s\n", color(colorYellow), color(colorReset))
		fmt.Fprintf(os.Stderr, "Please report this at: https://github.com/go-drift/drift/issues\n\n")

		os.Exit(1)
	}
}

// Suggestion provides a suggestion message.
type Suggestion struct {
	Title   string
	Details string
}

// FormatWithSuggestion formats an error with a suggestion.
func FormatWithSuggestion(err error, suggestion Suggestion) string {
	var sb strings.Builder

	sb.WriteString(FormatError(err))
	sb.WriteString("\n")
	sb.WriteString(color(colorCyan))
	sb.WriteString("Suggestion: ")
	sb.WriteString(color(colorReset))
	sb.WriteString(suggestion.Title)
	sb.WriteString("\n")

	if suggestion.Details != "" {
		lines := strings.Split(suggestion.Details, "\n")
		for _, line := range lines {
			sb.WriteString("  ")
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// CommonSuggestions provides suggestions for common errors.
var CommonSuggestions = map[string]Suggestion{
	"NDK not found": {
		Title: "Set up Android NDK",
		Details: `1. Install Android NDK via Android Studio's SDK Manager
2. Set ANDROID_NDK_HOME environment variable:
   export ANDROID_NDK_HOME=$ANDROID_HOME/ndk/<version>`,
	},
	"no drift.yaml": {
		Title: "Initialize a Drift project",
		Details: `1. Create a Go module (go mod init <module>)
2. Add a main.go that calls drift.Run(App())
3. Run drift from within the module root.`,
	},

	"gradle failed": {
		Title: "Check Gradle setup",
		Details: `1. Ensure Java 17 or later is installed
2. Set JAVA_HOME to your JDK installation
3. Try running './gradlew clean' in the android directory`,
	},
	"xcode not found": {
		Title: "Install Xcode",
		Details: `1. Install Xcode from the Mac App Store
2. Run: xcode-select --install
3. Accept the license: sudo xcodebuild -license`,
	},
}
