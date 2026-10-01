package mutate

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// ApplyGradleApplyPlugins inserts an `apply plugin: "<id>"` line for each
// op into app/build.gradle, placed immediately after the closing brace of
// the top-level android { } block. Plugins like Firebase's
// `com.google.gms.google-services` need the android block configured first.
//
// Idempotent: an op whose plugin id already appears in a live (uncommented)
// `apply plugin:` line anywhere in the file is skipped, so reruns on an
// ejected project converge and a line the user wrote (even inside a
// conditional) is respected.
func ApplyGradleApplyPlugins(appGradlePath string, ops []*protocol.OpAndroidGradleApplyPlugin) (string, bool, error) {
	if len(ops) == 0 {
		return appGradlePath, false, nil
	}
	changed, err := rewriteGradle(appGradlePath, func(src []byte) ([]byte, error) {
		present := applyPluginIDsPresent(src)
		var toInsert []string
		for _, op := range ops {
			if present[op.ID] {
				continue
			}
			present[op.ID] = true
			toInsert = append(toInsert, fmt.Sprintf("apply plugin: \"%s\"", op.ID))
		}
		if len(toInsert) == 0 {
			return src, nil
		}
		return insertAfterAndroidBlock(src, toInsert)
	})
	return appGradlePath, changed, err
}

// ApplyGradleProjectPlugins declares each op with a Version in the
// project-level build.gradle `plugins { }` block as
// `id "<id>" version "<version>" apply false`, putting the plugin on the
// build classpath. Ops without a Version are skipped (the plugin is expected
// to be on the classpath already).
//
// An id already declared at the same version is skipped. An id already
// declared at a different version is an error: Gradle loads one version
// per plugin id, and silently keeping the scaffold's version would hand the
// plugin a version it did not ask for.
func ApplyGradleProjectPlugins(projectGradlePath string, ops []*protocol.OpAndroidGradleApplyPlugin) (string, bool, error) {
	if len(ops) == 0 {
		return projectGradlePath, false, nil
	}
	changed, err := rewriteGradle(projectGradlePath, func(src []byte) ([]byte, error) {
		blk, err := findTopLevelBlock(src, "plugins")
		if err != nil {
			return nil, err
		}
		declared := projectPluginVersions(blk.body(src))
		var toInsert []string
		for _, op := range ops {
			if op.Version == "" {
				continue
			}
			if have, ok := declared[op.ID]; ok {
				if have != "" && have != op.Version {
					return nil, fmt.Errorf("plugin %q requests Gradle plugin %q version %s, but the project declares version %s",
						op.PluginPackage(), op.ID, op.Version, have)
				}
				continue
			}
			declared[op.ID] = op.Version
			toInsert = append(toInsert, fmt.Sprintf("    id \"%s\" version \"%s\" apply false", op.ID, op.Version))
		}
		return insertBeforeClose(src, blk.closeIdx, toInsert), nil
	})
	return projectGradlePath, changed, err
}

// applyPluginIDRE matches `apply plugin: "<id>"` and `apply plugin: '<id>'`.
var applyPluginIDRE = regexp.MustCompile(`apply\s+plugin\s*:\s*["']([^"']+)["']`)

// projectPluginRE matches `id "<id>"` with an optional `version "<v>"`,
// tolerating both quote styles.
var projectPluginRE = regexp.MustCompile(`id\s+["']([^"']+)["'](?:\s+version\s+["']([^"']+)["'])?`)

// applyPluginIDsPresent returns the set of plugin ids with a live
// `apply plugin:` line in src. Line comments are stripped first so a
// commented-out occurrence (full-line or trailing) never suppresses
// insertion.
func applyPluginIDsPresent(src []byte) map[string]bool {
	present := make(map[string]bool)
	for line := range strings.SplitSeq(string(src), "\n") {
		for _, m := range applyPluginIDRE.FindAllStringSubmatch(stripLineComment(line), -1) {
			present[m[1]] = true
		}
	}
	return present
}

// projectPluginVersions maps each plugin id declared in a plugins { } block
// body to its declared version ("" when the declaration has no version).
func projectPluginVersions(body string) map[string]string {
	declared := make(map[string]string)
	for line := range strings.SplitSeq(body, "\n") {
		for _, m := range projectPluginRE.FindAllStringSubmatch(stripLineComment(line), -1) {
			declared[m[1]] = m[2]
		}
	}
	return declared
}

// stripLineComment drops a trailing `//` comment. Pragmatic: it does not
// understand `//` inside string literals, which the lines scanned here
// (plugin declarations) never contain.
func stripLineComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		return line[:i]
	}
	return line
}

// insertAfterAndroidBlock returns src with lines inserted after the closing
// `}` of the top-level android block, separated from it by a blank line.
func insertAfterAndroidBlock(src []byte, lines []string) ([]byte, error) {
	blk, err := findTopLevelBlock(src, "android")
	if err != nil {
		return nil, err
	}
	insertPoint := blk.closeIdx + 1
	var b bytes.Buffer
	b.Write(src[:insertPoint])
	if insertPoint >= len(src) || src[insertPoint] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	for _, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.Write(src[insertPoint:])
	return b.Bytes(), nil
}
