package plugin

import (
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/go-drift/drift/pkg/plugin/protocol"
)

// The plugin guide's op reference is hand-written. It must list every op
// type, and no other, under its wire name (file.add also naming its
// location), and every recorder.
func TestOpReferenceListsEveryOp(t *testing.T) {
	recorders, ops := opReference(t)

	listed := map[string]bool{}
	for _, name := range ops {
		listed[name] = true
		file := &protocol.OpAddFile{Location: protocol.FileLocation(name)}
		if !slices.Contains(protocol.OpTypes(), name) && file.Platform() == "" {
			t.Errorf("op reference lists %q, which is neither an op type nor a file location", name)
		}
	}
	for _, typ := range protocol.OpTypes() {
		if !listed[typ] {
			t.Errorf("op reference does not list %q", typ)
		}
	}

	for _, method := range recorderMethods(reflect.TypeFor[*BuildCtx](), map[reflect.Type]bool{}) {
		if !strings.Contains(recorders, method+"(") {
			t.Errorf("op reference does not list recorder %s", method)
		}
	}
}

// opReference returns the guide's op reference tables: the Recorder
// column's text, and every backquoted name in the Op column.
func opReference(t *testing.T) (recorders string, ops []string) {
	t.Helper()
	guide, err := os.ReadFile("../../website-docs/guides/plugin-authoring.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(guide), "\n## Op reference\n")
	if !ok {
		t.Fatal("guide has no ## Op reference")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	quoted := regexp.MustCompile("`([^`]+)`")
	var sb strings.Builder
	for _, line := range strings.Split(section, "\n") {
		cols := strings.Split(line, "|")
		if len(cols) < 4 || !strings.HasPrefix(strings.TrimSpace(cols[1]), "`") {
			continue // not a table row, or a header
		}
		sb.WriteString(cols[1])
		for _, m := range quoted.FindAllStringSubmatch(cols[2], -1) {
			if !strings.Contains(m[1], "*") { // "the two `ios.plist.*` ops above"
				ops = append(ops, m[1])
			}
		}
	}
	return sb.String(), ops
}

// recorderMethods returns the exported methods that record ops on t, a
// *BuildCtx or one of its scopes, and on the scopes its fields hold.
func recorderMethods(t reflect.Type, seen map[reflect.Type]bool) []string {
	if seen[t] {
		return nil
	}
	seen[t] = true
	// BuildCtx's other methods read the build, they record nothing.
	accessors := []string{"Err", "Ops", "Plugin", "PluginName", "Platform", "AppID", "ProjectRoot", "BuildDir", "ResolveAsset"}
	var out []string
	for i := range t.NumMethod() {
		name := t.Method(i).Name
		if t == reflect.TypeFor[*BuildCtx]() && slices.Contains(accessors, name) {
			continue
		}
		out = append(out, name)
	}
	for i := range t.Elem().NumField() {
		f := t.Elem().Field(i)
		if f.IsExported() && f.Type.Kind() == reflect.Pointer && strings.HasSuffix(f.Type.Elem().Name(), "Scope") {
			out = append(out, recorderMethods(f.Type, seen)...)
		}
	}
	return out
}
