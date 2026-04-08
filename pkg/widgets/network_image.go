package widgets

import (
	"maps"
	"time"

	"github.com/go-drift/drift/pkg/core"
	driftimage "github.com/go-drift/drift/pkg/image"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
)

// NetworkImage loads and displays an image from a URL with automatic caching.
//
// While loading, it shows a placeholder (defaults to a centered
// [CircularProgressIndicator]). On failure, it shows the ErrorBuilder result
// (defaults to a centered error text). Once loaded, the image fades in over
// FadeDuration using [AnimatedOpacity].
//
// Images are fetched, decoded, and cached by an [image.Loader]. The default
// loader uses a 100-entry in-memory LRU cache and a 500 MB disk cache.
//
// # Example
//
//	widgets.NetworkImage{
//	    URL:    "https://example.com/avatar.jpg",
//	    Width:  200,
//	    Height: 200,
//	    Fit:    widgets.ImageFitCover,
//	}
//
// # Custom Placeholder and Error
//
//	widgets.NetworkImage{
//	    URL:          "https://example.com/photo.jpg",
//	    Width:        300,
//	    Height:       200,
//	    Placeholder:  widgets.Center{Child: widgets.Text{Content: "Loading..."}},
//	    ErrorBuilder: func(err error) core.Widget {
//	        return widgets.Center{Child: widgets.Text{Content: "Failed to load"}}
//	    },
//	}
type NetworkImage struct {
	core.StatefulBase

	// URL is the HTTP(S) image URL to load.
	URL string
	// Headers are optional HTTP headers added to the request (e.g., authorization).
	Headers map[string]string

	// Width constrains the image width. Zero uses the image's intrinsic width.
	Width float64
	// Height constrains the image height. Zero uses the image's intrinsic height.
	Height float64
	// Fit controls how the image is scaled within its bounds. Default: ImageFitCover.
	Fit ImageFit
	// Alignment positions the image within its bounds.
	Alignment layout.Alignment

	// Placeholder is shown while the image is loading.
	// Default: a centered CircularProgressIndicator.
	Placeholder core.Widget
	// ErrorBuilder builds a widget to show on load failure.
	// Default: a centered error message.
	ErrorBuilder func(err error) core.Widget
	// FadeDuration controls the fade-in animation after loading.
	// Zero disables the fade. Default: 300ms.
	FadeDuration *time.Duration

	// Loader overrides the default global image loader.
	// When nil, [image.DefaultLoader] is used.
	Loader *driftimage.Loader

	// SemanticLabel provides an accessibility description of the image.
	SemanticLabel string
}

func (n NetworkImage) CreateState() core.State {
	return &networkImageState{}
}

type networkImageState struct {
	core.StateBase
	loadedURL string
	result    *driftimage.LoadResult
	cancel    func()
	opacity   float64 // 0.0 while fading in, 1.0 once visible or fade disabled
}

func (s *networkImageState) currentWidget() NetworkImage {
	return s.Element().Widget().(NetworkImage)
}

func (s *networkImageState) InitState() {
	s.startLoad(s.currentWidget())
}

func (s *networkImageState) DidUpdateWidget(oldWidget core.StatefulWidget) {
	old := oldWidget.(NetworkImage)
	w := s.currentWidget()
	if needsReload(old, w) {
		s.cancelLoad()
		s.result = nil
		s.loadedURL = ""
		s.startLoad(w)
	}
}

func (s *networkImageState) Dispose() {
	s.cancelLoad()
	s.StateBase.Dispose()
}

func (s *networkImageState) startLoad(w NetworkImage) {
	if w.URL == "" {
		return
	}

	loader := w.Loader
	if loader == nil {
		loader = driftimage.DefaultLoader()
	}

	s.cancel = loader.Load(w.URL, driftimage.LoadOptions{Headers: w.Headers}, func(result driftimage.LoadResult) {
		platform.Dispatch(func() {
			fade := result.Err == nil && s.fadeDuration(s.currentWidget()) > 0
			s.SetState(func() {
				s.loadedURL = w.URL
				s.result = &result
				if fade {
					s.opacity = 0
				} else {
					s.opacity = 1.0
				}
			})
			if fade {
				// Schedule opacity change for the next frame so
				// AnimatedOpacity sees the 0 to 1 transition.
				platform.Dispatch(func() {
					s.SetState(func() { s.opacity = 1.0 })
				})
			}
		})
	})
}

func (s *networkImageState) cancelLoad() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *networkImageState) Build(ctx core.BuildContext) core.Widget {
	w := s.currentWidget()

	// Error state.
	if s.result != nil && s.result.Err != nil {
		return s.wrapSize(w, s.buildError(w, s.result.Err))
	}

	// Loading state.
	if s.result == nil || s.result.Image == nil {
		return s.wrapSize(w, s.buildPlaceholder(w))
	}

	// Loaded state.
	img := Image{
		Source:        s.result.Image,
		Width:         w.Width,
		Height:        w.Height,
		Fit:           w.Fit,
		Alignment:     w.Alignment,
		SemanticLabel: w.SemanticLabel,
	}

	fadeDuration := s.fadeDuration(w)
	if fadeDuration <= 0 {
		return img
	}

	return AnimatedOpacity{
		Duration: fadeDuration,
		Opacity:  s.opacity,
		Child:    img,
	}
}

func (s *networkImageState) buildPlaceholder(w NetworkImage) core.Widget {
	if w.Placeholder != nil {
		return w.Placeholder
	}
	return Center{Child: CircularProgressIndicator{}}
}

func (s *networkImageState) buildError(w NetworkImage, err error) core.Widget {
	if w.ErrorBuilder != nil {
		return w.ErrorBuilder(err)
	}
	return Center{Child: Text{Content: "Failed to load image"}}
}

// wrapSize wraps a widget in a SizedBox if explicit dimensions are set,
// so the placeholder and error states occupy the same space as the loaded image.
func (s *networkImageState) wrapSize(w NetworkImage, child core.Widget) core.Widget {
	if w.Width > 0 || w.Height > 0 {
		return SizedBox{Width: w.Width, Height: w.Height, Child: child}
	}
	return child
}

const defaultFadeDuration = 300 * time.Millisecond

func (s *networkImageState) fadeDuration(w NetworkImage) time.Duration {
	if w.FadeDuration != nil {
		return *w.FadeDuration
	}
	return defaultFadeDuration
}

func needsReload(old, next NetworkImage) bool {
	return old.URL != next.URL ||
		old.Loader != next.Loader ||
		!maps.Equal(old.Headers, next.Headers)
}
