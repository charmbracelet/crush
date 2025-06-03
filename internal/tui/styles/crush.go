package styles

import (
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

func NewCrushTheme() *Theme {
	return &Theme{
		Name:   "crush",
		IsDark: true,

		Primary:   charmtone.Charple,
		Secondary: charmtone.Dolly,
		Tertiary:  charmtone.Bok,
		Accent:    charmtone.Zest,

		Blue: charmtone.Malibu,

		// Backgrounds
		BgBase:    charmtone.Pepper,
		BgSubtle:  charmtone.Charcoal,
		BgOverlay: charmtone.Iron,

		// Foregrounds
		FgBase:     charmtone.Ash,
		FgMuted:    charmtone.Squid,
		FgSubtle:   charmtone.Oyster,
		FgSelected: charmtone.Salt,

		// Borders
		Border:      charmtone.Charcoal,
		BorderFocus: charmtone.Charple,

		// Status
		Success: charmtone.Guac,
		Error:   charmtone.Sriracha,
		Warning: charmtone.Uni,
		Info:    charmtone.Malibu,

		// TODO: fix this.
		SyntaxBg:      lipgloss.Color("#1C1C1F"),
		SyntaxKeyword: lipgloss.Color("#FF6DFE"),
		SyntaxString:  lipgloss.Color("#E8FE96"),
		SyntaxComment: lipgloss.Color("#6B6F85"),
	}
}
