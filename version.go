package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

const shortRevisionLen = 12

// version is the build version. It defaults to "dev" and can be overridden at
// build time with -ldflags "-X main.version=<v>". When left as "dev", the
// version is derived from the embedded build info instead.
var version = "dev"

// versionString returns the resolved version, appending the VCS revision (and a
// "-dirty" marker for uncommitted builds) when the binary carries build info.
func versionString() string {
	resolved := version

	revision := ""
	modified := false

	info, ok := debug.ReadBuildInfo()
	if ok {
		if resolved == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			resolved = info.Main.Version
		}

		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.modified":
				modified = setting.Value == "true"
			}
		}
	}

	if revision != "" {
		if len(revision) > shortRevisionLen {
			revision = revision[:shortRevisionLen]
		}

		dirty := ""
		if modified {
			dirty = "-dirty"
		}

		resolved = fmt.Sprintf("%s (%s%s)", resolved, revision, dirty)
	}

	return resolved
}

func printVersion() {
	fmt.Printf("git-tools %s %s/%s\n", versionString(), runtime.GOOS, runtime.GOARCH)
}
