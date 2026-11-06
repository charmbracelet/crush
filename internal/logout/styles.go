package logout

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// buttonPadding is the horizontal padding inside a button.
const buttonPadding = 5

var (
	questionStyle = lipgloss.NewStyle().
			Bold(true)

	buttonFocusedStyle = lipgloss.NewStyle().
				Foreground(charmtone.Sash).
				Background(charmtone.Blush)
	buttonBlurredStyle = lipgloss.NewStyle().
				Foreground(charmtone.Soda).
				Background(charmtone.BBQ)

	choiceSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(charmtone.Blush)
)

// renderButton renders a selectable button with an underlined accelerator
// key (its first letter).
func renderButton(text string, selected bool) string {
	base := buttonBlurredStyle
	if selected {
		base = buttonFocusedStyle
	}
	rendered := base.Padding(0, buttonPadding).Render(text)
	if text != "" {
		// Underline the accelerator key (the first letter). The range style
		// must not carry padding, as StyleRanges re-renders the matched text.
		rendered = lipgloss.StyleRanges(rendered,
			lipgloss.NewRange(buttonPadding, buttonPadding+1, base.Underline(true)))
	}
	return rendered
}
