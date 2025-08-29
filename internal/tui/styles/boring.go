package styles

import "github.com/charmbracelet/lipgloss/v2"

func NewBoringTheme() *Theme {
	// Grayscale Palette
	var (
		night       = lipgloss.Color("#1c1c1c")
		raisinBlack = lipgloss.Color("#222222")
		charleston  = lipgloss.Color("#2b2b2b")
		jet         = lipgloss.Color("#333333")
		onyx        = lipgloss.Color("#3d3d3d")
		gunmetal    = lipgloss.Color("#4e4e4e")
		granite     = lipgloss.Color("#5f5f5f")
		stone       = lipgloss.Color("#7a7a7a")
		lightSilver = lipgloss.Color("#c5c5c5")
		white       = lipgloss.Color("#ffffff")
	)

	// Accent Palette
	var (
		fern       = lipgloss.Color("#6a8e5f")
		forest     = lipgloss.Color("#4a703c")
		olivine    = lipgloss.Color("#b5c8b0")
		terracotta = lipgloss.Color("#c95e52")
		redwood    = lipgloss.Color("#a14f46")
		salmon     = lipgloss.Color("#e48981")
		rose       = lipgloss.Color("#d17d8a")
		sand       = lipgloss.Color("#d7c08d")
		teal       = lipgloss.Color("#5e9a9b")
		sky        = lipgloss.Color("#669cd6")
	)

	t := &Theme{
		Name:   "boring",
		IsDark: true,

		Primary:   granite,
		Secondary: gunmetal,
		Tertiary:  onyx,
		Accent:    stone,

		// Backgrounds
		BgBase:        night,
		BgBaseLighter: charleston,
		BgSubtle:      raisinBlack,
		BgOverlay:     jet,

		// Foregrounds
		FgBase:      lightSilver,
		FgMuted:     stone,
		FgHalfMuted: granite,
		FgSubtle:    gunmetal,
		FgSelected:  white,

		// Borders
		Border:      raisinBlack,
		BorderFocus: granite,

		// Status
		Success: fern,
		Error:   terracotta,
		Warning: sand,
		Info:    teal,

		// Colors
		White: lightSilver,

		BlueLight: sky,
		Blue:      teal,

		Yellow: sand,
		Citron: olivine,

		Green:      fern,
		GreenDark:  forest,
		GreenLight: olivine,

		Red:      terracotta,
		RedDark:  redwood,
		RedLight: salmon,
		Cherry:   rose,
	}

	// Text selection.
	t.TextSelection = lipgloss.NewStyle().Foreground(white).Background(granite)

	// LSP and MCP status.
	t.ItemOfflineIcon = lipgloss.NewStyle().Foreground(stone).SetString("●")
	t.ItemBusyIcon = t.ItemOfflineIcon.Foreground(sand)
	t.ItemErrorIcon = t.ItemOfflineIcon.Foreground(terracotta)
	t.ItemOnlineIcon = t.ItemOfflineIcon.Foreground(fern)

	t.YoloIconFocused = lipgloss.NewStyle().Foreground(night).Background(sand).Bold(true).SetString(" ! ")
	t.YoloIconBlurred = t.YoloIconFocused.Foreground(lightSilver).Background(gunmetal)
	t.YoloDotsFocused = lipgloss.NewStyle().Foreground(sand).SetString(":::")
	t.YoloDotsBlurred = t.YoloDotsFocused.Foreground(stone)

	return t
}
