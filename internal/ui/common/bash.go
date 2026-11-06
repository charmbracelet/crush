package common

import (
	"fmt"
	"strings"
)

// StripBashDisplayPrefix removes a leading "cd <path> && " prefix from a
// command for display purposes, but only when the cd target is exactly the
// project working directory.
func StripBashDisplayPrefix(cmd, workingDir string) string {
	prefix := fmt.Sprintf("cd %s && ", workingDir)
	return strings.TrimPrefix(cmd, prefix)
}
