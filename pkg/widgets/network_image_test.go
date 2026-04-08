package widgets

import (
	"testing"

	driftimage "github.com/go-drift/drift/pkg/image"
)

func TestNeedsReload(t *testing.T) {
	loaderA := driftimage.NewLoader(driftimage.LoaderOptions{})
	loaderB := driftimage.NewLoader(driftimage.LoaderOptions{})

	tests := []struct {
		name string
		old  NetworkImage
		next NetworkImage
		want bool
	}{
		{
			name: "same inputs",
			old: NetworkImage{
				URL:     "https://example.com/a.png",
				Headers: map[string]string{"Authorization": "Bearer one"},
				Loader:  loaderA,
			},
			next: NetworkImage{
				URL:     "https://example.com/a.png",
				Headers: map[string]string{"Authorization": "Bearer one"},
				Loader:  loaderA,
			},
			want: false,
		},
		{
			name: "url changed",
			old:  NetworkImage{URL: "https://example.com/a.png"},
			next: NetworkImage{URL: "https://example.com/b.png"},
			want: true,
		},
		{
			name: "headers changed",
			old: NetworkImage{
				URL:     "https://example.com/a.png",
				Headers: map[string]string{"Authorization": "Bearer one"},
			},
			next: NetworkImage{
				URL:     "https://example.com/a.png",
				Headers: map[string]string{"Authorization": "Bearer two"},
			},
			want: true,
		},
		{
			name: "loader changed",
			old: NetworkImage{
				URL:    "https://example.com/a.png",
				Loader: loaderA,
			},
			next: NetworkImage{
				URL:    "https://example.com/a.png",
				Loader: loaderB,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsReload(tt.old, tt.next); got != tt.want {
				t.Fatalf("needsReload() = %v, want %v", got, tt.want)
			}
		})
	}
}
