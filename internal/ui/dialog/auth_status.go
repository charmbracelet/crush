package dialog

import (
	"charm.land/bubbles/v2/spinner"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// authSpinnerLine renders a sign-in progress row: the spinner followed by
// msg, sized to width and inset by one cell.
func authSpinnerLine(t *styles.Styles, sp spinner.Model, width int, msg string) string {
	status := t.Dialog.OAuth.StatusText
	return status.
		Width(width).
		Padding(0, 1).
		Render(t.Dialog.OAuth.Success.Render(sp.View()) + status.Render(msg))
}

// authSuccessBlock renders the sign-in confirmation, sized to width and
// padded by one cell on every side.
func authSuccessBlock(t *styles.Styles, width int) string {
	return t.Dialog.OAuth.Success.
		Width(width).
		Padding(1).
		Render("Authentication successful!")
}
