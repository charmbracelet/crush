package diff

import (
	"strings"

	"github.com/aymanbagabas/go-udiff"
)

// normalizeLineEndings normalizes line endings to LF for consistent diff.
func normalizeLineEndings(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return s
}

// GenerateDiff creates a unified diff from two file contents
func GenerateDiff(beforeContent, afterContent, fileName string) (string, int, int) {
	fileName = strings.TrimPrefix(fileName, "/")

	// Normalize line endings to ignore CRLF vs LF differences
	beforeContent = normalizeLineEndings(beforeContent)
	afterContent = normalizeLineEndings(afterContent)

	var (
		unified   = udiff.Unified("a/"+fileName, "b/"+fileName, beforeContent, afterContent)
		additions = 0
		removals  = 0
	)

	lines := strings.SplitSeq(unified, "\n")
	for line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			additions++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			removals++
		}
	}

	return unified, additions, removals
}
