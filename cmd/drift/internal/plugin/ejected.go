package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// iosCallbackCallSite names the template file that must call the generated
// DriftPluginRegistrant method for a callback. The method name is the
// callback's string value. URL and user-activity events arrive at the scene,
// not the app delegate, so their call sites live in SceneDelegate.swift.
func iosCallbackCallSite(cb driftplugin.IOSAppDelegateCallback) (file, call string) {
	call = "DriftPluginRegistrant." + string(cb) + "("
	switch cb {
	case driftplugin.IOSCallbackOpenURL, driftplugin.IOSCallbackContinueUserActivity:
		return "SceneDelegate.swift", call
	default:
		return "AppDelegate.swift", call
	}
}

// CheckEjectedIOS verifies that an ejected xcodeproj project is wired for
// the plugin features the ops use. Ejected sources were copied from the
// templates at eject time and are never regenerated, so a project ejected
// before a feature existed would otherwise build fine and silently drop the
// plugin's integration. Each problem is reported with the fix to apply by
// hand; auto-patching user-edited Swift and pbxproj files is too fragile.
func CheckEjectedIOS(projectDir string, ops []driftplugin.Op) error {
	var problems []string

	needed := make(map[driftplugin.IOSAppDelegateCallback][]string)
	needsSPM := false
	for _, op := range ops {
		switch v := op.(type) {
		case *driftplugin.OpIOSAppDelegateRegistrant:
			needed[v.Callback] = append(needed[v.Callback], v.PluginPackage())
		case *driftplugin.OpIOSAddPackageDependency:
			needsSPM = true
		}
	}

	sources := make(map[string]string)
	readSource := func(name string) string {
		if src, ok := sources[name]; ok {
			return src
		}
		data, _ := os.ReadFile(filepath.Join(projectDir, "Runner", name))
		sources[name] = string(data)
		return sources[name]
	}
	for _, cb := range driftplugin.IOSAppDelegateCallbacks {
		plugins, ok := needed[cb]
		if !ok {
			continue
		}
		file, call := iosCallbackCallSite(cb)
		if !strings.Contains(readSource(file), call) {
			problems = append(problems, fmt.Sprintf(
				"Runner/%s must call %s...) from its %s handler (needed by %s); see the Drift iOS template for the exact call",
				file, call, cb, strings.Join(uniqueSorted(plugins), ", ")))
		}
	}

	if needsSPM {
		pbxproj := filepath.Join(projectDir, "Runner.xcodeproj", "project.pbxproj")
		data, _ := os.ReadFile(pbxproj)
		if !strings.Contains(string(data), `relativePath = "Drift/Plugins"`) {
			problems = append(problems,
				`Runner.xcodeproj must reference the local Swift package at Drift/Plugins and link its "DriftPlugins" product `+
					`(Xcode: File > Add Package Dependencies > Add Local..., select Drift/Plugins)`)
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("ejected iOS project at %s is missing plugin wiring:\n  - %s", projectDir, strings.Join(problems, "\n  - "))
}

func uniqueSorted(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
