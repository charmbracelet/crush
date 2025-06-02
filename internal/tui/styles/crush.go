package styles

import (
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

func NewCrushTheme() *Theme {
	return &Theme{
		Name:   "crush",
		IsDark: true,

		Primary:   lipgloss.Color(charmtone.Charple.Hex()),
		Secondary: lipgloss.Color(charmtone.Dolly.Hex()),
		Tertiary:  lipgloss.Color(charmtone.Bok.Hex()),
		Accent:    lipgloss.Color(charmtone.Zest.Hex()),

		Blue: lipgloss.Color(charmtone.Malibu.Hex()),

		// Backgrounds
		BgBase:    lipgloss.Color(charmtone.Pepper.Hex()),
		BgSubtle:  lipgloss.Color(charmtone.Charcoal.Hex()),
		BgOverlay: lipgloss.Color(charmtone.Iron.Hex()),

		// Foregrounds
		FgBase:     lipgloss.Color(charmtone.Ash.Hex()),
		FgMuted:    lipgloss.Color(charmtone.Squid.Hex()),
		FgSubtle:   lipgloss.Color(charmtone.Oyster.Hex()),
		FgSelected: lipgloss.Color(charmtone.Salt.Hex()),

		// Borders
		Border:      lipgloss.Color(charmtone.Charcoal.Hex()),
		BorderFocus: lipgloss.Color(charmtone.Charple.Hex()),

		// Status
		Success: lipgloss.Color(charmtone.Guac.Hex()),
		Error:   lipgloss.Color(charmtone.Sriracha.Hex()),
		Warning: lipgloss.Color(charmtone.Uni.Hex()),
		Info:    lipgloss.Color(charmtone.Malibu.Hex()),

		// TODO: fix this.
		SyntaxBg:      lipgloss.Color("#1C1C1F"),
		SyntaxKeyword: lipgloss.Color("#FF6DFE"),
		SyntaxString:  lipgloss.Color("#E8FE96"),
		SyntaxComment: lipgloss.Color("#6B6F85"),
	}
}
