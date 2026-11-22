package common

import "github.com/charmbracelet/crush/internal/format"

// StripBashDisplayPrefix removes a leading "cd <path> && " prefix from a
// command for display purposes, but only when the cd target is exactly the
// project working directory.
func StripBashDisplayPrefix(cmd, workingDir string) string {
	return format.StripBashDisplayPrefix(cmd, workingDir)
}
