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
		FgMuted:     charmtone.BBQ,      // Dark for muted text (#2d2c35)
		FgHalfMuted: charmtone.Charcoal, // Medium dark for half-muted (#3A3943)
		FgSubtle:    charmtone.Iron,     // Medium gray for subtle text (#4D4C57)
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
