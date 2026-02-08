package platform

import "sync"

type nativeWebViewFactory struct{}

func (nativeWebViewFactory) ViewType() string {
	return "native_webview"
}

func (nativeWebViewFactory) Create(viewID int64, params map[string]any) (PlatformView, error) {
	return &nativeWebView{
		basePlatformView: basePlatformView{
			viewID:   viewID,
			viewType: "native_webview",
		},
	}, nil
}

type nativeWebView struct {
	basePlatformView
	mu sync.RWMutex

	// OnPageStarted is called when a page starts loading.
	// Called on the UI thread via [Dispatch].
	OnPageStarted func(url string)

	// OnPageFinished is called when a page finishes loading.
	// Called on the UI thread via [Dispatch].
	OnPageFinished func(url string)

	// OnError is called when a loading error occurs.
	// The code parameter is one of the ErrCodeNetworkError, ErrCodeSSLError,
	// or ErrCodeLoadFailed constants. Called on the UI thread via [Dispatch].
	OnError func(code, message string)
}

func (v *nativeWebView) Create(params map[string]any) error {
	return nil
}

func (v *nativeWebView) Dispose() {}

// handlePageStarted processes page start events from native.
func (v *nativeWebView) handlePageStarted(url string) {
	v.mu.RLock()
	cb := v.OnPageStarted
	v.mu.RUnlock()

	if cb != nil {
		Dispatch(func() {
			cb(url)
		})
	}
}

// handlePageFinished processes page finish events from native.
func (v *nativeWebView) handlePageFinished(url string) {
	v.mu.RLock()
	cb := v.OnPageFinished
	v.mu.RUnlock()

	if cb != nil {
		Dispatch(func() {
			cb(url)
		})
	}
}

// handleError processes error events from native.
func (v *nativeWebView) handleError(code, message string) {
	v.mu.RLock()
	cb := v.OnError
	v.mu.RUnlock()

	if cb != nil {
		Dispatch(func() {
			cb(code, message)
		})
	}
}

func init() {
	GetPlatformViewRegistry().RegisterFactory(nativeWebViewFactory{})
}
