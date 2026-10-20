package model

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

// truncateRows keeps at most maxItems rows, spending the last slot on an
// "…and N more" hint when they do not all fit.
func truncateRows(rows []string, maxItems int, hint lipgloss.Style) []string {
	if maxItems <= 0 {
		return nil
	}
	if len(rows) <= maxItems {
		return rows
	}
	remaining := len(rows) - (maxItems - 1)
	return append(rows[:maxItems-1:maxItems-1], hint.Render(fmt.Sprintf("…and %d more", remaining)))
}
