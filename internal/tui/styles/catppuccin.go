package styles

import (
	"charm.land/lipgloss/v2"
)

func NewCatppuccinFrappeTheme() *Theme {
	t := &Theme{
		Name:   "catppuccin-frappe",
		IsDark: true,

		Primary:   ParseHex("#ca9ee6"), // Mauve
		Secondary: ParseHex("#8caaee"), // Blue
		Tertiary:  ParseHex("#99d1db"), // Sky
		Accent:    ParseHex("#ef9f76"), // Peach

		// Backgrounds
		BgBase:        ParseHex("#303446"), // Base
		BgBaseLighter: ParseHex("#292c3c"), // Mantle
		BgSubtle:      ParseHex("#232634"), // Crust
		BgOverlay:     ParseHex("#737994"), // Overlay 0

		// Foregrounds
		FgBase:      ParseHex("#c6d0f5"), // Text
		FgMuted:     ParseHex("#a5adce"), // Subtext 0
		FgHalfMuted: ParseHex("#b5bfe2"), // Subtext 1
		FgSubtle:    ParseHex("#949cbb"), // Overlay 2
		FgSelected:  ParseHex("#303446"), // Base (inverted for selection)

		// Borders
		Border:      ParseHex("#626880"), // Surface 2
		BorderFocus: ParseHex("#babbf1"), // Lavender

		// Status
		Success: ParseHex("#a6d189"), // Green
		Error:   ParseHex("#e78284"), // Red
		Warning: ParseHex("#e5c890"), // Yellow
		Info:    ParseHex("#8caaee"), // Blue

		// Colors
		White: ParseHex("#c6d0f5"), // Text

		BlueLight: ParseHex("#99d1db"), // Sky
		Blue:      ParseHex("#8caaee"), // Blue

		Yellow: ParseHex("#e5c890"), // Yellow
		Citron: ParseHex("#e5c890"), // Yellow (alias)

		Green:      ParseHex("#a6d189"), // Green
		GreenDark:  ParseHex("#81c8be"), // Teal
		GreenLight: ParseHex("#85c1dc"), // Sapphire

		Red:      ParseHex("#e78284"), // Red
		RedDark:  ParseHex("#ea999c"), // Maroon
		RedLight: ParseHex("#eebebe"), // Flamingo
		Cherry:   ParseHex("#f4b8e4"), // Pink
	}

	// Text selection.
	t.TextSelection = lipgloss.NewStyle().Foreground(ParseHex("#303446")).Background(ParseHex("#ca9ee6"))

	// LSP and MCP status.
	t.ItemOfflineIcon = lipgloss.NewStyle().Foreground(ParseHex("#737994")).SetString("●")
	t.ItemBusyIcon = t.ItemOfflineIcon.Foreground(ParseHex("#e5c890"))
	t.ItemErrorIcon = t.ItemOfflineIcon.Foreground(ParseHex("#e78284"))
	t.ItemOnlineIcon = t.ItemOfflineIcon.Foreground(ParseHex("#a6d189"))

	// Yolo mode.
	t.YoloIconFocused = lipgloss.NewStyle().Foreground(ParseHex("#303446")).Background(ParseHex("#e5c890")).Bold(true).SetString(" ! ")
	t.YoloIconBlurred = t.YoloIconFocused.Foreground(ParseHex("#303446")).Background(ParseHex("#737994"))
	t.YoloDotsFocused = lipgloss.NewStyle().Foreground(ParseHex("#ef9f76")).SetString(":::")
	t.YoloDotsBlurred = t.YoloDotsFocused.Foreground(ParseHex("#737994"))

	return t
}
