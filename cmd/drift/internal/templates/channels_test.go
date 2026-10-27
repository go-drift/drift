package templates

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Every Drift channel pkg/platform calls must be registered natively on
// both platforms, through registerChannel like a plugin's, so its methods
// run on the main thread. A channel lost on one platform fails every call
// with channel_not_found.
func TestTemplatesRegisterEveryDriftChannel(t *testing.T) {
	channels := platformMethodChannels(t)
	if len(channels) == 0 {
		t.Fatal("found no drift/ method channels in pkg/platform")
	}
	for _, glob := range []string{"ios/*.swift", "android/java/*.kt"} {
		sources := concatTemplates(t, glob)
		for _, name := range channels {
			if !strings.Contains(sources, `registerChannel("`+name+`")`) {
				t.Errorf("%s: no registerChannel(%q)", glob, name)
			}
		}
	}
}

// platformMethodChannels returns the drift/ channels pkg/platform invokes:
// each NewMethodChannel's name, and the stream lifecycle channel.
func platformMethodChannels(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("../../../../pkg/platform/*.go")
	if err != nil {
		t.Fatal(err)
	}
	var src strings.Builder
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src.Write(data)
	}
	text := src.String()
	consts := map[string]string{}
	for _, m := range regexp.MustCompile(`(\w+)\s*=\s*"(drift/[\w/]+)"`).FindAllStringSubmatch(text, -1) {
		consts[m[1]] = m[2]
	}
	var names []string
	for _, m := range regexp.MustCompile(`NewMethodChannel\(("drift/[\w/]+"|\w+)\)`).FindAllStringSubmatch(text, -1) {
		name, ok := strings.Trim(m[1], `"`), true
		if !strings.HasPrefix(m[1], `"`) {
			name, ok = consts[m[1]]
		}
		if !ok {
			t.Fatalf("pkg/platform: NewMethodChannel(%s) names no drift/ constant", m[1])
		}
		names = append(names, name)
	}
	stream, ok := consts["streamsChannel"]
	if !ok {
		t.Fatal("pkg/platform: streamsChannel not found")
	}
	return append(names, stream)
}

func concatTemplates(t *testing.T, glob string) string {
	t.Helper()
	paths, err := fs.Glob(FS, glob)
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob %s: %v (%d files)", glob, err, len(paths))
	}
	var sb strings.Builder
	for _, p := range paths {
		data, err := ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(data)
	}
	return sb.String()
}

// Every first-party plugin method replies the same way (method or
// asyncMethod) on iOS and Android. Go's call is the same on both, and iOS
// fails an asyncMethod called from the UI thread with blocks_ui_thread,
// so a method sync on one platform and async on the other works for a UI
// thread caller on one and fails it on the other.
func TestPluginMethodsReplyAlikeOnBothPlatforms(t *testing.T) {
	plugins, err := filepath.Glob("../../../../plugins/*/plugin")
	if err != nil || len(plugins) == 0 {
		t.Fatalf("glob plugins: %v (%d plugins)", err, len(plugins))
	}
	compared := 0
	for _, dir := range plugins {
		name := filepath.Base(filepath.Dir(dir))
		ios := nativeMethods(t, filepath.Join(dir, "ios", "*.swift"))
		android := nativeMethods(t, filepath.Join(dir, "android*", "*.kt"))
		methods := maps.Clone(ios.named)
		maps.Copy(methods, android.named)
		for method := range methods {
			iosKinds, androidKinds := ios.kinds(method), android.kinds(method)
			switch {
			case len(iosKinds) == 0:
				t.Errorf("%s: %s registered on Android only", name, method)
			case len(androidKinds) == 0:
				t.Errorf("%s: %s registered on iOS only", name, method)
			case !maps.Equal(iosKinds, androidKinds):
				t.Errorf("%s: %s is %v on iOS, %v on Android", name, method, slices.Sorted(maps.Keys(iosKinds)), slices.Sorted(maps.Keys(androidKinds)))
			}
			compared++
		}
	}
	if compared == 0 {
		t.Fatal("found no plugin methods to compare")
	}
}

// pluginMethods are the methods one platform's plugin sources declare.
type pluginMethods struct {
	// named maps each method declared by a literal name to its kinds
	// (method, asyncMethod).
	named map[string]map[string]bool
	// dynamic holds the kinds of methods declared by a variable name, as
	// in a loop over the names.
	dynamic map[string]bool
}

// kinds returns how method replies: its literal declarations' kinds, or
// otherwise the dynamic declarations'.
func (m pluginMethods) kinds(method string) map[string]bool {
	if k, ok := m.named[method]; ok {
		return k
	}
	return m.dynamic
}

var methodDecl = regexp.MustCompile(`\b(method|asyncMethod)\(\s*(?:"(\w+)"|(\w+)\s*\))`)

// nativeMethods collects the method and asyncMethod declarations in the
// files glob matches, skipping comment lines.
func nativeMethods(t *testing.T, glob string) pluginMethods {
	t.Helper()
	paths, err := filepath.Glob(glob)
	if err != nil {
		t.Fatal(err)
	}
	m := pluginMethods{named: map[string]map[string]bool{}, dynamic: map[string]bool{}}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "*") {
				continue
			}
			for _, d := range methodDecl.FindAllStringSubmatch(line, -1) {
				kind, literal := d[1], d[2]
				if literal == "" {
					m.dynamic[kind] = true
					continue
				}
				if m.named[literal] == nil {
					m.named[literal] = map[string]bool{}
				}
				m.named[literal][kind] = true
			}
		}
	}
	return m
}
