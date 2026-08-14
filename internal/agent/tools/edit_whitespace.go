package tools

import (
	"fmt"
	"slices"
	"strings"
)

// whitespaceCorrectedNote tells the caller that old_string did not match
// byte-for-byte and that the edit was applied to whitespace-equivalent text
// found in the file, so it can verify the outcome.
const whitespaceCorrectedNote = "Note: old_string did not match exactly. The edit was applied to whitespace-equivalent text in the file and new_string was re-indented to match the file's style. Verify the result."

// normMatch is a line range in the original (un-normalized) content that
// matches the search pattern after whitespace normalization.
type normMatch struct{ startLine, endLine int }

// findNormalizedMatches searches content for old after collapsing each line's
// whitespace runs to single spaces. It returns the line ranges of all
// non-overlapping matches, mapped back to the original content's line numbers.
// Only matches that span whole lines are reported: replacements happen at line
// granularity, so accepting a partial-line match would discard the rest of the
// line.
func findNormalizedMatches(content, old string) []normMatch {
	contentLines := strings.Split(content, "\n")
	oldLines := strings.Split(old, "\n")

	normOld := joinNormalized(oldLines)
	if strings.TrimSpace(normOld) == "" {
		return nil
	}

	normContentLines := make([]string, len(contentLines))
	for i, l := range contentLines {
		normContentLines[i] = normalizeWS(l)
	}
	normContent := strings.Join(normContentLines, "\n")

	var matches []normMatch
	searchFrom := 0
	for searchFrom <= len(normContent) {
		idx := strings.Index(normContent[searchFrom:], normOld)
		if idx == -1 {
			break
		}
		absIdx := searchFrom + idx
		end := absIdx + len(normOld)
		atLineStart := absIdx == 0 || normContent[absIdx-1] == '\n'
		atLineEnd := end == len(normContent) || normContent[end] == '\n'
		if !atLineStart || !atLineEnd {
			searchFrom = absIdx + 1
			continue
		}
		startLine := lineAtOffset(normContentLines, absIdx)
		endLine := startLine + len(oldLines) - 1
		if endLine >= len(contentLines) {
			break
		}
		matches = append(matches, normMatch{startLine, endLine})
		// Whole-line matches consume every line they cover, so resuming after
		// the match keeps the reported ranges disjoint.
		searchFrom = end + 1
	}
	return matches
}

// normalizedReplace attempts a whitespace-normalized find-and-replace when an
// exact match fails. If a unique match is found (or replaceAll is set), it
// extracts the actual text from the file, adapts new's indentation to match
// the file's style, and performs the replacement. Returns the new content and
// true on success, or ("", false) if no safe match was found.
func normalizedReplace(content, old, new string, replaceAll bool) (string, bool) {
	matches := findNormalizedMatches(content, old)
	if len(matches) == 0 {
		return "", false
	}
	if !replaceAll && len(matches) > 1 {
		return "", false // Ambiguous; let the model disambiguate.
	}

	contentLines := strings.Split(content, "\n")
	fileUnit := detectIndentUnit(contentLines)

	// Replace in reverse order to preserve line indices.
	result := slices.Clone(contentLines)
	for _, m := range slices.Backward(matches) {
		actual := strings.Join(contentLines[m.startLine:m.endLine+1], "\n")
		adapted := adaptIndentation(actual, old, new, fileUnit)
		adaptedLines := strings.Split(adapted, "\n")
		result = slices.Concat(result[:m.startLine], adaptedLines, result[m.endLine+1:])
	}

	return strings.Join(result, "\n"), true
}

// joinNormalized normalizes each line and joins with newlines.
func joinNormalized(lines []string) string {
	norm := make([]string, len(lines))
	for i, l := range lines {
		norm[i] = normalizeWS(l)
	}
	return strings.Join(norm, "\n")
}

// adaptIndentation rewrites newStr's leading whitespace to match the file's
// indentation style (fileUnit, detected from the whole file). It converts
// newStr's indentation from the style the caller wrote it in to the file's
// style, applying a depth offset so newStr lands at the same nesting level as
// the actual matched text from the file. The offset is applied even when both
// styles agree, because oldStr's nesting level may still differ from the
// matched text's.
func adaptIndentation(actualStr, oldStr, newStr, fileUnit string) string {
	if fileUnit == "" {
		return newStr
	}

	actualLines := strings.Split(actualStr, "\n")
	oldLines := strings.Split(oldStr, "\n")
	newLines := strings.Split(newStr, "\n")

	// The unit the caller indented with. oldStr is the best signal, but it may
	// have no indentation at all, in which case newStr's own indentation (and
	// finally the file's) is the next best guess.
	sourceUnit := detectIndentUnit(oldLines)
	if sourceUnit == "" {
		sourceUnit = detectIndentUnit(newLines)
	}
	if sourceUnit == "" {
		sourceUnit = fileUnit
	}

	// Compute the base indent depth of the actual match and the old string
	// (from their first non-empty lines). The difference is applied as an
	// offset so that newStr lands at the correct nesting level.
	baseDepth := firstIndentDepth(actualLines, fileUnit)
	oldBaseDepth := firstIndentDepth(oldLines, sourceUnit)
	depthOffset := baseDepth - oldBaseDepth
	if sourceUnit == fileUnit && depthOffset == 0 {
		return newStr
	}

	result := make([]string, len(newLines))
	for i, line := range newLines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			result[i] = line
			continue
		}
		leading := line[:len(line)-len(trimmed)]
		depth := max(measureDepth(leading, sourceUnit)+depthOffset, 0)
		result[i] = strings.Repeat(fileUnit, depth) + trimmed
	}
	return strings.Join(result, "\n")
}

// firstIndentDepth returns the indent depth of the first non-empty line.
func firstIndentDepth(lines []string, unit string) int {
	for _, l := range lines {
		trimmed := strings.TrimLeft(l, " \t")
		if trimmed == "" {
			continue
		}
		leading := l[:len(l)-len(trimmed)]
		return measureDepth(leading, unit)
	}
	return 0
}

// detectIndentUnit returns the indentation unit used by the given lines:
// "\t" for tab-indented files, or a string of N spaces for space-indented
// files. Returns "" if indentation cannot be determined.
func detectIndentUnit(lines []string) string {
	minSpaces := 0
	hasTabs := false
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		leading := line[:len(line)-len(trimmed)]
		if leading == "" {
			continue
		}
		if strings.Contains(leading, "\t") {
			hasTabs = true
			break
		}
		n := len(leading)
		if n > 0 && (minSpaces == 0 || n < minSpaces) {
			minSpaces = n
		}
	}
	if hasTabs {
		return "\t"
	}
	if minSpaces > 0 {
		return strings.Repeat(" ", minSpaces)
	}
	return ""
}

// measureDepth returns how many indent units deep the leading whitespace
// represents.
func measureDepth(leading, unit string) int {
	if unit == "" {
		return 0
	}
	if unit == "\t" {
		return strings.Count(leading, "\t")
	}
	spaces := strings.Count(leading, " ")
	return spaces / len(unit)
}

// diagnoseMismatch produces a diagnostic hint when old_string is not found
// in content and normalized matching also failed. It helps models
// self-correct by showing what the file actually contains near the best
// match.
func diagnoseMismatch(content, old string) string {
	contentLines := strings.Split(content, "\n")
	oldLines := strings.Split(old, "\n")

	if len(oldLines) == 0 {
		return ""
	}

	// Strategy 1: whitespace-normalized search.
	if hint := diagnoseWhitespaceMismatch(contentLines, oldLines); hint != "" {
		return hint
	}

	// Strategy 2: line-similarity search.
	if hint := diagnoseBestLineMatch(contentLines, oldLines); hint != "" {
		return hint
	}

	return ""
}

// normalizeWS collapses all whitespace runs to a single space.
func normalizeWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// diagnoseWhitespaceMismatch checks whether old matches content after
// whitespace normalization. If so, it reports the actual lines.
func diagnoseWhitespaceMismatch(contentLines, oldLines []string) string {
	matches := findNormalizedMatches(strings.Join(contentLines, "\n"), strings.Join(oldLines, "\n"))
	if len(matches) == 0 {
		return ""
	}
	return formatWhitespaceHint(contentLines, matches[0].startLine, matches[0].endLine)
}

// lineAtOffset returns the line index for a character offset in
// newline-joined lines.
func lineAtOffset(lines []string, offset int) int {
	pos := 0
	for i, line := range lines {
		if pos+len(line) >= offset {
			return i
		}
		pos += len(line) + 1 // +1 for the "\n" join separator.
	}
	return len(lines) - 1
}

func formatWhitespaceHint(contentLines []string, startLine, endLine int) string {
	var b strings.Builder
	b.WriteString("A whitespace-normalized match was found, so the text exists but with different whitespace (tabs vs spaces, or different indentation).\n")
	fmt.Fprintf(&b, "Actual file content (lines %d-%d):\n", startLine+1, endLine+1)
	for i := startLine; i <= endLine && i < len(contentLines); i++ {
		fmt.Fprintf(&b, "%6d|%s\n", i+1, visualizeWS(contentLines[i]))
	}
	b.WriteString("Use the exact whitespace shown above (→ = tab, · = space).")
	return b.String()
}

// diagnoseBestLineMatch finds the window of lines in contentLines that best
// matches oldLines (compared after trimming leading/trailing whitespace).
func diagnoseBestLineMatch(contentLines, oldLines []string) string {
	if len(oldLines) == 0 || len(contentLines) == 0 {
		return ""
	}

	// Trimmed old lines for comparison.
	trimmedOld := make([]string, len(oldLines))
	for i, l := range oldLines {
		trimmedOld[i] = strings.TrimSpace(l)
	}

	// Remove leading/trailing empty lines from the search pattern.
	for len(trimmedOld) > 0 && trimmedOld[0] == "" {
		trimmedOld = trimmedOld[1:]
	}
	for len(trimmedOld) > 0 && trimmedOld[len(trimmedOld)-1] == "" {
		trimmedOld = trimmedOld[:len(trimmedOld)-1]
	}
	if len(trimmedOld) == 0 {
		return ""
	}

	bestScore := 0
	bestStart := -1
	window := len(trimmedOld)

	for start := range len(contentLines) - window + 1 {
		score := 0
		for j := range window {
			candidate := strings.TrimSpace(contentLines[start+j])
			if candidate == trimmedOld[j] {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestStart = start
		}
	}

	// Require at least half the lines to match for a useful hint.
	if bestStart == -1 || bestScore < (window+1)/2 {
		return ""
	}

	endLine := bestStart + window - 1

	// Show a bit of extra context (1 line before and after).
	ctxStart := max(0, bestStart-1)
	ctxEnd := min(len(contentLines)-1, endLine+1)

	var b strings.Builder
	fmt.Fprintf(&b, "No exact match found. Closest match at lines %d-%d (%d/%d lines match after trimming whitespace):\n",
		bestStart+1, endLine+1, bestScore, window)
	for i := ctxStart; i <= ctxEnd; i++ {
		fmt.Fprintf(&b, "%6d|%s\n", i+1, visualizeWS(contentLines[i]))
	}
	b.WriteString("Use the exact text shown above (→ = tab, · = space).")
	return b.String()
}

// visualizeWS replaces tabs and leading spaces with visible markers so
// models can distinguish them. Interior spaces are left as-is to keep
// the output readable.
func visualizeWS(s string) string {
	s = strings.ReplaceAll(s, "\t", "→")
	trimmed := strings.TrimLeft(s, " ")
	leading := len(s) - len(trimmed)
	if leading > 0 {
		s = strings.Repeat("·", leading) + trimmed
	}
	return s
}
