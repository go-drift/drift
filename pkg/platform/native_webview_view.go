package platform

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
}

func (v *nativeWebView) Create(params map[string]any) error {
	return nil
}

func (v *nativeWebView) Dispose() {}

func init() {
	GetPlatformViewRegistry().RegisterFactory(nativeWebViewFactory{})
}
