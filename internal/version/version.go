package version

import (
	"context"
	"os/exec"
	"runtime/debug"
	"strings"
	"time"
)

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

	// Try to get more detailed version info if we are in a dev version.
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

	// If we still have a generic version, try to use the one from build info.
	if Version == "devel" || Version == "dev" {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}

	// Finally, fallback to git command if we are still on a generic version.
	if Version == "devel" || Version == "dev" {
		if gitVersion := getVersionFromGit(); gitVersion != "" {
			Version = gitVersion
		}
	}
}

func getVersionFromGit() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	revision := strings.TrimSpace(string(out))
	if revision == "" {
		return ""
	}

	res := "dev-" + revision
	cmd = exec.CommandContext(ctx, "git", "diff", "--quiet")
	if err := cmd.Run(); err != nil {
		// git diff --quiet returns exit code 1 if there are changes
		res += " (dirty)"
	}
	return res
}
