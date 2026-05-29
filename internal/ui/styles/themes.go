package styles

import (
	"fmt"
	"sort"
	"strings"

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

// CharmtonePantera returns the Charmtone dark theme. It's the default style
// for the UI.
func CharmtonePantera() Styles {
	return quickStyle(charmtoneOpts())
}

// charmtoneOpts returns the quickStyleOpts for the Charmtone dark theme,
// using colors from the upstream charmbracelet/x/exp/charmtone package.
func charmtoneOpts() quickStyleOpts {
	return quickStyleOpts{
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
	}
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
	})
}

// gruvboxDarkOpts returns the quickStyleOpts for the Gruvbox Dark theme,
// using canonical colors from the morhetz/gruvbox palette.
func gruvboxDarkOpts() quickStyleOpts {
	return quickStyleOpts{
		primary:   lipgloss.Color("#fabd2f"), // yellow
		secondary: lipgloss.Color("#d3869b"), // purple
		accent:    lipgloss.Color("#b8bb26"), // green
		keyword:   lipgloss.Color("#fe8019"), // orange

		fgBase:       lipgloss.Color("#ebdbb2"), // fg
		fgMoreSubtle: lipgloss.Color("#a89984"), // fg4/gray
		fgSubtle:     lipgloss.Color("#bdae93"), // fg3
		fgMostSubtle: lipgloss.Color("#928374"), // gray

		onPrimary: lipgloss.Color("#282828"), // bg on primary

		bgBase:         lipgloss.Color("#282828"), // bg
		bgLeastVisible: lipgloss.Color("#3c3836"), // bg1
		bgLessVisible:  lipgloss.Color("#504945"), // bg2
		bgMostVisible:  lipgloss.Color("#665c54"), // bg3

		separator: lipgloss.Color("#504945"), // bg2

		destructive:       lipgloss.Color("#fb4934"), // red bright
		error:             lipgloss.Color("#cc241d"), // red dark
		warningSubtle:     lipgloss.Color("#fabd2f"), // yellow bright
		warning:           lipgloss.Color("#d79921"), // yellow dark
		denied:            lipgloss.Color("#fe8019"), // orange
		busy:              lipgloss.Color("#fabd2f"), // yellow bright
		info:              lipgloss.Color("#83a598"), // blue bright
		infoMoreSubtle:    lipgloss.Color("#83a598"), // blue bright
		infoMostSubtle:    lipgloss.Color("#458588"), // blue dark
		success:           lipgloss.Color("#b8bb26"), // green bright
		successMoreSubtle: lipgloss.Color("#b8bb26"), // green bright
		successMostSubtle: lipgloss.Color("#8ec07c"), // aqua bright
	}
}

// builtinThemes maps theme names to their quickStyleOpts palette definitions.
var builtinThemes = map[string]func() quickStyleOpts{
	"charmtone":    charmtoneOpts,
	"gruvbox-dark": gruvboxDarkOpts,
}

// BuiltinThemeNames returns the names of all built-in themes, sorted.
func BuiltinThemeNames() []string {
	names := make([]string, 0, len(builtinThemes))
	for name := range builtinThemes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// LoadTheme loads a theme by built-in name. Returns CharmtonePantera styles
// for an empty name. Returns an error if the name is not recognized.
func LoadTheme(name string) (Styles, error) {
	if name == "" {
		return CharmtonePantera(), nil
	}
	if optsFn, ok := builtinThemes[strings.ToLower(name)]; ok {
		return quickStyle(optsFn()), nil
	}
	return Styles{}, fmt.Errorf("unknown theme %q; available themes: %s", name, strings.Join(BuiltinThemeNames(), ", "))
}
