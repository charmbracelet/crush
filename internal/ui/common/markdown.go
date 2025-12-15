package common

import (
	"charm.land/glamour/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
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

// PlainMarkdownRenderer returns a glamour [glamour.TermRenderer] with muted
// colors on a subtle background, for thinking content.
func PlainMarkdownRenderer(t *styles.Styles, width int) *glamour.TermRenderer {
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(t.PlainMarkdown),
		glamour.WithWordWrap(width),
	)
	return r
}
