package styles

// buildBuiltinThemes returns the complete map of built-in theme palettes.
// Called once during package-level var initialization.
func buildBuiltinThemes() map[string]ThemePalette {
	return map[string]ThemePalette{
		"charmtone":  DefaultPalette(),
		"gruvbox-dark": gruvboxDark(),
	}
}

func gruvboxDark() ThemePalette {
	return ThemePalette{
		Name:   "Gruvbox Dark",
		Author: "morhetz",
		Colors: ThemeColors{
			Primary:   "#fabd2f",
			Secondary: "#d3869b",
			Tertiary:  "#b8bb26",

			BgBase:        "#282828",
			BgBaseLighter: "#3c3836",
			BgSubtle:      "#504945",
			BgOverlay:     "#665c54",

			FgBase:      "#ebdbb2",
			FgMuted:     "#a89984",
			FgHalfMuted: "#bdae93",
			FgSubtle:    "#928374",

			Border:      "#504945",
			BorderFocus: "#fabd2f",

			Error:   "#fb4934",
			Warning: "#fabd2f",
			Info:    "#83a598",

			White:      "#fbf1c7",
			BlueLight:  "#83a598",
			Blue:       "#83a598",
			BlueDark:   "#665c54",
			GreenLight: "#b8bb26",
			Green:      "#b8bb26",
			GreenDark:  "#8ec07c",
			Red:        "#fb4934",
			RedDark:    "#cc241d",
			Yellow:     "#fabd2f",

			DiffInsertFg:      "#b8bb26",
			DiffInsertBg:      "#32361a",
			DiffInsertBgLight: "#3d4220",
			DiffDeleteFg:      "#fb4934",
			DiffDeleteBg:      "#3c1f1e",
			DiffDeleteBgLight: "#462726",
		},
	}
}
