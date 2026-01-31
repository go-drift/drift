package engine

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

// waitForServer polls the health endpoint until ready or timeout.
func waitForServer(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://localhost:%d/health", port)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("server not ready after %v", timeout)
}

// waitForServerDown polls until the server stops responding or timeout.
func waitForServerDown(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://localhost:%d/health", port)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err != nil {
			return nil // Connection refused = server is down
		}
		resp.Body.Close()
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("server still running after %v", timeout)
}

func TestDebugServer_StartStop(t *testing.T) {
	// Use ephemeral port (0)
	port, err := startDebugServer(0)
	if err != nil {
		t.Fatalf("failed to start debug server: %v", err)
	}
	defer stopDebugServer()

	if err := waitForServer(port, 2*time.Second); err != nil {
		t.Fatalf("server not ready: %v", err)
	}

	// Test health endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		t.Fatalf("failed to reach health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var health map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if health["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", health["status"])
	}

	// Stop server
	stopDebugServer()

	// Verify server is stopped
	if err := waitForServerDown(port, 2*time.Second); err != nil {
		t.Errorf("server did not stop: %v", err)
	}
}

func TestDebugServer_RenderTreeEndpoint_NoRoot(t *testing.T) {
	port, err := startDebugServer(0)
	if err != nil {
		t.Fatalf("failed to start debug server: %v", err)
	}
	defer stopDebugServer()

	if err := waitForServer(port, 2*time.Second); err != nil {
		t.Fatalf("server not ready: %v", err)
	}

	// Without a root render object, should return 503
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/render-tree", port))
	if err != nil {
		t.Fatalf("failed to reach render-tree endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 with no root, got %d", resp.StatusCode)
	}
}

func TestDebugServer_WidgetTreeEndpoint_NoRoot(t *testing.T) {
	port, err := startDebugServer(0)
	if err != nil {
		t.Fatalf("failed to start debug server: %v", err)
	}
	defer stopDebugServer()

	if err := waitForServer(port, 2*time.Second); err != nil {
		t.Fatalf("server not ready: %v", err)
	}

	// Without a root element, should return 503
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/widget-tree", port))
	if err != nil {
		t.Fatalf("failed to reach widget-tree endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 with no root, got %d", resp.StatusCode)
	}
}

func TestDebugServer_MethodNotAllowed(t *testing.T) {
	port, err := startDebugServer(0)
	if err != nil {
		t.Fatalf("failed to start debug server: %v", err)
	}
	defer stopDebugServer()

	if err := waitForServer(port, 2*time.Second); err != nil {
		t.Fatalf("server not ready: %v", err)
	}

	// POST to health should fail
	resp, err := http.Post(fmt.Sprintf("http://localhost:%d/health", port), "application/json", nil)
	if err != nil {
		t.Fatalf("failed to reach health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405 for POST, got %d", resp.StatusCode)
	}
}

func TestDebugServer_FailFastOnPortConflict(t *testing.T) {
	// Occupy a port with a plain listener
	blocker, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to create blocker listener: %v", err)
	}
	defer blocker.Close()

	blockedPort := blocker.Addr().(*net.TCPAddr).Port

	// Try to start debug server on the occupied port - should fail immediately
	_, err = startDebugServer(blockedPort)
	if err == nil {
		stopDebugServer()
		t.Error("expected error when binding to occupied port, got nil")
	}
}

func TestDebugServer_AlreadyRunningReturnsPort(t *testing.T) {
	// Start server
	port1, err := startDebugServer(0)
	if err != nil {
		t.Fatalf("failed to start debug server: %v", err)
	}
	defer stopDebugServer()

	// Calling start again should return the same port (no error)
	port2, err := startDebugServer(0)
	if err != nil {
		t.Fatalf("second start returned error: %v", err)
	}

	if port1 != port2 {
		t.Errorf("expected same port %d, got %d", port1, port2)
	}
}

func TestSetDiagnostics_DebugServerPort(t *testing.T) {
	// Allocate an ephemeral port first to get a free port number
	tempListener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to get ephemeral port: %v", err)
	}
	port := tempListener.Addr().(*net.TCPAddr).Port
	tempListener.Close() // Release so debug server can use it

	// Enable debug server through SetDiagnostics
	SetDiagnostics(&DiagnosticsConfig{
		DebugServerPort: port,
	})

	if err := waitForServer(port, 2*time.Second); err != nil {
		t.Fatalf("debug server not running after SetDiagnostics: %v", err)
	}

	// Disable by setting nil
	SetDiagnostics(nil)

	// Verify server stopped
	if err := waitForServerDown(port, 2*time.Second); err != nil {
		t.Errorf("debug server still running after disabling diagnostics: %v", err)
	}
}

func TestSetDiagnostics_PortChange(t *testing.T) {
	// Allocate two ephemeral ports
	temp1, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to get first ephemeral port: %v", err)
	}
	port1 := temp1.Addr().(*net.TCPAddr).Port
	temp1.Close()

	temp2, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to get second ephemeral port: %v", err)
	}
	port2 := temp2.Addr().(*net.TCPAddr).Port
	temp2.Close()

	// Start on first port
	SetDiagnostics(&DiagnosticsConfig{
		DebugServerPort: port1,
	})

	if err := waitForServer(port1, 2*time.Second); err != nil {
		t.Fatalf("first server not ready: %v", err)
	}

	// Switch to second port
	SetDiagnostics(&DiagnosticsConfig{
		DebugServerPort: port2,
	})

	// Verify old port is stopped
	if err := waitForServerDown(port1, 2*time.Second); err != nil {
		t.Errorf("old port %d still running: %v", port1, err)
	}

	// Verify new port is running
	if err := waitForServer(port2, 2*time.Second); err != nil {
		t.Fatalf("new port %d not ready: %v", port2, err)
	}

	// Cleanup
	SetDiagnostics(nil)
	waitForServerDown(port2, 2*time.Second)
}

func TestSetDiagnostics_SamePortNoRestart(t *testing.T) {
	// Allocate an ephemeral port
	temp, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("failed to get ephemeral port: %v", err)
	}
	port := temp.Addr().(*net.TCPAddr).Port
	temp.Close()

	// Start server
	SetDiagnostics(&DiagnosticsConfig{
		DebugServerPort: port,
	})
	defer SetDiagnostics(nil)

	if err := waitForServer(port, 2*time.Second); err != nil {
		t.Fatalf("server not ready: %v", err)
	}

	// Get the listener reference
	debugSrv.mu.Lock()
	listener1 := debugSrv.listener
	debugSrv.mu.Unlock()

	// Call SetDiagnostics with same port - should not restart
	SetDiagnostics(&DiagnosticsConfig{
		DebugServerPort: port,
		ShowFPS:         true, // Different config, same port
	})

	// Verify same listener (no restart)
	debugSrv.mu.Lock()
	listener2 := debugSrv.listener
	debugSrv.mu.Unlock()

	if listener1 != listener2 {
		t.Error("server was restarted when port didn't change")
	}
}
