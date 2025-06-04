package core

import (
	"image/color"
	"strings"

	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/opencode-ai/opencode/internal/tui/styles"
)

func Section(text string, width int) string {
	t := styles.CurrentTheme()
	char := "─"
	length := lipgloss.Width(text) + 1
	remainingWidth := width - length
	lineStyle := t.S().Base.Foreground(t.Border)
	if remainingWidth > 0 {
		text = text + " " + lineStyle.Render(strings.Repeat(char, remainingWidth))
	}
	return text
}

func Title(title string, width int) string {
	t := styles.CurrentTheme()
	char := "╱"
	length := lipgloss.Width(title) + 1
	remainingWidth := width - length
	titleStyle := t.S().Base.Foreground(t.Primary)
	if remainingWidth > 0 {
		lines := strings.Repeat(char, remainingWidth)
		lines = styles.ApplyForegroundGrad(lines, t.Primary, t.Secondary)
		title = titleStyle.Render(title) + " " + lines
	}
	return title
}

type StatusOpts struct {
	Icon             string
	IconColor        color.Color
	Title            string
	TitleColor       color.Color
	Description      string
	DescriptionColor color.Color
}

func Status(ops StatusOpts, width int) string {
	t := styles.CurrentTheme()
	icon := "●"
	iconColor := t.Success
	if ops.Icon != "" {
		icon = ops.Icon
	}
	if ops.IconColor != nil {
		iconColor = ops.IconColor
	}
	title := ops.Title
	titleColor := t.FgMuted
	if ops.TitleColor != nil {
		titleColor = ops.TitleColor
	}
	description := ops.Description
	descriptionColor := t.FgSubtle
	if ops.DescriptionColor != nil {
		descriptionColor = ops.DescriptionColor
	}
	icon = t.S().Base.Foreground(iconColor).Render(icon)
	title = t.S().Base.Foreground(titleColor).Render(title)
	if description != "" {
		description = ansi.Truncate(description, width-lipgloss.Width(icon)-lipgloss.Width(title)-2, "…")
	}
	description = t.S().Base.Foreground(descriptionColor).Render(description)
	return strings.Join([]string{
		icon,
		title,
		description,
	}, " ")
}
