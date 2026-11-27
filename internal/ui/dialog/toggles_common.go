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

// truncateToggleName truncates a row's name so the 2-column mark, the
// name, at least one space and a status of statusWidth fit rowWidth on one
// line; an untruncated long name wraps inside the dialog frame.
func truncateToggleName(name string, rowWidth, statusWidth int) string {
	return ansi.Truncate(name, max(0, rowWidth-2-1-statusWidth), "…")
}

// fitToggleRow shares a row's text width between a name and its status
// after the 2-column mark and a 1-column gap. Both keep their full width
// when they fit; otherwise the status gives way first, but never below
// half the row, so a long error cannot push the name out entirely.
func fitToggleRow(name, status string, rowWidth int) (string, string) {
	avail := max(0, rowWidth-2-1)
	nameWidth, statusWidth := ansi.StringWidth(name), ansi.StringWidth(status)
	if nameWidth+statusWidth <= avail {
		return name, status
	}
	status = ansi.Truncate(status, min(statusWidth, max(avail/2, avail-nameWidth)), "…")
	return ansi.Truncate(name, avail-ansi.StringWidth(status), "…"), status
}

// fitToggleRows returns how many rows of a toggles dialog can show in an
// area of areaHeight, given the dialog's height when it shows visible rows.
// The dialog never grows past maxVisibleToggleRows, and always keeps one
// row so the cursor stays visible.
func fitToggleRows(dialogHeight, visible, areaHeight int) int {
	if over := dialogHeight - areaHeight; over > 0 {
		return max(1, visible-over)
	}
	return visible
}

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

// toggleWindow is the scrolling window shared by the toggles dialogs: the
// cursor row, the first visible row, and the visible row cap fitted to the
// screen.
type toggleWindow struct {
	cursor int
	// offset is the first visible row; the window follows the cursor.
	offset int
	// maxRows is the visible row cap fitted to the screen in fit; zero
	// means maxVisibleToggleRows.
	maxRows int
}

// visibleRows returns the cap on visible rows.
func (w *toggleWindow) visibleRows() int {
	if w.maxRows > 0 {
		return w.maxRows
	}
	return maxVisibleToggleRows
}

// cursorUp moves the cursor up one of count rows, scrolling the window to
// keep it in view.
func (w *toggleWindow) cursorUp(count int) {
	w.cursor = max(0, w.cursor-1)
	w.offset = toggleVisibleOffset(w.cursor, w.offset, count, w.visibleRows())
}

// cursorDown moves the cursor down one of count rows, scrolling the window
// to keep it in view.
func (w *toggleWindow) cursorDown(count int) {
	w.cursor = min(count-1, w.cursor+1)
	w.offset = toggleVisibleOffset(w.cursor, w.offset, count, w.visibleRows())
}

// span returns the first visible row and the number of visible rows of a
// window over count rows.
func (w *toggleWindow) span(count int) (first, visible int) {
	visible = min(w.visibleRows(), count)
	return max(0, min(w.offset, count-visible)), visible
}

// fit renders the dialog with render and, when it is taller than
// areaHeight, shrinks the visible rows to fit the screen, keeping the
// cursor in the window, and renders it again.
func (w *toggleWindow) fit(count, areaHeight int, render func() string) string {
	w.maxRows = maxVisibleToggleRows
	content := render()
	if rows := fitToggleRows(lipgloss.Height(content), min(w.maxRows, count), areaHeight); count > 0 && rows < min(w.maxRows, count) {
		w.maxRows = rows
		w.offset = toggleVisibleOffset(w.cursor, w.offset, count, rows)
		content = render()
	}
	return content
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

// toggleEmptyState renders a toggles dialog's empty message with the same
// blank spacer lines above and below as a populated list, so it does not
// crowd the title or the help line.
func toggleEmptyState(t *styles.Styles, msg string, innerWidth int) string {
	return lipgloss.JoinVertical(lipgloss.Left, "", dialogEmptyState(t, msg, innerWidth), "")
}
