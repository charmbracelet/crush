package styles

import (
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

func NewCharmtoneTheme() *Theme {
	t := &Theme{
		Name:   "charmtone",
		IsDark: true,

		Primary:   charmtone.Charple,
		Secondary: charmtone.Dolly,
		Tertiary:  charmtone.Bok,
		Accent:    charmtone.Zest,

		// Backgrounds
		BgBase:        charmtone.Pepper,
		BgBaseLighter: charmtone.BBQ,
		BgSubtle:      charmtone.Charcoal,
		BgOverlay:     charmtone.Iron,

		// Foregrounds
		FgBase:      charmtone.Ash,
		FgMuted:     charmtone.Squid,
		FgHalfMuted: charmtone.Smoke,
		FgSubtle:    charmtone.Oyster,
		FgSelected:  charmtone.Salt,

		// Borders
		Border:      charmtone.Charcoal,
		BorderFocus: charmtone.Charple,

		// Status
		Success: charmtone.Guac,
		Error:   charmtone.Sriracha,
		Warning: charmtone.Zest,
		Info:    charmtone.Malibu,

		// Colors
		White: charmtone.Butter,

		BlueLight: charmtone.Sardine,
		Blue:      charmtone.Malibu,

		Yellow: charmtone.Mustard,
		Citron: charmtone.Citron,

		Green:      charmtone.Julep,
		GreenDark:  charmtone.Guac,
		GreenLight: charmtone.Bok,

		Red:      charmtone.Coral,
		RedDark:  charmtone.Sriracha,
		RedLight: charmtone.Salmon,
		Cherry:   charmtone.Cherry,

		// Markdown colors
		Markdown: MarkdownColors{
			// Basic elements - map current hardcoded colors
			Document:     charmtone.Smoke,
			Heading:      charmtone.Malibu,
			H1Bg:         charmtone.Charple,
			H1Fg:         charmtone.Zest,
			H6:           charmtone.Guac,
			Rule:         charmtone.Charcoal,
			BlockQuote:   nil, // Use theme default
			Item:         nil, // Use theme default
			Enumeration:  nil, // Use theme default
			TaskTicked:   nil, // Use theme default
			TaskUnticked: nil, // Use theme default
			Link:         charmtone.Zinc,
			LinkText:     charmtone.Guac,
			Image:        charmtone.Cheeky,
			ImageText:    charmtone.Squid,
			Code:         charmtone.Coral,
			CodeBg:       charmtone.Charcoal,
			CodeBlock:    charmtone.Charcoal,

			Chroma: ChromaColors{
				Text:                charmtone.Smoke,
				Error:               charmtone.Butter, // with charmtone.Sriracha bg
				Comment:             charmtone.Oyster,
				CommentPreproc:      charmtone.Bengal,
				Keyword:             charmtone.Malibu,
				KeywordReserved:     charmtone.Pony,
				KeywordNamespace:    charmtone.Pony,
				KeywordType:         charmtone.Guppy,
				Operator:            charmtone.Salmon,
				Punctuation:         charmtone.Zest,
				Name:                charmtone.Smoke,
				NameBuiltin:         charmtone.Cheeky,
				NameTag:             charmtone.Mauve,
				NameAttribute:       charmtone.Hazy,
				NameClass:           charmtone.Salt,
				NameDecorator:       charmtone.Citron,
				NameFunction:        charmtone.Guac,
				LiteralNumber:       charmtone.Julep,
				LiteralString:       charmtone.Cumin,
				LiteralStringEscape: charmtone.Bok,
				GenericDeleted:      charmtone.Coral,
				GenericInserted:     charmtone.Guac,
				GenericSubheading:   charmtone.Squid,
				Background:          charmtone.Charcoal,
			},
		},
	}

	// Text selection.
	t.TextSelection = lipgloss.NewStyle().Foreground(charmtone.Salt).Background(charmtone.Charple)

	// LSP and MCP status.
	t.ItemOfflineIcon = lipgloss.NewStyle().Foreground(charmtone.Squid).SetString("●")
	t.ItemBusyIcon = t.ItemOfflineIcon.Foreground(charmtone.Citron)
	t.ItemErrorIcon = t.ItemOfflineIcon.Foreground(charmtone.Coral)
	t.ItemOnlineIcon = t.ItemOfflineIcon.Foreground(charmtone.Guac)

	t.YoloIconFocused = lipgloss.NewStyle().Foreground(charmtone.Oyster).Background(charmtone.Citron).Bold(true).SetString(" ! ")
	t.YoloIconBlurred = t.YoloIconFocused.Foreground(charmtone.Pepper).Background(charmtone.Squid)
	t.YoloDotsFocused = lipgloss.NewStyle().Foreground(charmtone.Zest).SetString(":::")
	t.YoloDotsBlurred = t.YoloDotsFocused.Foreground(charmtone.Squid)

	return t
}
