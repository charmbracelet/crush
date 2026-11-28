package dialog

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// hangIndent puts lead in front of the first of lines and blanks as wide as
// lead in front of the rest, so text spanning several rows hangs under its
// prompt or marker instead of returning to the left edge.
func hangIndent(lead string, lines []string) []string {
	indent := strings.Repeat(" ", lipgloss.Width(lead))
	out := make([]string, len(lines))
	for i, line := range lines {
		if i == 0 {
			out[i] = lead + line
		} else {
			out[i] = indent + line
		}
	}
	return out
}

// wrapHanging word-wraps text so it fits width together with lead, renders
// each wrapped line with style, and hangs the result under lead. The lead
// itself stays unstyled.
func wrapHanging(text string, width int, lead string, style lipgloss.Style) []string {
	lines := wrapLines(text, width-lipgloss.Width(lead))
	for i, line := range lines {
		lines[i] = style.Render(line)
	}
	return hangIndent(lead, lines)
}

// wrapLines word-wraps plain text at width (hard-breaking words
// longer than a line) and returns its visual lines.
func wrapLines(text string, width int) []string {
	return strings.Split(ansi.Wrap(text, max(width, 1), ""), "\n")
}

// balancedWrap wraps s to at most width columns using the narrowest width
// that still needs the same number of lines, so a wrapped sentence splits
// into lines of similar length instead of leaving one word on its own.
func balancedWrap(s string, width int) string {
	wrapped := ansi.Wrap(s, width, "")
	lines := strings.Count(wrapped, "\n")
	if lines == 0 {
		return wrapped
	}
	for w := ansi.StringWidth(s)/(lines+1) + 1; w < width; w++ {
		if b := ansi.Wrap(s, w, ""); strings.Count(b, "\n") == lines {
			return b
		}
	}
	return wrapped
}

// blockHeight is the row count of s, counting an empty block as none.
func blockHeight(s string) int {
	if s == "" {
		return 0
	}
	return lipgloss.Height(s)
}
