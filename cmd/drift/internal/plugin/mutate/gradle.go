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
// (build.gradle.kts) instead of a Groovy DSL one. v1 supports Groovy only;
// silent no-op would leak as a missing dependency or plugin at runtime, so
// the failure is loud and actionable.
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

	// Build the set of lines to insert. Format mirrors the existing
	// scaffold (four spaces of indent inside the block, double-quoted coord).
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

	changed, err := rewriteGradle(gradlePath, func(src []byte) ([]byte, error) {
		return insertIntoDependenciesBlock(src, desired)
	})
	return gradlePath, changed, err
}

// insertIntoDependenciesBlock inserts new lines at the end of the top-level
// dependencies { ... } block, skipping any that are already byte-identical
// (trimmed) inside the block. Returns the updated source.
func insertIntoDependenciesBlock(src []byte, lines []string) ([]byte, error) {
	blk, err := findTopLevelBlock(src, "dependencies")
	if err != nil {
		return nil, err
	}
	existing := make(map[string]bool)
	for l := range strings.SplitSeq(blk.body(src), "\n") {
		existing[strings.TrimSpace(l)] = true
	}
	var toInsert []string
	for _, line := range lines {
		if existing[strings.TrimSpace(line)] {
			continue
		}
		toInsert = append(toInsert, line)
	}
	return insertBeforeClose(src, blk.closeIdx, toInsert), nil
}

// ---- Groovy DSL helpers ---------------------------------------------------
//
// Shared by every build.gradle mutator. The scanning is deliberately
// pragmatic: blocks are located by a column-0 opener and matched by brace
// counting that does not understand strings or comments. That holds for the
// scaffold templates, which are the only files these mutators edit.

// requireGroovyDSL errors with errKotlinDSL when a `.kts` sibling of a
// `.gradle` path exists, so the error names the actual situation (Groovy
// expected, Kotlin DSL found) instead of a confusing read failure.
func requireGroovyDSL(path string) error {
	if !strings.HasSuffix(path, ".gradle") {
		return nil
	}
	if _, err := os.Stat(path + ".kts"); err == nil {
		return errKotlinDSL
	}
	return nil
}

// rewriteGradle reads path, applies edit, and writes the result back only
// when it differs. Reports whether the file changed.
func rewriteGradle(path string, edit func([]byte) ([]byte, error)) (bool, error) {
	if err := requireGroovyDSL(path); err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	updated, err := edit(data)
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if bytes.Equal(updated, data) {
		return false, nil
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

// gradleBlock locates a top-level `name { ... }` block within a source
// buffer: bodyStart is just past the opening brace, closeIdx is the index of
// the matching closing brace.
type gradleBlock struct {
	bodyStart int
	closeIdx  int
}

func (b gradleBlock) body(src []byte) string { return string(src[b.bodyStart:b.closeIdx]) }

// findTopLevelBlock finds the first `name {` opener at column 0 and its
// matching closing brace. Anchoring on column 0 skips nested blocks of the
// same name.
func findTopLevelBlock(src []byte, name string) (gradleBlock, error) {
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\s*\{`)
	loc := re.FindIndex(src)
	if loc == nil {
		return gradleBlock{}, fmt.Errorf("could not locate `%s {` block", name)
	}
	depth := 1
	for i := loc[1]; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return gradleBlock{bodyStart: loc[1], closeIdx: i}, nil
			}
		}
	}
	return gradleBlock{}, fmt.Errorf("could not locate closing `}` of %s block", name)
}

// insertBeforeClose inserts lines immediately before the brace at closeIdx,
// each on its own line, keeping the brace on its own line. Returns src
// unchanged when lines is empty.
func insertBeforeClose(src []byte, closeIdx int, lines []string) []byte {
	if len(lines) == 0 {
		return src
	}
	prefix := src[:closeIdx]
	var b bytes.Buffer
	b.Write(prefix)
	if len(prefix) > 0 && prefix[len(prefix)-1] != '\n' {
		b.WriteByte('\n')
	}
	for _, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.Write(src[closeIdx:])
	return b.Bytes()
}
