package plugin

import (
	"slices"
	"testing"

	driftplugin "github.com/go-drift/drift/pkg/plugin"
	"github.com/go-drift/drift/pkg/plugin/protocol"
)

func deps(t *testing.T, config string) []string {
	t.Helper()
	ctx := driftplugin.NewTestCtx()
	if err := driftplugin.Bind("github.com/go-drift/drift/plugins/media/plugin", Plugin).Build(ctx, []byte(config)); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("ctx.Err: %v", err)
	}
	var coords []string
	for _, op := range ctx.Ops() {
		switch o := op.(type) {
		case *protocol.OpAndroidGradleAddDependency:
			coords = append(coords, o.Coord)
		case *protocol.OpAndroidManifestAddPermission:
			t.Errorf("declared %s; playback needs no permission", o.Name)
		}
	}
	return coords
}

// DASH is opt-in; HLS is always there.
func TestBuildDependencies(t *testing.T) {
	base := []string{
		"androidx.media3:media3-exoplayer:" + media3,
		"androidx.media3:media3-exoplayer-hls:" + media3,
		"androidx.media3:media3-ui:" + media3,
	}
	if got := deps(t, ""); !slices.Equal(got, base) {
		t.Errorf("default deps = %v, want %v", got, base)
	}
	withDASH := append(slices.Clone(base), "androidx.media3:media3-exoplayer-dash:"+media3)
	if got := deps(t, "android:\n  dash: true\n"); !slices.Equal(got, withDASH) {
		t.Errorf("dash deps = %v, want %v", got, withDASH)
	}
}
