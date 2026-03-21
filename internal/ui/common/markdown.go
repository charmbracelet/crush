package common

import (
	"charm.land/glamour/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// MarkdownRenderer returns a glamour [glamour.TermRenderer] configured with
// the given styles and width.
func MarkdownRenderer(sty *styles.Styles, width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(sty.Markdown),
		glamour.WithWordWrap(width),
	)
	return r
}

// PlainMarkdownRenderer returns a glamour [glamour.TermRenderer] with no colors
// (plain text with structure) and the given width.
func PlainMarkdownRenderer(sty *styles.Styles, width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(sty.PlainMarkdown),
		glamour.WithWordWrap(width),
	)
	return r
}
