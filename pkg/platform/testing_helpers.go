package platform

// noopBridge is a NativeBridge that accepts all calls without side effects.
type noopBridge struct{}

func (noopBridge) InvokeMethod(channel, method string, args []byte) ([]byte, error) {
	return DefaultCodec.Encode(nil)
}
func (noopBridge) StartEventStream(string) error { return nil }
func (noopBridge) StopEventStream(string) error  { return nil }

// SetupTestBridge installs a no-op native bridge and synchronous dispatch
// function for testing. The cleanup function should be testing.T.Cleanup or
// equivalent; it registers a teardown that calls ResetForTest.
//
//	platform.SetupTestBridge(t.Cleanup)
func SetupTestBridge(cleanup func(func())) {
	SetNativeBridge(noopBridge{})
	RegisterDispatch(func(cb func()) { cb() })
	cleanup(ResetForTest)
}
