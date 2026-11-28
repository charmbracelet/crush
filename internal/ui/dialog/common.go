package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// clampDialogLines keeps at most n lines of the rendered block s, so a
// free-wrapping description cannot push the rest of a dialog off-screen.
// When lines are dropped, the last kept line ends in an ellipsis right
// after its text. n <= 0 returns "".
func clampDialogLines(s string, n int) string {
	if n <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	lines = lines[:n]
	last := lines[n-1]
	text := ansi.StringWidth(strings.TrimRight(ansi.Strip(last), " "))
	lines[n-1] = ansi.Truncate(last, min(text, ansi.StringWidth(last)-1), "") + "…"
	return strings.Join(lines, "\n")
}

// InputCursor adjusts the cursor position for an input field within a dialog.
func InputCursor(t *styles.Styles, cur *tea.Cursor) *tea.Cursor {
	if cur != nil {
		titleStyle := t.Dialog.Title
		dialogStyle := t.Dialog.View
		inputStyle := t.Dialog.InputPrompt
		// Adjust cursor position to account for dialog layout
		cur.X += inputStyle.GetBorderLeftSize() +
			inputStyle.GetMarginLeft() +
			inputStyle.GetPaddingLeft() +
			dialogStyle.GetBorderLeftSize() +
			dialogStyle.GetPaddingLeft() +
			dialogStyle.GetMarginLeft()
		cur.Y += titleStyle.GetVerticalFrameSize() +
			inputStyle.GetBorderTopSize() +
			inputStyle.GetMarginTop() +
			inputStyle.GetPaddingTop() +
			inputStyle.GetBorderBottomSize() +
			inputStyle.GetMarginBottom() +
			inputStyle.GetPaddingBottom() +
			dialogStyle.GetPaddingTop() +
			dialogStyle.GetMarginTop() +
			dialogStyle.GetBorderTopSize()
	}
	return cur
}

// adjustOnboardingInputCursor removes the dialog view frame offset from an
// input cursor. Onboarding dialogs render without Dialog.View frame, while
// InputCursor includes that frame offset for regular dialogs.
func adjustOnboardingInputCursor(t *styles.Styles, cur *tea.Cursor) *tea.Cursor {
	if cur == nil {
		return nil
	}

	dialogStyle := t.Dialog.View
	cur.X -= dialogStyle.GetBorderLeftSize() +
		dialogStyle.GetPaddingLeft() +
		dialogStyle.GetMarginLeft()
	cur.Y -= dialogStyle.GetBorderTopSize() +
		dialogStyle.GetPaddingTop() +
		dialogStyle.GetMarginTop()
	return cur
}

// renameCursorOffset adjusts a cursor returned by an inline list item's
// text input so it lands at the correct screen position inside a dialog
// that is showing a rename prompt. The math accounts for the dialog
// border/padding/margin, the input prompt frame, and the height of the
// confirmation message rendered above the list. visibleStart and
// visibleEnd are the list's VisibleItemIndices; selectedIndex is the
// list's Selected index. The cursor Y is advanced one row per item
// between visibleStart and selectedIndex.
func renameCursorOffset(
	t *styles.Styles,
	cur *tea.Cursor,
	messageHeight int,
	visibleStart, visibleEnd, selectedIndex int,
) *tea.Cursor {
	if cur == nil {
		return nil
	}
	titleStyle := t.Dialog.Sessions.RenamingingTitle
	dialogStyle := t.Dialog.Sessions.RenamingView
	inputStyle := t.Dialog.InputPrompt

	cur.X += inputStyle.GetBorderLeftSize() +
		inputStyle.GetMarginLeft() +
		inputStyle.GetPaddingLeft() +
		dialogStyle.GetBorderLeftSize() +
		dialogStyle.GetPaddingLeft() +
		dialogStyle.GetMarginLeft()
	cur.Y += titleStyle.GetVerticalFrameSize() +
		inputStyle.GetBorderTopSize() +
		inputStyle.GetMarginTop() +
		inputStyle.GetPaddingTop() +
		inputStyle.GetBorderBottomSize() +
		inputStyle.GetMarginBottom() +
		inputStyle.GetPaddingBottom() +
		dialogStyle.GetPaddingTop() +
		dialogStyle.GetBorderTopSize() +
		messageHeight - 1

	for i := visibleStart; i <= visibleEnd && i != selectedIndex && selectedIndex > -1; i++ {
		cur.Y++
	}
	return cur
}
