package plugin

import "github.com/go-drift/drift/pkg/plugin/protocol"

// SPMRequirement is a SwiftPM version requirement for
// IOSScope.AddPackageDependency. It is the wire type itself, aliased so
// plugin authors need only this package.
type SPMRequirement = protocol.SPMRequirement

// SPMRequirementKind names one of SwiftPM's `.package(url:...)` version
// requirement forms.
type SPMRequirementKind = protocol.SPMRequirementKind

// SwiftPM requirement kinds, re-exported from protocol for plugin authors.
const (
	SPMFrom          = protocol.SPMFrom
	SPMExact         = protocol.SPMExact
	SPMBranch        = protocol.SPMBranch
	SPMRevision      = protocol.SPMRevision
	SPMUpToNextMajor = protocol.SPMUpToNextMajor
	SPMUpToNextMinor = protocol.SPMUpToNextMinor
	SPMRange         = protocol.SPMRange
)

// SPMRequirementFrom is sugar for the most common form (`from:`, which is
// SwiftPM's up-to-next-major shorthand).
func SPMRequirementFrom(version string) SPMRequirement {
	return SPMRequirement{Kind: SPMFrom, Value: version}
}
