// Package buildinfo identifies the executable currently running on a host.
package buildinfo

import (
	"fmt"
	"runtime/debug"
)

// These variables can be set at build time via -ldflags
var (
	Version   = "dev"
	Commit    = ""
	BuildTime = ""
)

func String() string {
	revision, built, dirty := Commit, BuildTime, ""

	// Fallback to VCS info if not set via ldflags
	if revision == "" || built == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					if revision == "" {
						revision = setting.Value
					}
				case "vcs.time":
					if built == "" {
						built = setting.Value
					}
				case "vcs.modified":
					if setting.Value == "true" {
						dirty = "+modified"
					}
				}
			}
		}
	}

	if revision == "" {
		revision = "unknown"
	}
	if built == "" {
		built = "unknown"
	}

	// Truncate long commit hashes
	if len(revision) > 12 {
		revision = revision[:12]
	}

	return fmt.Sprintf("%s (%s%s, built %s)", Version, revision, dirty, built)
}

// GetVersion returns the version string.
func GetVersion() string {
	if Version == "" || Version == "dev" {
		return "0.2.0"
	}
	return Version
}
