package styles

import (
	"image/color"
	"os"
	"strings"

	"github.com/charmbracelet/x/exp/charmtone"
)

// ThemeForProvider returns the Styles associated with the given provider
// ID. Unknown or empty provider IDs yield the default Charmtone Pantera
// theme.
//
// An optional theme preference overrides the provider-based default. Valid
// values are "auto" (provider-based default), "dark", and "light". When no
// preference is supplied, the CRUSH_THEME environment variable is consulted.
func ThemeForProvider(providerID string, themePref ...string) Styles {
	theme := "auto"
	if len(themePref) > 0 && themePref[0] != "" {
		theme = themePref[0]
	} else if v := os.Getenv("CRUSH_THEME"); v != "" {
		theme = v
	}

	switch strings.ToLower(theme) {
	case "light":
		return CharmtoneLatte()
	case "dark":
		// Explicit dark falls through to provider defaults below.
	}

	switch providerID {
	case "hyper":
		return HypercrushObsidiana()
	default:
		return CharmtonePantera()
	}
}

// CharmtonePantera returns the Charmtone dark theme. It's the default style
// for the UI.
func CharmtonePantera() Styles {
	return quickStyle(quickStyleOpts{
		primary:   charmtone.Charple,
		secondary: charmtone.Dolly,
		accent:    charmtone.Bok,
		keyword:   charmtone.Blush,

		fgBase:       charmtone.Sash,
		fgMoreSubtle: charmtone.Squid,
		fgSubtle:     charmtone.Smoke,
		fgMostSubtle: charmtone.Oyster,

		onPrimary: charmtone.Butter,

		bgBase:         charmtone.Pepper,
		bgLeastVisible: charmtone.BBQ,
		bgLessVisible:  charmtone.Char,
		bgMostVisible:  charmtone.Iron,

		separator: charmtone.Char,

		destructive:       charmtone.Coral,
		error:             charmtone.Sriracha,
		warningSubtle:     charmtone.Zest,
		warning:           charmtone.Mustard,
		denied:            charmtone.Tang,
		busy:              charmtone.Citron,
		info:              charmtone.Malibu,
		infoMoreSubtle:    charmtone.Sardine,
		infoMostSubtle:    charmtone.Damson,
		success:           charmtone.Julep,
		successMoreSubtle: charmtone.Bok,
		successMostSubtle: charmtone.Guac,

		diffInsertFg:       color.RGBA{R: 0x62, G: 0x96, B: 0x57, A: 0xFF},
		diffInsertBg:       color.RGBA{R: 0x2b, G: 0x32, B: 0x2a, A: 0xFF},
		diffInsertSymbolBg: color.RGBA{R: 0x32, G: 0x39, B: 0x31, A: 0xFF},
		diffDeleteFg:       color.RGBA{R: 0xa4, G: 0x5c, B: 0x59, A: 0xFF},
		diffDeleteBg:       color.RGBA{R: 0x31, G: 0x29, B: 0x29, A: 0xFF},
		diffDeleteSymbolBg: color.RGBA{R: 0x38, G: 0x30, B: 0x30, A: 0xFF},
	})
}

// HypercrushObsidiana returns the Hypercrush dark theme.
func HypercrushObsidiana() Styles {
	return quickStyle(quickStyleOpts{
		primary:   charmtone.Charple,
		secondary: charmtone.Dolly,
		accent:    charmtone.Bok,

		fgBase:       charmtone.Sash,
		fgMoreSubtle: charmtone.Squid,
		fgSubtle:     charmtone.Smoke,
		fgMostSubtle: charmtone.Oyster,

		onPrimary: charmtone.Butter,

		bgBase:         charmtone.Pepper,
		bgLeastVisible: charmtone.BBQ,
		bgLessVisible:  charmtone.Char,
		bgMostVisible:  charmtone.Iron,

		separator: charmtone.Char,

		destructive:       charmtone.Coral,
		error:             charmtone.Sriracha,
		warningSubtle:     charmtone.Zest,
		warning:           charmtone.Mustard,
		denied:            charmtone.Tang,
		busy:              charmtone.Citron,
		info:              charmtone.Malibu,
		infoMoreSubtle:    charmtone.Sardine,
		infoMostSubtle:    charmtone.Damson,
		success:           charmtone.Julep,
		successMoreSubtle: charmtone.Bok,
		successMostSubtle: charmtone.Guac,

		diffInsertFg:       color.RGBA{R: 0x62, G: 0x96, B: 0x57, A: 0xFF},
		diffInsertBg:       color.RGBA{R: 0x2b, G: 0x32, B: 0x2a, A: 0xFF},
		diffInsertSymbolBg: color.RGBA{R: 0x32, G: 0x39, B: 0x31, A: 0xFF},
		diffDeleteFg:       color.RGBA{R: 0xa4, G: 0x5c, B: 0x59, A: 0xFF},
		diffDeleteBg:       color.RGBA{R: 0x31, G: 0x29, B: 0x29, A: 0xFF},
		diffDeleteSymbolBg: color.RGBA{R: 0x38, G: 0x30, B: 0x30, A: 0xFF},
	})
}

// CharmtoneLatte returns a light variant of the Charmtone theme designed for
// terminals with a light background.
func CharmtoneLatte() Styles {
	return quickStyle(quickStyleOpts{
		primary:   charmtone.Charple,
		secondary: charmtone.Dolly,
		accent:    charmtone.Malibu,
		keyword:   charmtone.Blush,

		fgBase:       charmtone.Pepper,
		fgMoreSubtle: charmtone.Char,
		fgSubtle:     charmtone.Iron,
		fgMostSubtle: charmtone.Oyster,

		onPrimary: charmtone.Butter,

		bgBase:         charmtone.Soda,
		bgLeastVisible: charmtone.Sash,
		bgLessVisible:  charmtone.Steep,
		bgMostVisible:  charmtone.Squid,

		separator: charmtone.Steam,

		destructive:       charmtone.Coral,
		error:             charmtone.Sriracha,
		warningSubtle:     charmtone.Yam,
		warning:           charmtone.Tang,
		denied:            charmtone.Tang,
		busy:              charmtone.Mustard,
		info:              charmtone.Malibu,
		infoMoreSubtle:    charmtone.Damson,
		infoMostSubtle:    charmtone.Sapphire,
		success:           charmtone.Guac,
		successMoreSubtle: charmtone.Julep,
		successMostSubtle: charmtone.Pickle,

		diffInsertFg:       color.RGBA{R: 0x00, G: 0xa4, B: 0x75, A: 0xFF},
		diffInsertBg:       color.RGBA{R: 0xe6, G: 0xf7, B: 0xf1, A: 0xFF},
		diffInsertSymbolBg: color.RGBA{R: 0xd1, G: 0xf2, B: 0xe6, A: 0xFF},
		diffDeleteFg:       color.RGBA{R: 0xd7, G: 0x3a, B: 0x49, A: 0xFF},
		diffDeleteBg:       color.RGBA{R: 0xff, G: 0xe6, B: 0xe6, A: 0xFF},
		diffDeleteSymbolBg: color.RGBA{R: 0xff, G: 0xd1, B: 0xd1, A: 0xFF},
	})
}
