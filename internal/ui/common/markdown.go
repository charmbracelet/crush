package common

import (
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/glamour/v2"
	gstyles "github.com/charmbracelet/glamour/v2/styles"
)

// MarkdownRenderer returns a glamour [glamour.TermRenderer] configured with
// the given styles and width.
func MarkdownRenderer(t *styles.Styles, width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(t.Markdown),
		glamour.WithWordWrap(width),
	)
	return r
}

// PlainMarkdownRenderer returns a glamour [glamour.TermRenderer] with no colors
// (plain text with structure) and the given width.
func PlainMarkdownRenderer(width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(gstyles.ASCIIStyleConfig),
		glamour.WithWordWrap(width),
	)
	return r
}
