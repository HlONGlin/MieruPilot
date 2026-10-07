// Package buildinfo identifies the executable currently running on a host.
package buildinfo

import (
	"fmt"
	"runtime/debug"
)

var Version = "0.2.0"

func String() string {
	revision, built, dirty := "unknown", "unknown", ""
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
				if len(revision) > 12 {
					revision = revision[:12]
				}
			case "vcs.time":
				built = setting.Value
			case "vcs.modified":
				if setting.Value == "true" {
					dirty = "+modified"
				}
			}
		}
	}
	return fmt.Sprintf("%s (%s%s, commit time %s)", Version, revision, dirty, built)
}
