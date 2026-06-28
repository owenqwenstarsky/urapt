// Package version holds build-time version information for urapt.
package version

import "runtime/debug"

// Version is the urapt build version. It is populated from VCS info when
// available, otherwise it falls back to "dev".
var Version = "dev"

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				Version = s.Value
				return
			}
		}
	}
}
