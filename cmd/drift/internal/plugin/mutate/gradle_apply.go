package mutate

import (
	"bytes"
	"fmt"
	"regexp"

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
func ApplyGradleApplyPlugins(cs *Changeset, appGradlePath string, ops []*protocol.OpAndroidGradleApplyPlugin) (string, bool, error) {
	if len(ops) == 0 {
		return appGradlePath, false, nil
	}
	changed, err := rewriteGradle(cs, appGradlePath, func(src []byte) ([]byte, error) {
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
//
// replace maps a declaration to the one Drift wrote for the same id at an
// earlier version (an ejected project's lock): if the project still holds
// that line as written, it is updated in place rather than reported as a
// mismatch.
func ApplyGradleProjectPlugins(cs *Changeset, projectGradlePath string, ops []*protocol.OpAndroidGradleApplyPlugin, replace map[string]string) (string, bool, error) {
	if len(ops) == 0 {
		return projectGradlePath, false, nil
	}
	changed, err := rewriteGradle(cs, projectGradlePath, func(src []byte) ([]byte, error) {
		blk, err := findTopLevelBlock(src, "plugins")
		if err != nil {
			return nil, err
		}
		var wanted []string
		for _, op := range ops {
			if op.Version != "" {
				wanted = append(wanted, GradlePluginDeclaration(op))
			}
		}
		src = replaceBlockLines(src, blk, staleLines(wanted, replace))
		if blk, err = findTopLevelBlock(src, "plugins"); err != nil {
			return nil, err
		}
		declared := projectPluginVersions(blk.body(maskGroovy(src, false)))
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
			toInsert = append(toInsert, GradlePluginDeclaration(op))
		}
		return insertBeforeClose(src, blk.closeIdx, toInsert), nil
	})
	return projectGradlePath, changed, err
}

// GradlePluginDeclaration is the line ApplyGradleProjectPlugins writes for
// op, which has a Version.
func GradlePluginDeclaration(op *protocol.OpAndroidGradleApplyPlugin) string {
	return fmt.Sprintf("    id \"%s\" version \"%s\" apply false", op.ID, op.Version)
}

// applyPluginIDRE matches `apply plugin: "<id>"` and `apply plugin: '<id>'`.
var applyPluginIDRE = regexp.MustCompile(`apply\s+plugin\s*:\s*["']([^"']+)["']`)

// projectPluginRE matches `id "<id>"` with an optional `version "<v>"`,
// tolerating both quote styles.
var projectPluginRE = regexp.MustCompile(`id\s+["']([^"']+)["'](?:\s+version\s+["']([^"']+)["'])?`)

// applyPluginIDsPresent returns the set of plugin ids with a live
// `apply plugin:` line in src. Comments are masked first so a
// commented-out occurrence never suppresses insertion.
func applyPluginIDsPresent(src []byte) map[string]bool {
	present := make(map[string]bool)
	for _, m := range applyPluginIDRE.FindAllSubmatch(maskGroovy(src, false), -1) {
		present[string(m[1])] = true
	}
	return present
}

// projectPluginVersions maps each plugin id declared in a plugins { } block
// body, comments masked, to its declared version ("" when the declaration
// has no version).
func projectPluginVersions(body string) map[string]string {
	declared := make(map[string]string)
	for _, m := range projectPluginRE.FindAllStringSubmatch(body, -1) {
		declared[m[1]] = m[2]
	}
	return declared
}

// insertAfterAndroidBlock returns src with lines inserted after the closing
// `}` of the top-level android block, separated from it by a blank line.
func insertAfterAndroidBlock(src []byte, lines []string) ([]byte, error) {
	blk, err := findTopLevelBlock(src, "android")
	if err != nil {
		return nil, err
	}
	rest := blk.closeIdx + 1
	if rest < len(src) && src[rest] == '\n' {
		rest++
	}
	var b bytes.Buffer
	b.Write(src[:blk.closeIdx+1])
	b.WriteString("\n\n")
	for _, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.Write(src[rest:])
	return b.Bytes(), nil
}
