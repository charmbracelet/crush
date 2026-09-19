package plan

import (
	"strings"
	"time"
	"unicode"
)

// TimestampLayout is the layout used for the timestamp prefix of plan file
// names. It sorts lexicographically in chronological order.
const TimestampLayout = "2006-01-02-150405"

// maxSlugLength caps the slug derived from a plan title so file names stay
// readable in a terminal and well under typical path length limits.
const maxSlugLength = 60

// Filename builds a plan file name from its markdown text and the time it
// was saved: a chronological timestamp followed by a slug of the plan title,
// e.g. "2026-09-19-153045-fix-login-timeout.md". Plans without a recognizable
// title fall back to "plan".
func Filename(text string, at time.Time) string {
	return at.Format(TimestampLayout) + "-" + slug(Title(text)) + ".md"
}

// Title returns the first top-level markdown heading in text, trimmed of
// its leading hashes. It skips blank lines, plan marker lines, and fenced
// code blocks, so a heading quoted inside an example does not win. Returns
// an empty string when no heading is found.
func Title(text string) string {
	inFence := false
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if isCodeFenceLine(trimmed) {
			inFence = !inFence
			continue
		}
		if inFence || trimmed == "" {
			continue
		}
		if markerLine(trimmed, StartMarker) || markerLine(trimmed, ReadyMarker) {
			continue
		}
		if title, ok := headingText(trimmed); ok {
			return title
		}
	}
	return ""
}

// headingText extracts the text of a top-level ATX heading line, reporting
// false for any other line.
func headingText(line string) (string, bool) {
	if !strings.HasPrefix(line, "# ") {
		return "", false
	}
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "#")), "#")), true
}

// slug normalizes a title into a file-name-safe token: lowercased, with every
// run of non-alphanumeric characters collapsed into a single hyphen. It falls
// back to "plan" when nothing usable is left.
func slug(title string) string {
	var b strings.Builder
	pendingSep := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if pendingSep && b.Len() > 0 {
				b.WriteRune('-')
			}
			pendingSep = false
			b.WriteRune(r)
			if b.Len() >= maxSlugLength {
				break
			}
			continue
		}
		pendingSep = true
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "plan"
	}
	return result
}
