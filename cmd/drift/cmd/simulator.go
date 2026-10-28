package cmd

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// iosSimulator is one available iOS Simulator device as simctl reports it.
type iosSimulator struct {
	Name    string
	UDID    string
	Booted  bool
	Runtime iosRuntime
}

// iosRuntime is a Simulator runtime's iOS version, e.g. 26.5.
type iosRuntime struct {
	Major, Minor int
}

func (r iosRuntime) String() string {
	return fmt.Sprintf("iOS %d.%d", r.Major, r.Minor)
}

func (r iosRuntime) compare(o iosRuntime) int {
	return cmp.Or(cmp.Compare(r.Major, o.Major), cmp.Compare(r.Minor, o.Minor))
}

func (s iosSimulator) String() string {
	return fmt.Sprintf("%s (%s, %s)", s.Name, s.Runtime, s.UDID)
}

func (s iosSimulator) isIPhone() bool {
	return strings.HasPrefix(s.Name, "iPhone")
}

// loadIOSSimulators lists the available iOS Simulators.
func loadIOSSimulators() ([]iosSimulator, error) {
	cmd := exec.Command("xcrun", "simctl", "list", "devices", "available", "--json")
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to list iOS simulators: %w\n%s", err, stderr.String())
	}
	return parseIOSSimulators(out.Bytes())
}

// parseIOSSimulators parses `simctl list devices available --json` output,
// keeping iOS runtimes only (not watchOS, tvOS or visionOS).
func parseIOSSimulators(data []byte) ([]iosSimulator, error) {
	var result struct {
		Devices map[string][]struct {
			Name  string `json:"name"`
			State string `json:"state"`
			UDID  string `json:"udid"`
		} `json:"devices"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse simctl output: %w", err)
	}
	var sims []iosSimulator
	for key, devices := range result.Devices {
		rt, ok := parseIOSRuntime(key)
		if !ok {
			continue
		}
		for _, d := range devices {
			sims = append(sims, iosSimulator{Name: d.Name, UDID: d.UDID, Booted: d.State == "Booted", Runtime: rt})
		}
	}
	return sims, nil
}

// parseIOSRuntime reads the version from a runtime identifier such as
// "com.apple.CoreSimulator.SimRuntime.iOS-26-5".
func parseIOSRuntime(key string) (iosRuntime, bool) {
	_, version, ok := strings.Cut(key, "SimRuntime.iOS-")
	if !ok {
		return iosRuntime{}, false
	}
	major, minor, _ := strings.Cut(version, "-")
	ma, err := strconv.Atoi(major)
	if err != nil {
		return iosRuntime{}, false
	}
	mi, err := strconv.Atoi(minor)
	if err != nil {
		mi = 0
	}
	return iosRuntime{Major: ma, Minor: mi}, true
}

// resolveIOSSimulator picks the simulator to run on. An empty id picks the
// booted iPhone, else the iPhone on the newest runtime. Otherwise id is a
// UDID or a device name; a name on several runtimes picks the booted one,
// else the one on the newest runtime.
//
// The result is always one device with its UDID, so xcodebuild, simctl and
// the log stream all target the same simulator: a bare name makes xcodebuild
// assume the newest runtime, where an older device of that name is not found.
func resolveIOSSimulator(sims []iosSimulator, id string) (iosSimulator, error) {
	var candidates []iosSimulator
	if id == "" {
		for _, s := range sims {
			if s.isIPhone() {
				candidates = append(candidates, s)
			}
		}
		if len(candidates) == 0 {
			return iosSimulator{}, fmt.Errorf("no iPhone simulators available\nInstall an iOS Simulator runtime in Xcode (Settings, Components)")
		}
	} else {
		for _, s := range sims {
			if strings.EqualFold(s.UDID, id) {
				return s, nil
			}
			if strings.EqualFold(s.Name, id) {
				candidates = append(candidates, s)
			}
		}
		if len(candidates) == 0 {
			return iosSimulator{}, fmt.Errorf("simulator %q not found\nAvailable iPhone simulators:\n%s", id, formatIOSSimulators(sims))
		}
	}
	return slices.MaxFunc(candidates, func(a, b iosSimulator) int {
		if a.Booted != b.Booted {
			if a.Booted {
				return 1
			}
			return -1
		}
		return a.Runtime.compare(b.Runtime)
	}), nil
}

// formatIOSSimulators lists the iPhone simulators, newest runtime first.
func formatIOSSimulators(sims []iosSimulator) string {
	var phones []iosSimulator
	for _, s := range sims {
		if s.isIPhone() {
			phones = append(phones, s)
		}
	}
	if len(phones) == 0 {
		return "  (none)"
	}
	slices.SortFunc(phones, func(a, b iosSimulator) int {
		return cmp.Or(b.Runtime.compare(a.Runtime), strings.Compare(a.Name, b.Name))
	})
	lines := make([]string, len(phones))
	for i, s := range phones {
		lines[i] = "  " + s.String()
	}
	return strings.Join(lines, "\n")
}
