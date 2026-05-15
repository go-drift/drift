package mutate

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
)

// errKotlinDSL is returned when the project ships a Kotlin DSL build file
// (app/build.gradle.kts) instead of a Groovy DSL one. v1 supports Groovy
// only; silent no-op would leak as a missing dependency at runtime, so the
// failure is loud and actionable.
var errKotlinDSL = fmt.Errorf(
	"plugin requires Groovy DSL build.gradle (found build.gradle.kts). " +
		"Convert to Groovy or open an issue for Kotlin DSL support.")

// ApplyGradleAddDependencies inserts each requested dependency line into the
// Groovy DSL `dependencies { ... }` block of app/build.gradle. Already-
// present byte-identical lines are skipped (no-op). Returns (path, changed,
// err).
//
// Idempotent: re-runs of `drift build` against an unchanged project produce
// no Gradle file modifications. This is what makes the plugin pipeline cheap
// to re-run in watch mode without invalidating Gradle's incremental cache.
func ApplyGradleAddDependencies(gradlePath string, ops []*driftplugin.OpAndroidGradleAddDependency) (string, bool, error) {
	if len(ops) == 0 {
		return gradlePath, false, nil
	}
	// .kts variant is not supported in v1; check sibling path explicitly so
	// the error names the actual situation (Groovy expected, .kts found).
	if strings.HasSuffix(gradlePath, ".gradle") {
		if _, err := os.Stat(gradlePath + ".kts"); err == nil {
			return gradlePath, false, errKotlinDSL
		}
	}

	data, err := os.ReadFile(gradlePath)
	if err != nil {
		return gradlePath, false, fmt.Errorf("read %s: %w", gradlePath, err)
	}

	// Build the set of lines to insert. Format mirrors the existing
	// scaffold (one space of indent inside the block, double-quoted coord).
	desired := make([]string, 0, len(ops))
	seen := make(map[string]bool, len(ops))
	for _, op := range ops {
		line := fmt.Sprintf("    %s \"%s\"", op.Configuration, op.Coord)
		if seen[line] {
			continue
		}
		seen[line] = true
		desired = append(desired, line)
	}

	updated, err := insertIntoDependenciesBlock(data, desired)
	if err != nil {
		return gradlePath, false, err
	}
	if bytes.Equal(updated, data) {
		return gradlePath, false, nil
	}
	if err := os.WriteFile(gradlePath, updated, 0o644); err != nil {
		return gradlePath, false, fmt.Errorf("write %s: %w", gradlePath, err)
	}
	return gradlePath, true, nil
}

// dependenciesBlockRE matches the top-level `dependencies {` opener at the
// start of a line, anchored to make sure we don't match nested blocks
// (e.g. an Android-DSL `dependencies` inside another scope). Pragmatic: the
// scaffold template always uses a top-level block at column 0.
var dependenciesBlockRE = regexp.MustCompile(`(?m)^dependencies\s*\{`)

// insertIntoDependenciesBlock inserts new lines at the end of the top-level
// dependencies { ... } block, skipping any that are already byte-identical
// somewhere inside the block. Returns the updated source.
func insertIntoDependenciesBlock(src []byte, lines []string) ([]byte, error) {
	loc := dependenciesBlockRE.FindIndex(src)
	if loc == nil {
		return nil, fmt.Errorf("could not locate `dependencies {` block in build.gradle")
	}
	// Find the matching closing brace by scanning forward.
	start := loc[1] // just past the `{`
	depth := 1
	closeIdx := -1
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				closeIdx = i
			}
		}
		if closeIdx != -1 {
			break
		}
	}
	if closeIdx == -1 {
		return nil, fmt.Errorf("could not locate closing `}` of dependencies block")
	}

	block := string(src[start:closeIdx])
	// Skip any line that is already byte-identical (trimmed) inside the
	// existing block. Allows safe re-application without duplicate entries.
	existing := make(map[string]bool)
	for l := range strings.SplitSeq(block, "\n") {
		existing[strings.TrimSpace(l)] = true
	}

	var toInsert []string
	for _, line := range lines {
		if existing[strings.TrimSpace(line)] {
			continue
		}
		toInsert = append(toInsert, line)
	}
	if len(toInsert) == 0 {
		return src, nil
	}

	// Insert immediately before the closing `}`. Ensure the block ends with
	// a newline before the inserted lines, and that the inserted lines end
	// with a newline so the closing brace stays on its own line.
	prefix := src[:closeIdx]
	suffix := src[closeIdx:]
	var b bytes.Buffer
	b.Write(prefix)
	if len(prefix) > 0 && prefix[len(prefix)-1] != '\n' {
		b.WriteByte('\n')
	}
	for _, line := range toInsert {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.Write(suffix)
	return b.Bytes(), nil
}
