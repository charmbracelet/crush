package styles

import (
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

func NewCharmtoneLightTheme() *Theme {
	t := &Theme{
		Name:   "charmtone-light",
		IsDark: false,

		Primary:   charmtone.Charple,
		Secondary: charmtone.Malibu,
		Tertiary:  charmtone.Julep,
		Accent:    charmtone.Coral,

		// Backgrounds - Light theme uses light colors
		BgBase:        charmtone.Butter, // Very light cream
		BgBaseLighter: charmtone.Salt,   // Almost white
		BgSubtle:      charmtone.Ash,    // Light gray
		BgOverlay:     charmtone.Smoke,  // Slightly darker gray

		// Foregrounds - Light theme uses dark colors for better contrast
		FgBase:      charmtone.Pepper,   // Very dark for main text (#201F26)
		FgMuted:     charmtone.Iron,     // Medium gray for muted text (#4D4C57)
		FgHalfMuted: charmtone.BBQ,      // Dark for half-muted text (#2d2c35)
		FgSubtle:    charmtone.Charcoal, // Medium dark for subtle text (#3A3943)
		FgSelected:  charmtone.Butter,   // Light text for selected items

		// Borders
		Border:      charmtone.Oyster,  // Light border
		BorderFocus: charmtone.Charple, // Purple focus border

		// Status colors - Keep vibrant for visibility
		Success: charmtone.Guac,    // Green
		Error:   charmtone.Coral,   // Red
		Warning: charmtone.Mustard, // Yellow
		Info:    charmtone.Malibu,  // Blue

		// Colors - Keep the same vibrant colors
		White: charmtone.Butter,

		BlueLight: charmtone.Malibu,
		Blue:      charmtone.Sapphire,

		Yellow: charmtone.Citron,
		Citron: charmtone.Zest,

		Green:      charmtone.Guac,
		GreenDark:  charmtone.Turtle,
		GreenLight: charmtone.Julep,

		Red:      charmtone.Coral,
		RedDark:  charmtone.Cherry,
		RedLight: charmtone.Salmon,
		Cherry:   charmtone.Cheeky,

		// Markdown colors
		Markdown: MarkdownColors{
			Document:     charmtone.Pepper,   // Dark text on light bg
			Heading:      charmtone.Sapphire, // Darker blue for contrast
			H1Bg:         charmtone.Hazy,     // Light purple background
			H1Fg:         charmtone.Pepper,   // Dark text
			H6:           charmtone.Turtle,   // Darker green
			Rule:         charmtone.Ash,      // Medium gray
			BlockQuote:   nil,                // Use theme default
			Item:         nil,                // Use theme default
			Enumeration:  nil,                // Use theme default
			TaskTicked:   nil,                // Use theme default
			TaskUnticked: nil,                // Use theme default
			Link:         charmtone.Sapphire, // Blue for links
			LinkText:     charmtone.Turtle,   // Dark green
			Code:         charmtone.Cherry,   // Dark red
			CodeBg:       charmtone.Ash,      // Light gray background
			CodeBlock:    charmtone.Ash,      // Light gray background

			Chroma: ChromaColors{
				Text:                charmtone.Pepper,
				Error:               charmtone.Cherry,
				Comment:             charmtone.Iron,
				CommentPreproc:      charmtone.Squid,
				Keyword:             charmtone.Sapphire,
				KeywordReserved:     charmtone.Citron,
				KeywordNamespace:    charmtone.Citron,
				KeywordType:         charmtone.Turtle,
				Operator:            charmtone.Salmon,
				Punctuation:         charmtone.Charcoal,
				Name:                charmtone.Pepper,
				NameBuiltin:         charmtone.Cheeky,
				NameTag:             charmtone.Mauve,
				NameAttribute:       charmtone.Hazy,
				NameClass:           charmtone.Charcoal,
				NameDecorator:       charmtone.Citron,
				NameFunction:        charmtone.Turtle,
				LiteralNumber:       charmtone.Julep,
				LiteralString:       charmtone.Cumin,
				LiteralStringEscape: charmtone.Bok,
				GenericDeleted:      charmtone.Cherry,
				GenericInserted:     charmtone.Turtle,
				GenericSubheading:   charmtone.Squid,
				Background:          charmtone.Ash,
			},
		},
	}

	// Text selection - Dark text on light purple background
	t.TextSelection = lipgloss.NewStyle().Foreground(charmtone.Charcoal).Background(charmtone.Hazy)

	// LSP and MCP status - Adjust for light theme with better contrast
	t.ItemOfflineIcon = lipgloss.NewStyle().Foreground(charmtone.Charcoal).SetString("●")
	t.ItemBusyIcon = t.ItemOfflineIcon.Foreground(charmtone.Mustard)
	t.ItemErrorIcon = t.ItemOfflineIcon.Foreground(charmtone.Coral)
	t.ItemOnlineIcon = t.ItemOfflineIcon.Foreground(charmtone.Guac)

	// Yolo mode indicators - Adjusted for light theme with better contrast
	t.YoloIconFocused = lipgloss.NewStyle().Foreground(charmtone.Pepper).Background(charmtone.Citron).Bold(true).SetString(" ! ")
	t.YoloIconBlurred = t.YoloIconFocused.Foreground(charmtone.Squid).Background(charmtone.Ash)
	t.YoloDotsFocused = lipgloss.NewStyle().Foreground(charmtone.Mustard).SetString(":::")
	t.YoloDotsBlurred = t.YoloDotsFocused.Foreground(charmtone.Charcoal)

	return t
}
