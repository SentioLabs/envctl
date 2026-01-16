// Package version contains build-time version information.
package version

// These variables are set at build time via -ldflags.
var (
	// Version is the semantic version (e.g., "v0.1.0")
	Version = "dev"

	// GitCommit is the git commit SHA
	GitCommit = "unknown"

	// BuildDate is the build timestamp in RFC3339 format
	BuildDate = "unknown"
)
