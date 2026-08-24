package styles

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
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
	return charmtoneOverrides(quickStyle(charmtoneOpts()))
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
	}
}

// charmtoneOverrides applies Charmtone-specific tweaks that don't fit the
// token model of [quickStyleOpts].
func charmtoneOverrides(s Styles) Styles {
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

	// Restore the original Charmtone syntax-highlight and markdown colors
	// where the generic quickStyle token choices diverge from the palette
	// this theme has always used.
	chroma := s.Markdown.CodeBlock.Chroma
	if chroma != nil {
		chroma.CommentPreproc.Color = hex(charmtone.Bengal)
		chroma.KeywordReserved.Color = hex(charmtone.Pony)
		chroma.KeywordNamespace.Color = hex(charmtone.Pony)
		chroma.KeywordType.Color = hex(charmtone.Guppy)
		chroma.Operator.Color = hex(charmtone.Salmon)
		chroma.NameTag.Color = hex(charmtone.Mauve)
		chroma.NameAttribute.Color = hex(charmtone.Hazy)
		chroma.NameClass.Color = hex(charmtone.Salt)
		chroma.LiteralString.Color = hex(charmtone.Cumin)
	}
	s.Markdown.Link.Color = hex(charmtone.Zinc)
	s.Markdown.Image.Color = hex(charmtone.Cheeky)

	return s
}

// HypercrushObsidiana returns the Hypercrush dark theme.
func HypercrushObsidiana() Styles {
	return CharmtonePantera()
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
		attention:         lipgloss.Color("#fe8019"), // orange
		busy:              lipgloss.Color("#fabd2f"), // yellow bright
		info:              lipgloss.Color("#83a598"), // blue bright
		infoMoreSubtle:    lipgloss.Color("#83a598"), // blue bright
		infoMostSubtle:    lipgloss.Color("#458588"), // blue dark
		success:           lipgloss.Color("#b8bb26"), // green bright
		successMoreSubtle: lipgloss.Color("#b8bb26"), // green bright
		successMostSubtle: lipgloss.Color("#8ec07c"), // aqua bright

		// ANSI 16-color palette for remapping raw terminal output
		// (e.g. bang-mode shell commands) onto legible Gruvbox colors.
		ansiBlack:   lipgloss.Color("#282828"),
		ansiRed:     lipgloss.Color("#cc241d"),
		ansiGreen:   lipgloss.Color("#98971a"),
		ansiYellow:  lipgloss.Color("#d79921"),
		ansiBlue:    lipgloss.Color("#458588"),
		ansiMagenta: lipgloss.Color("#b16286"),
		ansiCyan:    lipgloss.Color("#689d6a"),
		ansiWhite:   lipgloss.Color("#a89984"),

		ansiBrightBlack:   lipgloss.Color("#928374"),
		ansiBrightRed:     lipgloss.Color("#fb4934"),
		ansiBrightGreen:   lipgloss.Color("#b8bb26"),
		ansiBrightYellow:  lipgloss.Color("#fabd2f"),
		ansiBrightBlue:    lipgloss.Color("#83a598"),
		ansiBrightMagenta: lipgloss.Color("#d3869b"),
		ansiBrightCyan:    lipgloss.Color("#8ec07c"),
		ansiBrightWhite:   lipgloss.Color("#ebdbb2"),
	}
}

// gruvboxDarkOverrides applies Gruvbox-specific tweaks on top of the
// token-driven base styles.
func gruvboxDarkOverrides(s Styles) Styles {
	// The shared quickStyle renders inline code as the destructive
	// (bright red) color on the code background. In Gruvbox that pairing
	// (#fb4934 on #504945) is only ~2.6:1 contrast, which is hard to
	// read. Use Gruvbox orange on the darkest background instead, which
	// keeps a warm "code" feel while clearing WCAG AA (~5.8:1).
	s.Markdown.Code.Color = hex(lipgloss.Color("#fe8019"))
	s.Markdown.Code.BackgroundColor = hex(lipgloss.Color("#282828"))
	return s
}

// builtinThemes maps theme names to their quickStyleOpts palette definitions.
var builtinThemes = map[string]func() quickStyleOpts{
	"charmtone":    charmtoneOpts,
	"gruvbox-dark": gruvboxDarkOpts,
}

// builtinThemeOverrides maps theme names to functions that apply
// theme-specific style tweaks on top of the styles produced by
// [quickStyle]. Themes without overrides are absent from the map.
var builtinThemeOverrides = map[string]func(Styles) Styles{
	"charmtone":    charmtoneOverrides,
	"gruvbox-dark": gruvboxDarkOverrides,
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

// LoadTheme loads a theme by name. User theme files (in ThemeDirs) take
// precedence over built-in themes of the same name. Returns
// CharmtonePantera styles for an empty name. Returns an error if the
// name is not recognized as either a user file or a built-in.
func LoadTheme(name string) (Styles, error) {
	if name == "" {
		return CharmtonePantera(), nil
	}
	key := strings.ToLower(name)

	// Check user theme files first (project-local, then global).
	if path, err := FindThemeFile(key); err == nil {
		tf, err := LoadThemeFile(path)
		if err != nil {
			return Styles{}, err
		}
		base := tf.Base
		if base == "" {
			base = "charmtone"
		}
		return LoadPaletteTheme(base, tf.Palette)
	}

	// Fall back to built-in themes.
	optsFn, ok := builtinThemes[key]
	if !ok {
		return Styles{}, fmt.Errorf("unknown theme %q; available themes: %s", name, strings.Join(BuiltinThemeNames(), ", "))
	}
	s := quickStyle(optsFn())
	if override, ok := builtinThemeOverrides[key]; ok {
		s = override(s)
	}
	return s, nil
}

// ThemeSource indicates where a theme definition comes from.
type ThemeSource int

const (
	// ThemeSourceBuiltin is a theme compiled into the binary.
	ThemeSourceBuiltin ThemeSource = iota
	// ThemeSourceUser is a theme file in ~/.config/crush/themes/.
	ThemeSourceUser
	// ThemeSourceProject is a theme file in ./.crush/themes/.
	ThemeSourceProject
)

// String returns a human-readable label for the theme source.
func (s ThemeSource) String() string {
	switch s {
	case ThemeSourceUser:
		return "user"
	case ThemeSourceProject:
		return "project"
	default:
		return "builtin"
	}
}

// ThemeInfo describes an available theme for listing purposes.
type ThemeInfo struct {
	Name       string
	Source     ThemeSource
	Overridden bool // true if a user/project file shadows a builtin
}

// ListAllThemes returns all available themes (built-in + user files),
// sorted by name. Built-in themes that are shadowed by a user file are
// marked as Overridden and appear with the user/project source instead.
func ListAllThemes() []ThemeInfo {
	userThemes, _ := ListUserThemes()
	userSet := make(map[string]bool, len(userThemes))
	for _, n := range userThemes {
		userSet[n] = true
	}

	// Determine which user themes shadow builtins.
	overridden := make(map[string]bool)
	for _, n := range userThemes {
		if _, ok := builtinThemes[n]; ok {
			overridden[n] = true
		}
	}

	var infos []ThemeInfo

	// Add built-in themes (mark overridden ones).
	for _, name := range BuiltinThemeNames() {
		infos = append(infos, ThemeInfo{
			Name:       name,
			Source:     ThemeSourceBuiltin,
			Overridden: overridden[name],
		})
	}

	// Add user themes that don't shadow a builtin.
	for _, name := range userThemes {
		if _, isBuiltin := builtinThemes[name]; isBuiltin {
			continue
		}
		source := ThemeSourceUser
		// Check if it's project-local by looking at the first match.
		if path, err := FindThemeFile(name); err == nil {
			dirs := ThemeDirs()
			if len(dirs) > 0 && filepath.Dir(path) == dirs[0] {
				source = ThemeSourceProject
			}
		}
		infos = append(infos, ThemeInfo{
			Name:   name,
			Source: source,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return infos
}

// ExportResolvedPalette resolves a theme fully (all palette tokens
// filled) and returns it as a ThemeFile suitable for writing to disk.
// This is used when forking a built-in theme or exporting the current
// palette. The returned ThemeFile has Base set to the source theme name
// and all Palette fields populated with resolved color values.
func ExportResolvedPalette(name string) (*ThemeFile, error) {
	key := strings.ToLower(name)

	// Check if there's a user theme file first.
	if path, err := FindThemeFile(key); err == nil {
		tf, err := LoadThemeFile(path)
		if err != nil {
			return nil, err
		}
		baseName := tf.Base
		if baseName == "" {
			baseName = "charmtone"
		}
		resolved, err := MergePalette(baseName, tf.Palette)
		if err != nil {
			return nil, err
		}
		return &ThemeFile{Base: baseName, Palette: resolved}, nil
	}

	// Built-in theme: extract full palette from its opts.
	optsFn, ok := builtinThemes[key]
	if !ok {
		return nil, fmt.Errorf("unknown theme %q", name)
	}
	return &ThemeFile{
		Base:    key,
		Palette: PaletteFromOpts(optsFn()),
	}, nil
}

// IsBuiltinTheme reports whether the given name matches a built-in theme.
func IsBuiltinTheme(name string) bool {
	_, ok := builtinThemes[strings.ToLower(name)]
	return ok
}
