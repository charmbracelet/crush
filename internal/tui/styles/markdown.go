package styles

import (
	"github.com/charmbracelet/glamour/v2"
)

// Helper functions for style pointers
func boolPtr(b bool) *bool       { return &b }
func stringPtr(s string) *string { return &s }
func uintPtr(u uint) *uint       { return &u }

// returns a glamour TermRenderer configured with the current theme
func GetMarkdownRenderer(width int) *glamour.TermRenderer {
	t := CurrentTheme()
	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(t.S().Markdown),
		glamour.WithWordWrap(width),
	)
	return r
}
