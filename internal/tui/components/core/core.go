package core

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/crush/internal/tui/exp/diffview"
	"github.com/charmbracelet/crush/internal/tui/styles"
	"github.com/charmbracelet/x/ansi"
)

type KeyMapHelp interface {
	Help() help.KeyMap
}

type simpleHelp struct {
	shortList []key.Binding
	fullList  [][]key.Binding
}

func NewSimpleHelp(shortList []key.Binding, fullList [][]key.Binding) help.KeyMap {
	return &simpleHelp{
		shortList: shortList,
		fullList:  fullList,
	}
}

// FullHelp implements help.KeyMap.
func (s *simpleHelp) FullHelp() [][]key.Binding {
	return s.fullList
}

// ShortHelp implements help.KeyMap.
func (s *simpleHelp) ShortHelp() []key.Binding {
	return s.shortList
}

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

func SectionWithInfo(text string, width int, info string) string {
	t := styles.CurrentTheme()
	char := "─"
	length := lipgloss.Width(text) + 1
	remainingWidth := width - length

	if info != "" {
		remainingWidth -= lipgloss.Width(info) + 1 // 1 for the space before info
	}
	lineStyle := t.S().Base.Foreground(t.Border)
	if remainingWidth > 0 {
		text = text + " " + lineStyle.Render(strings.Repeat(char, remainingWidth)) + " " + info
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
	Icon             string // if empty no icon will be shown
	Title            string
	TitleColor       color.Color
	Description      string
	DescriptionColor color.Color
	ExtraContent     string // additional content to append after the description
}

func Status(opts StatusOpts, width int) string {
	t := styles.CurrentTheme()
	icon := opts.Icon
	title := opts.Title
	titleColor := t.FgMuted
	if opts.TitleColor != nil {
		titleColor = opts.TitleColor
	}
	description := opts.Description
	descriptionColor := t.FgSubtle
	if opts.DescriptionColor != nil {
		descriptionColor = opts.DescriptionColor
	}
	title = t.S().Base.Foreground(titleColor).Render(title)
	if description != "" {
		extraContentWidth := lipgloss.Width(opts.ExtraContent)
		if extraContentWidth > 0 {
			extraContentWidth += 1
		}
		description = ansi.Truncate(description, width-lipgloss.Width(icon)-lipgloss.Width(title)-2-extraContentWidth, "…")
		description = t.S().Base.Foreground(descriptionColor).Render(description)
	}

	content := []string{}
	if icon != "" {
		content = append(content, icon)
	}
	content = append(content, title)
	if description != "" {
		content = append(content, description)
	}
	if opts.ExtraContent != "" {
		content = append(content, opts.ExtraContent)
	}

	return strings.Join(content, " ")
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

// SelectableButtonsVertical creates a vertical row of selectable buttons
func SelectableButtonsVertical(buttons []ButtonOpts, spacing int) string {
	var parts []string
	for i, button := range buttons {
		parts = append(parts, SelectableButton(button))
		if i < len(buttons)-1 {
			for range spacing {
				parts = append(parts, "")
			}
		}
	}

	return lipgloss.JoinVertical(lipgloss.Center, parts...)
}

func DiffFormatter() *diffview.DiffView {
	t := styles.CurrentTheme()
	formatDiff := diffview.New()
	style := chroma.MustNewStyle("crush", styles.GetChromaTheme())
	diff := formatDiff.ChromaStyle(style).Style(t.S().Diff).TabWidth(4)
	return diff
}
