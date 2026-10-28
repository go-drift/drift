package cmd

import (
	"strings"
	"testing"
)

// simctlJSON mirrors a Mac with several runtimes: "iPhone 15" exists only on
// an old runtime, "iPhone 17" on three, and the newest runtime has a single
// device.
const simctlJSON = `{
  "devices": {
    "com.apple.CoreSimulator.SimRuntime.iOS-17-2": [
      {"name": "iPhone 15", "state": "Shutdown", "udid": "UDID-15"}
    ],
    "com.apple.CoreSimulator.SimRuntime.iOS-26-5": [
      {"name": "iPhone 17", "state": "Booted", "udid": "UDID-17-265"},
      {"name": "iPad (A16)", "state": "Shutdown", "udid": "UDID-IPAD"}
    ],
    "com.apple.CoreSimulator.SimRuntime.iOS-27-0": [
      {"name": "iPhone 17", "state": "Shutdown", "udid": "UDID-17-270"}
    ],
    "com.apple.CoreSimulator.SimRuntime.iOS-26-0": [
      {"name": "iPhone 17", "state": "Shutdown", "udid": "UDID-17-260"}
    ],
    "com.apple.CoreSimulator.SimRuntime.iOS-27-1": [
      {"name": "iPhone Duo", "state": "Shutdown", "udid": "UDID-DUO"}
    ],
    "com.apple.CoreSimulator.SimRuntime.watchOS-12-0": [
      {"name": "Apple Watch Series 11", "state": "Booted", "udid": "UDID-WATCH"}
    ]
  }
}`

func mustParseSimulators(t *testing.T, data string) []iosSimulator {
	t.Helper()
	sims, err := parseIOSSimulators([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return sims
}

func TestParseIOSSimulatorsSkipsOtherPlatforms(t *testing.T) {
	sims := mustParseSimulators(t, simctlJSON)
	if len(sims) != 6 {
		t.Fatalf("got %d simulators, want 6 (watchOS skipped): %v", len(sims), sims)
	}
	for _, s := range sims {
		if s.UDID == "UDID-WATCH" {
			t.Errorf("watchOS simulator kept: %v", s)
		}
		if s.UDID == "UDID-DUO" && s.Runtime != (iosRuntime{27, 1}) {
			t.Errorf("iPhone Duo runtime = %v, want iOS 27.1", s.Runtime)
		}
	}
}

func TestResolveIOSSimulator(t *testing.T) {
	sims := mustParseSimulators(t, simctlJSON)
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"default prefers the booted iPhone", "", "UDID-17-265"},
		{"name on several runtimes prefers booted", "iPhone 17", "UDID-17-265"},
		{"name only on an old runtime", "iPhone 15", "UDID-15"},
		{"name is case-insensitive", "iphone duo", "UDID-DUO"},
		{"UDID", "UDID-17-260", "UDID-17-260"},
		{"non-iPhone by name", "iPad (A16)", "UDID-IPAD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveIOSSimulator(sims, tt.id)
			if err != nil {
				t.Fatal(err)
			}
			if got.UDID != tt.want {
				t.Errorf("resolveIOSSimulator(%q) = %v, want %s", tt.id, got, tt.want)
			}
		})
	}
}

func TestResolveIOSSimulatorWithoutBootedPicksNewestRuntime(t *testing.T) {
	sims := mustParseSimulators(t, strings.Replace(simctlJSON, `"Booted", "udid": "UDID-17-265"`, `"Shutdown", "udid": "UDID-17-265"`, 1))
	got, err := resolveIOSSimulator(sims, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.UDID != "UDID-DUO" {
		t.Errorf("default = %v, want the iPhone on iOS 27.1", got)
	}
	got, err = resolveIOSSimulator(sims, "iPhone 17")
	if err != nil {
		t.Fatal(err)
	}
	if got.UDID != "UDID-17-270" {
		t.Errorf("iPhone 17 = %v, want the one on iOS 27.0", got)
	}
}

func TestResolveIOSSimulatorErrors(t *testing.T) {
	sims := mustParseSimulators(t, simctlJSON)
	_, err := resolveIOSSimulator(sims, "iPhone 99")
	if err == nil || !strings.Contains(err.Error(), `"iPhone 99" not found`) || !strings.Contains(err.Error(), "iPhone Duo (iOS 27.1, UDID-DUO)") {
		t.Errorf("unknown name error = %v, want it to name the simulator and list the iPhones", err)
	}
	if _, err := resolveIOSSimulator(nil, ""); err == nil || !strings.Contains(err.Error(), "no iPhone simulators") {
		t.Errorf("no simulators error = %v", err)
	}
}
