package protocol

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func resolveText(t *testing.T, src string) (string, error) {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	n, err := ResolveYAML(&doc)
	if err != nil {
		return "", err
	}
	out, err := yaml.Marshal(n)
	if err != nil {
		t.Fatal(err)
	}
	return string(out), nil
}

func TestResolveYAML(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{
			name: "scalars keep their text",
			src:  "version: 1.10\ncode: 0123\nflag: yes\n",
			want: "version: 1.10\ncode: 0123\nflag: yes\n",
		},
		{
			name: "aliases expand and anchors go",
			src:  "a: &x {k: v}\nb: *x\n",
			want: "a: {k: v}\nb: {k: v}\n",
		},
		{
			name: "written keys beat merged keys",
			src:  "base: &b {k: merged, m: 1}\nc:\n  k: written\n  <<: *b\n",
			want: "base: {k: merged, m: 1}\nc:\n    k: written\n    m: 1\n",
		},
		{
			name: "earlier merged mapping wins",
			src:  "c:\n  <<: [{k: first}, {k: second, m: 2}]\n",
			want: "c:\n    k: first\n    m: 2\n",
		},
		{
			name: "empty document is an empty mapping",
			src:  "",
			want: "{}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveText(t, tt.src)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestResolveYAMLRejectsBadInput(t *testing.T) {
	for src, want := range map[string]string{
		"a: &x [1, *x]\n":   "refers to itself",
		"c:\n  <<: plain\n": "merge key",
	} {
		if _, err := resolveText(t, src); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want it to mention %q", src, err, want)
		}
	}
}
