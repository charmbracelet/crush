package dialog

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// maxVisibleToggleRows bounds the toggles dialogs' viewports.
const maxVisibleToggleRows = 12

// minToggleDialogWidth is the comfortable minimum width so short names do
// not shrink the dialog and truncate the help line.
const minToggleDialogWidth = 68

// scrollbarColumnWidth is the columns the scrollbar column adds to the
// row area: one space before the track glyph.
const scrollbarColumnWidth = 2

// toggleVisibleOffset returns the first visible row index for a window of
// visible rows over count items, scrolling so the cursor always stays in
// view.
func toggleVisibleOffset(cursor, offset, count, visible int) int {
	offset = min(offset, max(0, count-visible))
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+visible {
		offset = cursor - visible + 1
	}
	return max(0, offset)
}

// joinToggleRows stacks the rendered rows and, when the content overflows
// the viewport, appends a scrollbar column like the sessions and models
// pickers.
func joinToggleRows(t *styles.Styles, rows []string, contentSize, visible, offset int) string {
	if len(rows) == 0 {
		return ""
	}
	// Compose line by line instead of a block-level JoinHorizontal: the
	// scrollbar string splits into exactly len(rows) lines, so pairing
	// them keeps every row's track glyph on the same line.
	scrollLines := strings.Split(common.Scrollbar(t, len(rows), contentSize, visible, offset), "\n")
	var b strings.Builder
	for i, row := range rows {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(row)
		if i < len(scrollLines) {
			b.WriteString(" ")
			b.WriteString(scrollLines[i])
		}
	}
	// Blank spacer lines above and below, matching the unscrolled layout.
	return lipgloss.JoinVertical(lipgloss.Left, "", b.String(), "")
}
