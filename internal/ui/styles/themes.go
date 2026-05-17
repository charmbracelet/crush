package styles

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// ThemeForProvider returns the Styles associated with the given provider
// ID. Unknown or empty provider IDs yield the default Charmtone Pantera
// theme.
func ThemeForProvider(providerID string) Styles {
	switch providerID {
	case "hyper":
		return HypercrushObsidiana()
	default:
		return CharmtonePantera()
	}
}

// ThemeForBackground returns the theme tuned for the terminal background
// brightness. When hasDarkBackground is false, the light theme is returned;
// otherwise, the dark theme matching the given provider is returned.
func ThemeForBackground(hasDarkBackground bool, providerID string) Styles {
	if hasDarkBackground {
		return ThemeForProvider(providerID)
	}
	return CharmtoneDaylight()
}

// CharmtoneDaylight returns the Charmtone light theme for terminals with
// light backgrounds.
func CharmtoneDaylight() Styles {
	return quickStyle(quickStyleOpts{
		primary:   charmtone.Charple,
		secondary: charmtone.Dolly,
		accent:    charmtone.Pickle,
		keyword:   charmtone.Blush,

		fgBase:       charmtone.Pepper,
		fgSubtle:     charmtone.Charcoal,
		fgMoreSubtle: charmtone.Oyster,
		fgMostSubtle: charmtone.Iron,

		onPrimary: charmtone.Salt,

		bgBase:         charmtone.Salt,
		bgLeastVisible: charmtone.Ash,
		bgLessVisible:  charmtone.Smoke,
		bgMostVisible:  charmtone.Squid,

		separator: charmtone.Anchovy,

		destructive:       charmtone.Coral,
		error:             charmtone.Sriracha,
		warningSubtle:     charmtone.Tang,
		warning:           charmtone.Mustard,
		busy:              charmtone.Yam,
		info:              charmtone.Malibu,
		infoMoreSubtle:    charmtone.Sardine,
		infoMostSubtle:    charmtone.Damson,
		success:           charmtone.Pickle,
		successMoreSubtle: charmtone.Pickle,
		successMostSubtle: charmtone.Guac,

		diffInsertFG:     lipgloss.Color("#5a7555"),
		diffInsertBGCode: lipgloss.Color("#bfdbbc"),
		diffInsertBGNum:  lipgloss.Color("#b7ceb3"),
		diffDeleteFG:     lipgloss.Color("#ad6866"),
		diffDeleteBGCode: lipgloss.Color("#e0bebe"),
		diffDeleteBGNum:  lipgloss.Color("#d3baba"),
	})
}

// CharmtonePantera returns the Charmtone dark theme. It's the default style
// for the UI.
func CharmtonePantera() Styles {
	return quickStyle(quickStyleOpts{
		primary:   charmtone.Charple,
		secondary: charmtone.Dolly,
		accent:    charmtone.Bok,
		keyword:   charmtone.Blush,

		fgBase:       charmtone.Ash,
		fgMoreSubtle: charmtone.Squid,
		fgSubtle:     charmtone.Smoke,
		fgMostSubtle: charmtone.Oyster,

		onPrimary: charmtone.Butter,

		bgBase:         charmtone.Pepper,
		bgLeastVisible: charmtone.BBQ,
		bgLessVisible:  charmtone.Charcoal,
		bgMostVisible:  charmtone.Iron,

		separator: charmtone.Charcoal,

		destructive:       charmtone.Coral,
		error:             charmtone.Sriracha,
		warningSubtle:     charmtone.Zest,
		warning:           charmtone.Mustard,
		busy:              charmtone.Citron,
		info:              charmtone.Malibu,
		infoMoreSubtle:    charmtone.Sardine,
		infoMostSubtle:    charmtone.Damson,
		success:           charmtone.Julep,
		successMoreSubtle: charmtone.Bok,
		successMostSubtle: charmtone.Guac,

		diffInsertFG:     lipgloss.Color("#629657"),
		diffInsertBGCode: lipgloss.Color("#323931"),
		diffInsertBGNum:  lipgloss.Color("#2b322a"),
		diffDeleteFG:     lipgloss.Color("#a45c59"),
		diffDeleteBGCode: lipgloss.Color("#383030"),
		diffDeleteBGNum:  lipgloss.Color("#312929"),
	})
}

// HypercrushObsidiana returns the Hypercrush dark theme.
func HypercrushObsidiana() Styles {
	return quickStyle(quickStyleOpts{
		primary:   charmtone.Charple,
		secondary: charmtone.Dolly,
		accent:    charmtone.Bok,

		fgBase:       charmtone.Ash,
		fgMoreSubtle: charmtone.Squid,
		fgSubtle:     charmtone.Smoke,
		fgMostSubtle: charmtone.Oyster,

		onPrimary: charmtone.Butter,

		bgBase:         charmtone.Pepper,
		bgLeastVisible: charmtone.BBQ,
		bgLessVisible:  charmtone.Charcoal,
		bgMostVisible:  charmtone.Iron,

		separator: charmtone.Charcoal,

		destructive:       charmtone.Coral,
		error:             charmtone.Sriracha,
		warningSubtle:     charmtone.Zest,
		warning:           charmtone.Mustard,
		busy:              charmtone.Citron,
		info:              charmtone.Malibu,
		infoMoreSubtle:    charmtone.Sardine,
		infoMostSubtle:    charmtone.Damson,
		success:           charmtone.Julep,
		successMoreSubtle: charmtone.Bok,
		successMostSubtle: charmtone.Guac,

		diffInsertFG:     lipgloss.Color("#629657"),
		diffInsertBGCode: lipgloss.Color("#323931"),
		diffInsertBGNum:  lipgloss.Color("#2b322a"),
		diffDeleteFG:     lipgloss.Color("#a45c59"),
		diffDeleteBGCode: lipgloss.Color("#383030"),
		diffDeleteBGNum:  lipgloss.Color("#312929"),
	})
}
