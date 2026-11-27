package diff

import (
	"strings"

	"github.com/aymanbagabas/go-udiff"
)

// GenerateDiff creates a unified diff from two file contents
func GenerateDiff(beforeContent, afterContent, fileName string) (string, int, int) {
	fileName = strings.TrimPrefix(fileName, "/")

	var (
		unified   = udiff.Unified("a/"+fileName, "b/"+fileName, beforeContent, afterContent)
		additions = 0
		removals  = 0
	)

	// Only lines inside a hunk are counted. The "---" and "+++" file
	// header comes before the first "@@"; a changed line that itself
	// begins with "++" or "--" looks like a header but is still a change.
	inHunk := false
	lines := strings.SplitSeq(unified, "\n")
	for line := range lines {
		switch {
		case strings.HasPrefix(line, "@@"):
			inHunk = true
		case !inHunk:
		case strings.HasPrefix(line, "+"):
			additions++
		case strings.HasPrefix(line, "-"):
			removals++
		}
	}

	return unified, additions, removals
}
