package styles

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

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
	}
}

// gruvboxDarkOpts returns the quickStyleOpts for the Gruvbox Dark theme,
// using canonical colors from the morhetz/gruvbox palette.
func gruvboxDarkOpts() quickStyleOpts {
	return quickStyleOpts{
		primary:   lipgloss.Color("#fabd2f"), // yellow
		secondary: lipgloss.Color("#d3869b"), // purple
		tertiary:  lipgloss.Color("#b8bb26"), // green

		fgBase:      lipgloss.Color("#ebdbb2"), // fg
		fgMuted:     lipgloss.Color("#a89984"), // fg4/gray
		fgHalfMuted: lipgloss.Color("#bdae93"), // fg3
		fgSubtle:    lipgloss.Color("#928374"), // gray

		onPrimary: lipgloss.Color("#282828"), // bg on primary
		onAccent:  lipgloss.Color("#fbf1c7"), // fg0 light

		bgBase:        lipgloss.Color("#282828"), // bg
		bgBaseLighter: lipgloss.Color("#3c3836"), // bg1
		bgSubtle:      lipgloss.Color("#504945"), // bg2
		bgOverlay:     lipgloss.Color("#665c54"), // bg3

		border:      lipgloss.Color("#504945"), // bg2
		borderFocus: lipgloss.Color("#fabd2f"), // yellow

		danger:        lipgloss.Color("#fb4934"), // red bright
		error:         lipgloss.Color("#cc241d"), // red dark
		warning:       lipgloss.Color("#fabd2f"), // yellow bright
		warningStrong: lipgloss.Color("#d79921"), // yellow dark
		busy:          lipgloss.Color("#fabd2f"), // yellow bright
		info:          lipgloss.Color("#83a598"), // blue bright
		infoSubtle:    lipgloss.Color("#83a598"), // blue bright
		infoMuted:     lipgloss.Color("#458588"), // blue dark
		success:       lipgloss.Color("#b8bb26"), // green bright
		successSubtle: lipgloss.Color("#b8bb26"), // green bright
		successMuted:  lipgloss.Color("#8ec07c"), // aqua bright
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
