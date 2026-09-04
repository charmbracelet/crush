package styles

import (
	"image/color"

	"github.com/charmbracelet/x/exp/charmtone"
)

// ThemeKeyForProvider returns a stable identifier for the theme
// associated with the given provider ID. Providers that share a theme
// yield the same key, so callers can cheaply detect when switching
// providers would not actually change the active theme and skip the
// expensive style rebuild. This is the single source of truth for the
// provider-to-theme mapping; [ThemeForProvider] builds on it.
func ThemeKeyForProvider(providerID string) string {
	switch providerID {
	case "hyper":
		return "hyper"
	default:
		return "default"
	}
}

// ThemeForProvider returns the Styles associated with the given provider
// ID. Unknown or empty provider IDs yield the default Charmtone Pantera
// theme.
func ThemeForProvider(providerID string) Styles {
	switch ThemeKeyForProvider(providerID) {
	case "hyper":
		return HypercrushObsidiana()
	default:
		return CharmtonePantera()
	}
}

// CharmtonePantera returns the Charmtone dark theme. It's the default style
// for the UI.
func CharmtonePantera() Styles {
	s := quickStyle(quickStyleOpts{
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
		attention:         charmtone.Tang,
		busy:              charmtone.Citron,
		info:              charmtone.Malibu,
		infoMoreSubtle:    charmtone.Sardine,
		infoMostSubtle:    charmtone.Damson,
		success:           charmtone.Julep,
		successMoreSubtle: charmtone.Bok,
		successMostSubtle: charmtone.Guac,

		// ANSI 16-color palette for remapping raw terminal output
		// (e.g. bang-mode shell commands) onto legible Charmtone colors.
		ansiBlack:   charmtone.BBQ,
		ansiRed:     charmtone.Coral,
		ansiGreen:   charmtone.Guac,
		ansiYellow:  charmtone.Mustard,
		ansiBlue:    charmtone.Charple,
		ansiMagenta: charmtone.Dolly,
		ansiCyan:    charmtone.Malibu,
		ansiWhite:   charmtone.Smoke,

		ansiBrightBlack:   charmtone.Iron,
		ansiBrightRed:     charmtone.Tuna,
		ansiBrightGreen:   charmtone.Julep,
		ansiBrightYellow:  charmtone.Zest,
		ansiBrightBlue:    charmtone.Guppy,
		ansiBrightMagenta: charmtone.Blush,
		ansiBrightCyan:    charmtone.Sardine,
		ansiBrightWhite:   charmtone.Salt,
	})

	// Bang ! prompt overrides - use Salt/Hazy/Larple colors.
	s.Editor.PromptBangIconFocused = s.Editor.PromptBangIconFocused.
		Foreground(charmtone.Salt).
		Background(charmtone.Hazy)
	s.Editor.PromptBangDotsFocused = s.Editor.PromptBangDotsFocused.
		Foreground(charmtone.Hazy)
	s.Editor.PromptBangDotsBlurred = s.Editor.PromptBangDotsBlurred.
		Foreground(charmtone.Larple)

	// Shell bar/prompt overrides - use Charple/Iron/Hazy colors.
	s.Messages.ShellBarFocused = s.Messages.ShellBarFocused.
		BorderForeground(charmtone.Charple)
	s.Messages.ShellBarBlurred = s.Messages.ShellBarBlurred.
		BorderForeground(charmtone.Iron)
	s.Messages.ShellPrompt = s.Messages.ShellPrompt.
		Foreground(charmtone.Hazy)
	s.Messages.ShellPromptBlurred = s.Messages.ShellPromptBlurred.
		Foreground(charmtone.Hazy)

	return s
}

// HypercrushObsidiana returns the Hypercrush dark theme.
func HypercrushObsidiana() Styles {
	return CharmtonePantera()
}

// ThemeNames returns the selectable theme ids, in menu order.
func ThemeNames() []string {
	return []string{ThemeCharmtone, ThemeHyper, ThemeDyadMapper}
}

// Theme ids.
const (
	ThemeCharmtone  = "charmtone"
	ThemeHyper      = "hyper"
	ThemeDyadMapper = "dyad-mapper"
)

// ThemeForName returns the Styles for a theme id, defaulting to
// CharmtonePantera for unknown or empty ids.
func ThemeForName(name string) Styles {
	switch name {
	case ThemeHyper:
		return HypercrushObsidiana()
	case ThemeDyadMapper:
		return DyadMapper()
	default:
		return CharmtonePantera()
	}
}

// DyadMapper returns the dyad-mapper theme: a soft, low-glare navy palette
// made for sensitive eyes — built for Peter Lodri, blue eyes first, but
// shared because sight is a gift. Dark desaturated blues, no pure white,
// no pure black, every accent muted a step down from saturated.
func DyadMapper() Styles {
	navy := color.RGBA{R: 0x0B, G: 0x15, B: 0x26, A: 0xFF}
	navy1 := color.RGBA{R: 0x10, G: 0x1D, B: 0x33, A: 0xFF}
	navy2 := color.RGBA{R: 0x15, G: 0x26, B: 0x3F, A: 0xFF}
	navy3 := color.RGBA{R: 0x1B, G: 0x30, B: 0x4F, A: 0xFF}
	ice := color.RGBA{R: 0xE3, G: 0xEC, B: 0xF7, A: 0xFF}
	ice1 := color.RGBA{R: 0xA8, G: 0xBB, B: 0xD4, A: 0xFF}
	ice2 := color.RGBA{R: 0x8A, G: 0xA0, B: 0xBD, A: 0xFF}
	ice3 := color.RGBA{R: 0x6B, G: 0x80, B: 0x9C, A: 0xFF}
	azure := color.RGBA{R: 0x7C, G: 0xB8, B: 0xFF, A: 0xFF}
	periwinkle := color.RGBA{R: 0x9D, G: 0xB4, B: 0xFF, A: 0xFF}
	cyan := color.RGBA{R: 0x5E, G: 0xE0, B: 0xE6, A: 0xFF}
	softGreen := color.RGBA{R: 0x8F, G: 0xE0, B: 0xA8, A: 0xFF}
	softAmber := color.RGBA{R: 0xFF, G: 0xC8, B: 0x6B, A: 0xFF}
	softCoral := color.RGBA{R: 0xFF, G: 0x8A, B: 0x80, A: 0xFF}
	softRose := color.RGBA{R: 0xFF, G: 0x9E, B: 0x9E, A: 0xFF}
	softLilac := color.RGBA{R: 0xB0, G: 0xB8, B: 0xFF, A: 0xFF}

	s := quickStyle(quickStyleOpts{
		primary:   azure,
		secondary: periwinkle,
		accent:    cyan,
		keyword:   softLilac,

		fgBase:       ice,
		fgMoreSubtle: ice1,
		fgSubtle:     ice2,
		fgMostSubtle: ice3,

		onPrimary: navy,

		bgBase:         navy,
		bgLeastVisible: navy1,
		bgLessVisible:  navy2,
		bgMostVisible:  navy3,

		separator: navy2,

		destructive:       softRose,
		error:             softCoral,
		warningSubtle:     color.RGBA{R: 0xFF, G: 0xD9, B: 0xA0, A: 0xFF},
		warning:           softAmber,
		attention:         color.RGBA{R: 0xFF, G: 0xB8, B: 0x8A, A: 0xFF},
		busy:              softGreen,
		info:              azure,
		infoMoreSubtle:    navy3,
		infoMostSubtle:    navy2,
		success:           softGreen,
		successMoreSubtle: navy3,
		successMostSubtle: navy2,

		// ANSI 16-color remap — soft versions, no glare
		ansiBlack:         navy,
		ansiRed:           softCoral,
		ansiGreen:         softGreen,
		ansiYellow:        softAmber,
		ansiBlue:          azure,
		ansiMagenta:       periwinkle,
		ansiCyan:          cyan,
		ansiWhite:         ice1,
		ansiBrightBlack:   navy3,
		ansiBrightRed:     softRose,
		ansiBrightGreen:   color.RGBA{R: 0xB9, G: 0xEE, B: 0xC9, A: 0xFF},
		ansiBrightYellow:  color.RGBA{R: 0xFF, G: 0xDE, B: 0xA0, A: 0xFF},
		ansiBrightBlue:    color.RGBA{R: 0xA6, G: 0xCE, B: 0xFF, A: 0xFF},
		ansiBrightMagenta: color.RGBA{R: 0xBF, G: 0xCC, B: 0xFF, A: 0xFF},
		ansiBrightCyan:    color.RGBA{R: 0x9B, G: 0xEF, B: 0xF2, A: 0xFF},
		ansiBrightWhite:   ice,
	})

	return s
}
