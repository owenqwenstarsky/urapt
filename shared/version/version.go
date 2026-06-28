// Package version holds build-time version information for urapt.
package version

import "runtime/debug"

// Version is the urapt build version. It can be set at build time via
// -ldflags "-X urapt/shared/version.Version=v0.1.0"; otherwise it is
// populated from VCS info (the current commit) when available, and finally
// falls back to "dev".
var Version = "dev"

func init() {
	// If a version was injected via ldflags, honor it and do not override.
	if Version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				Version = s.Value
				return
			}
		}
	}
}
