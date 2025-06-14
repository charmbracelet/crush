package core

import (
	"image/color"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/crush/internal/exp/diffview"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

type ButtonOpts struct {
	Text           string
	UnderlineIndex int  // Index of character to underline (0-based)
	Selected       bool // Whether this button is selected
}

// SelectableButton creates a button with an underlined character and selection state
func SelectableButton(opts ButtonOpts) string {
	t := styles.CurrentTheme()

	// Base style for the button
	buttonStyle := t.S().Text

	// Apply selection styling
	if opts.Selected {
		buttonStyle = buttonStyle.Foreground(t.White).Background(t.Secondary)
	} else {
		buttonStyle = buttonStyle.Background(t.BgSubtle)
	}

	// Create the button text with underlined character
	text := opts.Text
	if opts.UnderlineIndex >= 0 && opts.UnderlineIndex < len(text) {
		before := text[:opts.UnderlineIndex]
		underlined := text[opts.UnderlineIndex : opts.UnderlineIndex+1]
		after := text[opts.UnderlineIndex+1:]

		message := buttonStyle.Render(before) +
			buttonStyle.Underline(true).Render(underlined) +
			buttonStyle.Render(after)

		return buttonStyle.Padding(0, 2).Render(message)
	}

	// Fallback if no underline index specified
	return buttonStyle.Padding(0, 2).Render(text)
}

// SelectableButtons creates a horizontal row of selectable buttons
func SelectableButtons(buttons []ButtonOpts, spacing string) string {
	if spacing == "" {
		spacing = "  "
	}

	var parts []string
	for i, button := range buttons {
		parts = append(parts, SelectableButton(button))
		if i < len(buttons)-1 {
			parts = append(parts, spacing)
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

func DiffFormatter() *diffview.DiffView {
	formatDiff := diffview.New()
	style := chroma.MustNewStyle("crush", styles.GetChromaTheme())
	diff := formatDiff.
		SyntaxHightlight(true).
		ChromaStyle(style)
	return diff
}
