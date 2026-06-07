package model

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
)

// parentBreadcrumbLine renders a one-line breadcrumb indicating the parent
// session. It returns an empty string when title is empty. The dot is colored
// with the subagent's palette color. The rendered line fits within width
// terminal columns.
func parentBreadcrumbLine(t *styles.Styles, color, title string, width int) string {
	if title == "" {
		return ""
	}

	dot := styles.SubagentDot(color)
	prefix := " ↑ parent: "
	maxTitleWidth := max(width-lipgloss.Width(dot)-lipgloss.Width(prefix), 0)
	truncated := ansi.Truncate(title, maxTitleWidth, "…")

	return dot + t.Resource.AdditionalText.Render(prefix+truncated)
}
