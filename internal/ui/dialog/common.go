package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

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

// HeaderInputListHelpView generates a view for dialogs with a header, input,
// list, and help sections.
func HeaderInputListHelpView(t *styles.Styles, width, listHeight int, header, input, list, help string) string {
	titleStyle := t.Dialog.Title
	helpStyle := t.Dialog.HelpView
	dialogStyle := t.Dialog.View.Width(width)
	inputStyle := t.Dialog.InputPrompt
	helpStyle = helpStyle.Width(width - dialogStyle.GetHorizontalFrameSize())
	listStyle := t.Dialog.List.Height(listHeight)
	listContent := listStyle.Render(list)

	content := strings.Join([]string{
		titleStyle.Render(header),
		inputStyle.Render(input),
		listContent,
		helpStyle.Render(help),
	}, "\n")

	return dialogStyle.Render(content)
}
