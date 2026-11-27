package dialog

import (
	"strings"

	"charm.land/lipgloss/v2"
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
