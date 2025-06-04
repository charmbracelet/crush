package styles

import (
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
		// Backgrounds
		BgBase:    charmtone.Pepper,
		BgSubtle:  charmtone.Charcoal,
		BgOverlay: charmtone.Iron,

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
		Warning: charmtone.Uni,
		Info:    charmtone.Malibu,

		// Colors
		Blue: charmtone.Malibu,

		Green:      charmtone.Julep,
		GreenDark:  charmtone.Guac,
		GreenLight: charmtone.Bok,

		Red:      charmtone.Coral,
		RedDark:  charmtone.Sriracha,
		RedLight: charmtone.Salmon,
	}
}
