// Package version contains build-time version information.
package version

import "runtime/debug"

// These variables are set at build time via -ldflags.
// If not set, we attempt to read from Go module info (for go install @version).
var (
	// Version is the semantic version (e.g., "v0.1.0")
	Version = "dev"

	// GitCommit is the git commit SHA
	GitCommit = "unknown"

	// BuildDate is the build timestamp in RFC3339 format
	BuildDate = "unknown"
)

func init() {
	// If ldflags weren't set, try to read from Go module build info.
	// This works when installed via: go install ...@v0.1.0
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}

	// Try to get VCS info from build settings
	if GitCommit == "unknown" || BuildDate == "unknown" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					if GitCommit == "unknown" && len(setting.Value) >= 7 {
						GitCommit = setting.Value[:7]
					}
				case "vcs.time":
					if BuildDate == "unknown" {
						BuildDate = setting.Value
					}
				}
			}
		}
	}
}
