package dialog

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// maxVisibleToggleRows bounds the toggles dialogs' viewports.
const maxVisibleToggleRows = 12

// minToggleDialogWidth is the comfortable minimum width so short names do
// not shrink the dialog and truncate the help line.
const minToggleDialogWidth = 68

// scrollbarColumnWidth is the columns the scrollbar column adds to the
// row area: one space before the track glyph.
const scrollbarColumnWidth = 2

// fitToggleRow fits name and status beside the mark, truncating status first but never below half the row.
func fitToggleRow(name, status string, rowWidth int) (string, string) {
	avail := max(0, rowWidth-2-1)
	nameWidth, statusWidth := ansi.StringWidth(name), ansi.StringWidth(status)
	if nameWidth+statusWidth <= avail {
		return name, status
	}
	status = ansi.Truncate(status, min(statusWidth, max(avail/2, avail-nameWidth)), "…")
	return ansi.Truncate(name, avail-ansi.StringWidth(status), "…"), status
}

// shrinkRowsToFit returns how many of visible rows fit areaHeight given the dialog's dialogHeight, keeping at least one.
func shrinkRowsToFit(dialogHeight, visible, areaHeight int) int {
	if over := dialogHeight - areaHeight; over > 0 {
		return max(1, visible-over)
	}
	return visible
}

// toggleVisibleOffset returns the first visible row of a window that keeps the cursor in view.
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

// toggleWindow is a toggles dialog's cursor and scrolling window.
type toggleWindow struct {
	cursor int
	// offset is the first visible row; the window follows the cursor.
	offset int
	// maxRows is the visible row cap fitted to the screen in fit.
	maxRows int
}

// cursorUp moves the cursor up one of count rows, keeping it in view.
func (w *toggleWindow) cursorUp(count int) {
	w.cursor = max(0, w.cursor-1)
	w.offset = toggleVisibleOffset(w.cursor, w.offset, count, w.maxRows)
}

// cursorDown moves the cursor down one of count rows, keeping it in view.
func (w *toggleWindow) cursorDown(count int) {
	w.cursor = min(count-1, w.cursor+1)
	w.offset = toggleVisibleOffset(w.cursor, w.offset, count, w.maxRows)
}

// span returns the first visible row and the visible row count over count rows.
func (w *toggleWindow) span(count int) (first, visible int) {
	visible = min(w.maxRows, count)
	return max(0, min(w.offset, count-visible)), visible
}

// fit renders with render, re-rendering with fewer rows when it is taller than areaHeight.
func (w *toggleWindow) fit(count, areaHeight int, render func() string) string {
	w.maxRows = maxVisibleToggleRows
	content := render()
	if rows := shrinkRowsToFit(lipgloss.Height(content), min(w.maxRows, count), areaHeight); count > 0 && rows < min(w.maxRows, count) {
		w.maxRows = rows
		w.offset = toggleVisibleOffset(w.cursor, w.offset, count, rows)
		content = render()
	}
	return content
}

// joinToggleRows stacks rows between blank lines, with a scrollbar column when they overflow.
func joinToggleRows(t *styles.Styles, rows []string, contentSize, visible, offset int) string {
	if len(rows) == 0 {
		return ""
	}
	// Pair lines one by one so each track glyph stays on its row.
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

// toggleEmptyState renders msg between the same blank lines as a populated list.
func toggleEmptyState(t *styles.Styles, msg string, innerWidth int) string {
	return lipgloss.JoinVertical(lipgloss.Left, "", dialogEmptyState(t, msg, innerWidth), "")
}
