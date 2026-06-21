package diffdetect

import "strings"

// Signal describes which unified-diff markers were found while scanning text.
type Signal struct {
	HasHunk       bool
	HasFileHeader bool
	HasGitHeader  bool
}

// Inspect scans content for unified-diff markers.
func Inspect(content string) Signal {
	var signal Signal
	for line := range strings.SplitSeq(content, "\n") {
		if strings.HasPrefix(line, "@@") {
			signal.HasHunk = true
		}
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			signal.HasFileHeader = true
		}
		if strings.HasPrefix(line, "diff --git ") {
			signal.HasGitHeader = true
		}
	}
	return signal
}

// IsUnifiedDiff reports whether content appears to be a unified diff.
func IsUnifiedDiff(content string) bool {
	signal := Inspect(content)
	if signal.HasGitHeader && signal.HasFileHeader {
		return true
	}
	return signal.HasHunk && signal.HasFileHeader
}
