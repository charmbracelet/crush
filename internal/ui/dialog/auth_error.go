package dialog

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// authErrorBlock renders a failed sign-in: an "Authentication failed."
// header and, when there is one, the reason below it, flattened onto one
// line and wrapped to width. Both are inset by one cell and framed by
// blank lines.
func authErrorBlock(t *styles.Styles, width int, reason string) string {
	header := t.Dialog.OAuth.ErrorText.
		Width(width).
		Padding(0, 1).
		Render("Authentication failed.")
	reason = strings.Join(strings.Fields(reason), " ")
	if reason == "" {
		return lipgloss.JoinVertical(lipgloss.Left, "", header, "")
	}
	detail := t.Dialog.OAuth.StatusText.
		Width(width).
		Padding(0, 1).
		Render(reason)
	return lipgloss.JoinVertical(lipgloss.Left, "", header, "", detail, "")
}
