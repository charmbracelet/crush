package styles

import "github.com/charmbracelet/x/exp/charmtone"

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

// CharmtonePantera returns the Charmtone dark theme. It's the default style
// for the UI.
func CharmtonePantera() Styles {
	return quickStyle(quickStyleOpts{
		primary:   charmtone.Charple,
		secondary: charmtone.Dolly,
		tertiary:  charmtone.Bok,

		fgBase:      charmtone.Ash,
		fgMuted:     charmtone.Squid,
		fgHalfMuted: charmtone.Smoke,
		fgSubtle:    charmtone.Oyster,

		onPrimary: charmtone.Salt,
		onAccent:  charmtone.Butter,

		bgBase:        charmtone.Pepper,
		bgBaseLighter: charmtone.BBQ,
		bgSubtle:      charmtone.Charcoal,
		bgOverlay:     charmtone.Iron,

		border:      charmtone.Charcoal,
		borderFocus: charmtone.Charple,

		danger:        charmtone.Coral,
		error:         charmtone.Sriracha,
		warning:       charmtone.Zest,
		warningStrong: charmtone.Mustard,
		busy:          charmtone.Citron,
		info:          charmtone.Malibu,
		infoSubtle:    charmtone.Sardine,
		infoMuted:     charmtone.Damson,
		success:       charmtone.Julep,
		successSubtle: charmtone.Bok,
		successMuted:  charmtone.Guac,
	})
}

// HypercrushObsidiana returns the Hypercrush dark theme.
func HypercrushObsidiana() Styles {
	return quickStyle(quickStyleOpts{
		primary:   charmtone.Charple,
		secondary: charmtone.Dolly,
		tertiary:  charmtone.Bok,

		fgBase:      charmtone.Ash,
		fgMuted:     charmtone.Squid,
		fgHalfMuted: charmtone.Smoke,
		fgSubtle:    charmtone.Oyster,

		onPrimary: charmtone.Salt,
		onAccent:  charmtone.Butter,

		bgBase:        charmtone.Pepper,
		bgBaseLighter: charmtone.BBQ,
		bgSubtle:      charmtone.Charcoal,
		bgOverlay:     charmtone.Iron,

		border:      charmtone.Charcoal,
		borderFocus: charmtone.Charple,

		danger:        charmtone.Coral,
		error:         charmtone.Sriracha,
		warning:       charmtone.Zest,
		warningStrong: charmtone.Mustard,
		busy:          charmtone.Citron,
		info:          charmtone.Malibu,
		infoSubtle:    charmtone.Sardine,
		infoMuted:     charmtone.Damson,
		success:       charmtone.Julep,
		successSubtle: charmtone.Bok,
		successMuted:  charmtone.Guac,
	})
}
