package diff

import (
	"strings"

	"github.com/aymanbagabas/go-udiff"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// convertMyersToUnifiedDiff converts Myers diff output to go-udiff's UnifiedDiff
// structure, preserving the hierarchical hunk organization that the rendering
// code expects.
func convertMyersToUnifiedDiff(diffs []diffmatchpatch.Diff, fromName, toName string, contextLines int) udiff.UnifiedDiff {
	if len(diffs) == 0 {
		return udiff.UnifiedDiff{
			From: fromName,
			To:   toName,
		}
	}

	var lines []diffLine
	for _, d := range diffs {
		splitDiff := splitDiffIntoLines(d)
		lines = append(lines, splitDiff...)
	}

	hunks := groupIntoHunks(lines, contextLines)

	hunkPointers := make([]*udiff.Hunk, len(hunks))
	for i := range hunks {
		hunkPointers[i] = &hunks[i]
	}

	return udiff.UnifiedDiff{
		From:  fromName,
		To:    toName,
		Hunks: hunkPointers,
	}
}

// diffLine represents a single line in a diff with its operation type.
type diffLine struct {
	op      udiff.OpKind
	content string
}

// splitDiffIntoLines splits a Myers Diff (which may contain multiple lines)
// into individual diffLines.
func splitDiffIntoLines(d diffmatchpatch.Diff) []diffLine {
	if d.Text == "" {
		return nil
	}

	// Convert Myers operation to go-udiff OpKind
	var op udiff.OpKind
	switch d.Type {
	case diffmatchpatch.DiffEqual:
		op = udiff.Equal
	case diffmatchpatch.DiffInsert:
		op = udiff.Insert
	case diffmatchpatch.DiffDelete:
		op = udiff.Delete
	default:
		op = udiff.Equal
	}

	// Split by newlines but preserve the newlines in content
	lines := strings.Split(d.Text, "\n")
	result := make([]diffLine, 0, len(lines))

	for i, line := range lines {
		// Skip empty last line (from trailing newline)
		if i == len(lines)-1 && line == "" {
			continue
		}

		result = append(result, diffLine{
			op:      op,
			content: line + "\n",
		})
	}

	return result
}

// groupIntoHunks groups diff lines into hunks with context lines.
func groupIntoHunks(lines []diffLine, contextLines int) []udiff.Hunk {
	if len(lines) == 0 {
		return nil
	}

	var hunks []udiff.Hunk
	var currentHunk *udiff.Hunk
	beforeLine := 1
	afterLine := 1

	// Track consecutive context lines to determine hunk boundaries
	contextCount := 0

	for i, line := range lines {
		isChange := line.op == udiff.Insert || line.op == udiff.Delete

		// Start a new hunk if:
		// 1. We encounter a change and have no current hunk
		// 2. We've accumulated too many context lines between changes
		if isChange && (currentHunk == nil || contextCount > contextLines*2) {
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}

			hunkStartBefore := beforeLine
			hunkStartAfter := afterLine

			lookback := min(contextLines, i)
			hunkStartBefore = beforeLine - countBeforeLines(lines[i-lookback:i])
			hunkStartAfter = afterLine - countAfterLines(lines[i-lookback:i])

			currentHunk = &udiff.Hunk{
				FromLine: hunkStartBefore,
				ToLine:   hunkStartAfter,
				Lines:    make([]udiff.Line, 0),
			}

			lookback = min(contextLines, i)
			for j := i - lookback; j < i; j++ {
				currentHunk.Lines = append(currentHunk.Lines, udiff.Line{
					Kind:    lines[j].op,
					Content: lines[j].content,
				})
			}

			contextCount = 0
		}

		if currentHunk != nil {
			currentHunk.Lines = append(currentHunk.Lines, udiff.Line{
				Kind:    line.op,
				Content: line.content,
			})
		}

		// Update line counters
		switch line.op {
		case udiff.Equal:
			beforeLine++
			afterLine++
			contextCount++
		case udiff.Insert:
			afterLine++
			contextCount = 0
		case udiff.Delete:
			beforeLine++
			contextCount = 0
		}

		// Add trailing context lines when we reach the end
		if i == len(lines)-1 && currentHunk != nil {
			// Current hunk is complete
			hunks = append(hunks, *currentHunk)
		}
	}

	return hunks
}

// countBeforeLines counts how many lines affect the "before" file.
func countBeforeLines(lines []diffLine) int {
	count := 0
	for _, line := range lines {
		if line.op == udiff.Equal || line.op == udiff.Delete {
			count++
		}
	}
	return count
}

// countAfterLines counts how many lines affect the "after" file.
func countAfterLines(lines []diffLine) int {
	count := 0
	for _, line := range lines {
		if line.op == udiff.Equal || line.op == udiff.Insert {
			count++
		}
	}
	return count
}
