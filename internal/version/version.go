// Package version contains build-time version information.
package version

import "runtime/debug"

const unknownValue = "unknown"

// These variables are set at build time via -ldflags.
// If not set, we attempt to read from Go module info (for go install @version).
var (
	// Version is the semantic version (e.g., "v0.1.0")
	Version = "dev"

	// GitCommit is the git commit SHA
	GitCommit = unknownValue

	// BuildDate is the build timestamp in RFC3339 format
	BuildDate = unknownValue
)

func init() {
	initVersionFromBuildInfo()
	initVCSInfoFromBuildInfo()
}

// initVersionFromBuildInfo attempts to set Version from Go module build info.
// This works when installed via: go install ...@v0.1.0
func initVersionFromBuildInfo() {
	if Version != "dev" {
		return
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
}

// initVCSInfoFromBuildInfo attempts to set GitCommit and BuildDate from VCS build settings.
func initVCSInfoFromBuildInfo() {
	if GitCommit != unknownValue && BuildDate != unknownValue {
		return
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if GitCommit == unknownValue && len(setting.Value) >= 7 {
				GitCommit = setting.Value[:7]
			}
		case "vcs.time":
			if BuildDate == unknownValue {
				BuildDate = setting.Value
			}
		}
	}
}
