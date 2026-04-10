package styles

import "github.com/charmbracelet/x/exp/charmtone"

// buildBuiltinThemes returns the complete map of built-in theme palettes.
// Called once during package-level var initialization.
func buildBuiltinThemes() map[string]ThemePalette {
	return map[string]ThemePalette{
		"charmtone":    DefaultPalette(),
		"gruvbox-dark": gruvboxDark(),
	}
}

// DefaultPalette returns the default Crush theme palette using charmtone
// colors from the upstream charmbracelet/x/exp/charmtone package.
func DefaultPalette() ThemePalette {
	return ThemePalette{
		Name:   "Charmtone",
		Author: "Charmbracelet",
		Colors: ThemeColors{
			Primary:   charmtone.Charple.Hex(),
			Secondary: charmtone.Dolly.Hex(),
			Tertiary:  charmtone.Bok.Hex(),

			BgBase:        charmtone.Pepper.Hex(),
			BgBaseLighter: charmtone.BBQ.Hex(),
			BgSubtle:      charmtone.Charcoal.Hex(),
			BgOverlay:     charmtone.Iron.Hex(),

			FgBase:      charmtone.Ash.Hex(),
			FgMuted:     charmtone.Squid.Hex(),
			FgHalfMuted: charmtone.Smoke.Hex(),
			FgSubtle:    charmtone.Oyster.Hex(),

			Border:      charmtone.Charcoal.Hex(),
			BorderFocus: charmtone.Charple.Hex(),

			Error:   charmtone.Sriracha.Hex(),
			Warning: charmtone.Zest.Hex(),
			Info:    charmtone.Malibu.Hex(),

			White:      charmtone.Butter.Hex(),
			BlueLight:  charmtone.Sardine.Hex(),
			Blue:       charmtone.Malibu.Hex(),
			BlueDark:   charmtone.Damson.Hex(),
			GreenLight: charmtone.Bok.Hex(),
			Green:      charmtone.Julep.Hex(),
			GreenDark:  charmtone.Guac.Hex(),
			Red:        charmtone.Coral.Hex(),
			RedDark:    charmtone.Sriracha.Hex(),
			Yellow:     charmtone.Mustard.Hex(),

			DiffInsertFg:      "#629657",
			DiffInsertBg:      "#2b322a",
			DiffInsertBgLight: "#323931",
			DiffDeleteFg:      "#a45c59",
			DiffDeleteBg:      "#312929",
			DiffDeleteBgLight: "#383030",
		},
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
			BlueDark:   "#458588",
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
