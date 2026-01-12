package version

import "runtime/debug"

// Build-time parameters set via -ldflags

var Version = "devel"

// A user may install crush using `go install github.com/charmbracelet/crush@latest`.
// without -ldflags, in which case the version above is unset. As a workaround
// we use the embedded build version that *is* set when using `go install` (and
// is only set for `go install` and not for `go build`).
func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	if Version == "devel" || Version == "dev" {
		var revision string
		var modified bool
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.modified":
				modified = setting.Value == "true"
			}
		}

		if revision != "" {
			if len(revision) > 7 {
				revision = revision[:7]
			}
			Version = "dev-" + revision
			if modified {
				Version += " (dirty)"
			}
		}
	}

	mainVersion := info.Main.Version
	if mainVersion != "" && mainVersion != "(devel)" {
		Version = mainVersion
	}
}
