package diff

import (
	"strings"

	"github.com/aymanbagabas/go-udiff"
	"github.com/sergi/go-diff/diffmatchpatch"
)

func generateMyersDiff(beforeContent, afterContent string) []diffmatchpatch.Diff {
	dmp := diffmatchpatch.New()

	a, b, lineArray := dmp.DiffLinesToRunes(beforeContent, afterContent)
	diffs := dmp.DiffMainRunes(a, b, false)
	return dmp.DiffCharsToLines(diffs, lineArray)
}

func GenerateDiff(beforeContent, afterContent, fileName string) (string, int, int) {
	fileName = strings.TrimPrefix(fileName, "/")

	diffs := generateMyersDiff(beforeContent, afterContent)
	unified := convertMyersToUnifiedDiff(diffs, "a/"+fileName, "b/"+fileName, udiff.DefaultContextLines)

	additions, removals := countChanges(diffs)

	return unified.String(), additions, removals
}

func GenerateUnifiedDiff(beforeContent, afterContent, beforePath, afterPath string, contextLines int) udiff.UnifiedDiff {
	diffs := generateMyersDiff(beforeContent, afterContent)
	return convertMyersToUnifiedDiff(diffs, beforePath, afterPath, contextLines)
}

func countChanges(diffs []diffmatchpatch.Diff) (additions, removals int) {
	for _, d := range diffs {
		lines := strings.Split(d.Text, "\n")
		lineCount := len(lines)
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lineCount--
		}

		switch d.Type {
		case diffmatchpatch.DiffInsert:
			additions += lineCount
		case diffmatchpatch.DiffDelete:
			removals += lineCount
		}
	}
	return additions, removals
}
