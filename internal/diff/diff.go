package diff

import (
	"strings"

	"github.com/aymanbagabas/go-udiff"
)

// GenerateDiff creates a unified diff from two file contents
func GenerateDiff(beforeContent, afterContent, fileName string) (string, int, int) {
	fileName = strings.TrimPrefix(fileName, "/")

	// A CRLF file reaches a diff in both conventions: the tools read it as LF
	// and report back the CRLF they wrote, and file history keeps the LF it
	// first saw next to the CRLF an edit left behind. Compared as they are,
	// every line disagrees, so both sides are reduced to LF first. This mirrors
	// diffview.normalizeLineEndings, which does the same for the diffs drawn in
	// the terminal.
	beforeContent = strings.ReplaceAll(beforeContent, "\r\n", "\n")
	afterContent = strings.ReplaceAll(afterContent, "\r\n", "\n")

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
