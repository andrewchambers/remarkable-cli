// Package buildinfo identifies the CLI and tablet helper builds.
package buildinfo

// Make sets these values at link time. Plain go builds remain identifiable as
// development builds rather than claiming to be a released version.
var (
	Version = "dev"
	Commit  = "unknown"
)

func String() string { return Version + " (commit " + Commit + ")" }
