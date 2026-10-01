package plugin

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// wiringCall is a call the platform templates make so plugins work: the
// launch, event and view hooks that feed DriftPlugins. Ejected projects
// copied the templates once, at eject time, and are never regenerated, so
// the checks below verify each call is still there rather than letting a
// stale project build and silently drop plugin integration. Each problem is
// reported with the fix to apply by hand; auto-patching user-edited Swift,
// Kotlin and pbxproj files is too fragile.
type wiringCall struct {
	// File is the path relative to the ejected project root. For Android,
	// "MainActivity.kt" is looked up under app/src/main/java.
	File string
	// Call is text that must appear in File.
	Call string
	// Why completes "... so that ..." in the error message.
	Why string
}

var iosWiring = []wiringCall{
	{"Runner/AppDelegate.swift", "DriftPlugins.shared.launch(", "plugins are created and registered at launch"},
	{"Runner/AppDelegate.swift", "DriftPlugins.shared.didRegisterForRemoteNotifications(", "plugins receive the APNs token"},
	{"Runner/AppDelegate.swift", "DriftPlugins.shared.didFailToRegisterForRemoteNotifications(", "plugins see APNs registration failures"},
	{"Runner/AppDelegate.swift", "DriftPlugins.shared.didReceiveRemoteNotification(", "plugins receive remote notifications"},
	{"Runner/DriftViewController.swift", "DriftPlugins.shared.attach(self, overlayView:", "plugins attach to the Drift view"},
	{"Runner/DriftViewController.swift", "DriftPlugins.shared.detach(", "plugins detach from the Drift view"},
	{"Runner.xcodeproj/project.pbxproj", "PBXFileSystemSynchronizedRootGroup", "Xcode compiles the plugin sources Drift writes under Runner/ (project format 77, Xcode 16+)"},
}

var androidWiring = []wiringCall{
	{"MainActivity.kt", "DriftPlugins.register(", "plugins are created and registered once per process"},
	{"MainActivity.kt", "DriftPlugins.attach(", "plugins attach to the Activity"},
	{"MainActivity.kt", "DriftPlugins.detach(", "plugins detach when the Activity is destroyed"},
	{"MainActivity.kt", "DriftPlugins.onNewIntent(", "plugins receive new intents"},
}

// CheckEjectedIOS verifies that an ejected xcodeproj project is wired for
// plugins (see wiringCall), links the local Drift/Plugins package, and, when
// a plugin edits entitlements, signs with Runner.entitlements.
func CheckEjectedIOS(projectDir string, ops []protocol.Op) error {
	problems := checkWiring(projectDir, iosWiring, func(name string) string {
		return filepath.Join(projectDir, filepath.FromSlash(name))
	})
	data, _ := os.ReadFile(filepath.Join(projectDir, "Runner.xcodeproj", "project.pbxproj"))
	pbxproj := string(data)
	if !strings.Contains(pbxproj, `relativePath = "Drift/Plugins"`) {
		problems = append(problems,
			`Runner.xcodeproj must reference the local Swift package at Drift/Plugins and link its "DriftPlugins" product, `+
				`which holds the plugin API the app imports (Xcode: File > Add Package Dependencies > Add Local..., select Drift/Plugins)`)
	}
	editsEntitlements := slices.ContainsFunc(ops, func(op protocol.Op) bool {
		p, ok := op.(protocol.PlistOp)
		return ok && p.PlistFile() == protocol.PlistEntitlements
	})
	if editsEntitlements {
		entitlements := iosPlistRelPath("ios", protocol.PlistEntitlements)
		if _, err := os.Stat(filepath.Join(projectDir, entitlements)); err != nil {
			problems = append(problems, fmt.Sprintf(
				"%s is required by plugins that add entitlements: create it as an empty plist (<dict/>) next to Runner.xcodeproj", entitlements))
		}
		if !strings.Contains(pbxproj, "CODE_SIGN_ENTITLEMENTS = "+entitlements+";") {
			problems = append(problems, fmt.Sprintf(
				"the Runner target must set CODE_SIGN_ENTITLEMENTS = %s in every configuration, so the app signs with the entitlements plugins add "+
					"(Xcode: Runner target > Build Settings > Code Signing Entitlements)", entitlements))
		}
	}
	return wiringError("iOS", projectDir, problems)
}

// CheckEjectedAndroid verifies that an ejected Android project is wired
// for plugins (see wiringCall), and uses the Groovy app/build.gradle the
// Gradle mutators edit when a plugin needs Gradle changes.
func CheckEjectedAndroid(projectDir string, ops []protocol.Op) error {
	mainActivity, err := findMainActivity(projectDir)
	if err != nil {
		return err
	}
	problems := checkWiring(projectDir, androidWiring, func(string) string { return mainActivity })
	editsGradle := slices.ContainsFunc(ops, func(op protocol.Op) bool {
		switch op.(type) {
		case *protocol.OpAndroidGradleAddDependency, *protocol.OpAndroidGradleApplyPlugin:
			return true
		}
		return false
	})
	if editsGradle {
		if _, err := os.Stat(filepath.Join(projectDir, "app", "build.gradle")); err != nil {
			problems = append(problems, "app/build.gradle (Groovy DSL) is required by plugins that add Gradle dependencies or plugins; Kotlin DSL build files are not supported")
		}
	}
	return wiringError("Android", projectDir, problems)
}

func checkWiring(projectDir string, calls []wiringCall, resolve func(name string) string) []string {
	var problems []string
	sources := map[string]string{}
	for _, c := range calls {
		path := resolve(c.File)
		src, ok := sources[path]
		if !ok {
			data, _ := os.ReadFile(path)
			src = string(data)
			sources[path] = src
		}
		if !strings.Contains(src, c.Call) {
			rel, err := filepath.Rel(projectDir, path)
			if err != nil {
				rel = path
			}
			problems = append(problems, fmt.Sprintf("%s must contain %q so that %s; see the Drift template for the exact code", filepath.ToSlash(rel), c.Call, c.Why))
		}
	}
	return problems
}

func wiringError(platform, projectDir string, problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("ejected %s project at %s is missing plugin wiring:\n  - %s", platform, projectDir, strings.Join(problems, "\n  - "))
}

// findMainActivity returns the path of the ejected project's
// MainActivity.kt, which lives under the app's package directory.
func findMainActivity(projectDir string) (string, error) {
	root := filepath.Join(projectDir, "app", "src", "main", "java")
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "MainActivity.kt" {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("find MainActivity.kt: %w", err)
	}
	if len(found) != 1 {
		return "", fmt.Errorf("ejected Android project at %s: expected one MainActivity.kt under app/src/main/java, found %d", projectDir, len(found))
	}
	return found[0], nil
}
