package plugin

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

// MinXcodeMajor is the lowest Xcode major version Drift supports. Bumping
// this requires updating the docs (website-docs/intro.md,
// website-docs/guides/getting-started.md, etc.).
const MinXcodeMajor = 16

// XcodeVersionPreflight aborts with a clear message if the installed Xcode is
// older than MinXcodeMajor. Runs whenever `platform == "ios"`, plugins or
// not, because the iOS template uses project format 77 (Xcode 16+)
// unconditionally. xtool builds skip this check; xtool ships its own
// toolchain.
//
// If `xcodebuild` is missing entirely, the function returns a clear error
// asking the user to install Xcode.
func XcodeVersionPreflight() error {
	out, err := exec.Command("xcodebuild", "-version").Output()
	if err != nil {
		if _, perr := exec.LookPath("xcodebuild"); perr != nil {
			return fmt.Errorf("xcodebuild not found on PATH; install Xcode 16+ to build for iOS")
		}
		return fmt.Errorf("xcodebuild -version failed: %w", err)
	}
	major, raw := parseXcodeVersion(string(out))
	if major < 0 {
		return fmt.Errorf("could not parse xcodebuild -version output: %s", raw)
	}
	if major < MinXcodeMajor {
		return fmt.Errorf("Drift requires Xcode %d+; detected Xcode %s. Upgrade Xcode to continue.", MinXcodeMajor, raw)
	}
	return nil
}

var xcodeVersionRE = regexp.MustCompile(`Xcode\s+(\d+)(?:\.(\d+))?`)

// parseXcodeVersion extracts the major version number from output like
// "Xcode 16.2\nBuild version 16C5032a". Returns (-1, raw) on failure.
func parseXcodeVersion(s string) (int, string) {
	m := xcodeVersionRE.FindStringSubmatch(s)
	if m == nil {
		return -1, s
	}
	major, err := strconv.Atoi(m[1])
	if err != nil {
		return -1, s
	}
	if m[2] != "" {
		return major, fmt.Sprintf("%s.%s", m[1], m[2])
	}
	return major, m[1]
}
