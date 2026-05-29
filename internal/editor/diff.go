package editor

import "strings"

// EditedRange returns the 0-indexed half-open line range [start, end) that
// covers the changed region between oldContent and newContent. When the
// contents are identical it returns (0, 0). When one side is empty the
// range covers the whole non-empty side.
//
// The result is intentionally coarse: callers (e.g. the FlashEdit
// highlight) only need a reasonable region, not a minimal diff.
func EditedRange(oldContent, newContent string) (start, end int) {
	if oldContent == newContent {
		return 0, 0
	}
	if oldContent == "" {
		return 0, lineCount(newContent)
	}
	if newContent == "" {
		return 0, lineCount(oldContent)
	}

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Common prefix.
	prefix := 0
	maxPrefix := min(len(oldLines), len(newLines))
	for prefix < maxPrefix && oldLines[prefix] == newLines[prefix] {
		prefix++
	}

	// Common suffix, not crossing the prefix on either side.
	suffix := 0
	for suffix < len(oldLines)-prefix && suffix < len(newLines)-prefix &&
		oldLines[len(oldLines)-1-suffix] == newLines[len(newLines)-1-suffix] {
		suffix++
	}

	end = len(newLines) - suffix
	if end <= prefix {
		// Pure deletion: highlight the line at the cut so the user sees
		// where content disappeared.
		end = prefix + 1
	}
	if end > len(newLines) {
		end = len(newLines)
	}
	return prefix, end
}

// lineCount returns the number of 1-indexed lines in s, treating the
// trailing newline (if any) as terminating the last line.
func lineCount(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
