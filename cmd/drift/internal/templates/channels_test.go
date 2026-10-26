package templates

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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
