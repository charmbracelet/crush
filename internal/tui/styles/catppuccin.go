package styles

func NewCatppuccinLatteTheme() *Theme {
	return &Theme{
		Name:   "catppuccin-latte",
		IsDark: false,

		Primary:   ParseHex("#8839ef"), // Mauve
		Secondary: ParseHex("#1e66f5"), // Blue
		Tertiary:  ParseHex("#04a5e5"), // Sky
		Accent:    ParseHex("#fe640b"), // Peach

		// Backgrounds
		BgBase:        ParseHex("#eff1f5"), // Base
		BgBaseLighter: ParseHex("#e6e9ef"), // Mantle
		BgSubtle:      ParseHex("#dce0e8"), // Crust
		BgOverlay:     ParseHex("#9ca0b0"), // Overlay 0

		// Foregrounds
		FgBase:      ParseHex("#4c4f69"), // Text
		FgMuted:     ParseHex("#6c6f85"), // Subtext 0
		FgHalfMuted: ParseHex("#5c5f77"), // Subtext 1
		FgSubtle:    ParseHex("#7c7f93"), // Overlay 2
		FgSelected:  ParseHex("#eff1f5"), // Base (inverted for selection)

		// Borders
		Border:      ParseHex("#acb0be"), // Surface 2
		BorderFocus: ParseHex("#7287fd"), // Lavender

		// Status
		Success: ParseHex("#40a02b"), // Green
		Error:   ParseHex("#d20f39"), // Red
		Warning: ParseHex("#df8e1d"), // Yellow
		Info:    ParseHex("#1e66f5"), // Blue

		// Colors
		White: ParseHex("#eff1f5"), // Base

		BlueLight: ParseHex("#04a5e5"), // Sky
		Blue:      ParseHex("#1e66f5"), // Blue

		Yellow: ParseHex("#df8e1d"), // Yellow

		Green:      ParseHex("#40a02b"), // Green
		GreenDark:  ParseHex("#179299"), // Teal
		GreenLight: ParseHex("#209fb5"), // Sapphire

		Red:      ParseHex("#d20f39"), // Red
		RedDark:  ParseHex("#e64553"), // Maroon
		RedLight: ParseHex("#dd7878"), // Flamingo
		Cherry:   ParseHex("#ea76cb"), // Pink
	}
}
