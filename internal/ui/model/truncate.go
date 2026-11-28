package model

import (
	"fmt"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// truncateRows keeps at most maxItems rows, spending the last slot on an
// "…and N more" hint when they do not all fit. Narrow columns get a terse
// "+N" so the hint never wraps onto a row it was not budgeted.
func truncateRows(rows []string, maxItems, width int, hint lipgloss.Style) []string {
	if maxItems <= 0 {
		return nil
	}
	if len(rows) <= maxItems {
		return rows
	}
	remaining := len(rows) - (maxItems - 1)
	text := fmt.Sprintf("…and %d more", remaining)
	if ansi.StringWidth(text) > width {
		text = ansi.Truncate(fmt.Sprintf("+%d", remaining), width, "")
	}
	return append(rows[:maxItems-1:maxItems-1], hint.Render(text))
}
