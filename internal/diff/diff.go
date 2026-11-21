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

	// The "---" and "+++" file headers come before the first hunk. Inside
	// a hunk every line is a change or context, so a removed "-- comment"
	// or an added "++i" counts even though it looks like a header.
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
