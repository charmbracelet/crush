// Package themes resolves named color themes into UI styles. Themes come
// from builtins, an embedded iTerm2 scheme catalog, and user-authored files.
package themes

import (
	"fmt"
	"image/color"
	"log/slog"
	"strings"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/styles"
)

// Scheme is a color theme expressed as a 16-color terminal palette plus
// background and foreground colors.
type Scheme struct {
	Name       string
	ANSI       [16]color.Color
	Background color.Color
	Foreground color.Color
}

// Catalog returns the ordered list of resolvable themes: builtins
// ("default", "hyper"), the embedded iTerm2 catalog, and user-authored themes.
// A user theme shadows an embedded theme of the same name.
func Catalog() []Scheme {
	embedded, err := catalogSchemes()
	if err != nil {
		slog.Warn("Failed to load embedded theme catalog", "error", err)
	}
	user := userSchemes()

	shadow := make(map[string]Scheme, len(user))
	for _, s := range user {
		shadow[strings.ToLower(s.Name)] = s
	}

	result := make([]Scheme, 0, len(embedded)+len(user)+2)
	result = append(result, Scheme{Name: "default"}, Scheme{Name: "hyper"})
	seen := map[string]bool{"default": true, "hyper": true}

	for _, s := range embedded {
		key := strings.ToLower(s.Name)
		if seen[key] {
			continue
		}
		if u, ok := shadow[key]; ok {
			result = append(result, u)
		} else {
			result = append(result, s)
		}
		seen[key] = true
	}
	for _, s := range user {
		key := strings.ToLower(s.Name)
		if seen[key] {
			continue
		}
		result = append(result, s)
		seen[key] = true
	}
	return result
}

// Resolve returns the Styles for a named theme. The empty name and
// "default" resolve to the default Charmtone Pantera theme. User-authored
// themes take precedence over the embedded catalog.
func Resolve(name string) (styles.Styles, error) {
	switch strings.ToLower(name) {
	case "", "default":
		return styles.CharmtonePantera(), nil
	case "hyper":
		return styles.HypercrushObsidiana(), nil
	}

	if s, ok := resolveScheme(userSchemes(), name); ok {
		return s, nil
	}

	schemes, err := catalogSchemes()
	if err != nil {
		return styles.Styles{}, err
	}
	if s, ok := resolveScheme(schemes, name); ok {
		return s, nil
	}
	return styles.Styles{}, fmt.Errorf("unknown theme %q", name)
}

func resolveScheme(schemes []Scheme, name string) (styles.Styles, bool) {
	for i := range schemes {
		if strings.EqualFold(schemes[i].Name, name) {
			return styles.FromPalette(Adapt(schemes[i])), true
		}
	}
	return styles.Styles{}, false
}

// ResolveConfigured resolves a configured theme name, returning fallback when
// the name is empty or unknown. It is the single entry point for turning an
// options.tui.theme value into a Styles.
func ResolveConfigured(name string, fallback styles.Styles) styles.Styles {
	if name == "" {
		return fallback
	}
	s, err := Resolve(name)
	if err != nil {
		slog.Warn("Unknown theme, falling back to default", "theme", name)
		return fallback
	}
	return s
}

// ResolveForProvider resolves the active theme from a config. An explicitly
// configured options.tui.theme wins; otherwise provider-derived theming
// applies. Nil cfg, Options, or TUI yield the provider-derived/default theme.
func ResolveForProvider(cfg *config.Config, providerID string) styles.Styles {
	return ResolveConfigured(ConfiguredTheme(cfg), styles.ThemeForProvider(providerID))
}

// ConfiguredTheme returns the configured theme name from cfg, or "" when
// unset. cfg may be nil.
func ConfiguredTheme(cfg *config.Config) string {
	if cfg == nil || cfg.Options == nil || cfg.Options.TUI == nil {
		return ""
	}
	return cfg.Options.TUI.Theme
}

// Adapt converts a 16-color scheme into a semantic palette, deriving the
// semantic roles from the ANSI slots and blending background/foreground for
// the subtle/visible levels.
func Adapt(scheme Scheme) styles.Palette {
	a := scheme.ANSI
	bg := firstNonNil(scheme.Background)
	fg := firstNonNil(scheme.Foreground)

	primary := firstNonNil(a[12], a[4], fg)
	secondary := firstNonNil(a[13], a[5], fg)
	accent := firstNonNil(a[14], a[6], fg)
	keyword := firstNonNil(a[9], a[1], fg)
	destructive := firstNonNil(a[1], a[9], fg)
	warning := firstNonNil(a[3], a[11], fg)
	attention := firstNonNil(a[11], a[3], fg)
	busy := firstNonNil(a[10], a[2], fg)
	info := firstNonNil(a[4], a[12], fg)
	success := firstNonNil(a[2], a[10], fg)

	return styles.Palette{
		Primary:   primary,
		Secondary: secondary,
		Accent:    accent,
		Keyword:   keyword,

		FgBase:       fg,
		BgBase:       bg,
		Separator:    blend(bg, fg, 0.15),
		FgSubtle:     blend(fg, bg, 0.15),
		FgMoreSubtle: blend(fg, bg, 0.35),
		FgMostSubtle: blend(fg, bg, 0.55),
		OnPrimary:    contrastText(primary),

		BgMostVisible:  blend(bg, fg, 0.24),
		BgLessVisible:  blend(bg, fg, 0.16),
		BgLeastVisible: blend(bg, fg, 0.08),

		Destructive:       destructive,
		Error:             destructive,
		Warning:           warning,
		WarningSubtle:     blend(warning, bg, 0.4),
		Attention:         attention,
		Busy:              busy,
		Info:              info,
		InfoMoreSubtle:    blend(info, bg, 0.5),
		InfoMostSubtle:    blend(info, bg, 0.7),
		Success:           success,
		SuccessMoreSubtle: blend(success, bg, 0.4),
		SuccessMostSubtle: blend(success, bg, 0.6),

		AnsiBlack:         firstNonNil(a[0], bg),
		AnsiRed:           firstNonNil(a[1], fg),
		AnsiGreen:         firstNonNil(a[2], fg),
		AnsiYellow:        firstNonNil(a[3], fg),
		AnsiBlue:          firstNonNil(a[4], fg),
		AnsiMagenta:       firstNonNil(a[5], fg),
		AnsiCyan:          firstNonNil(a[6], fg),
		AnsiWhite:         firstNonNil(a[7], fg),
		AnsiBrightBlack:   firstNonNil(a[8], fg),
		AnsiBrightRed:     firstNonNil(a[9], fg),
		AnsiBrightGreen:   firstNonNil(a[10], fg),
		AnsiBrightYellow:  firstNonNil(a[11], fg),
		AnsiBrightBlue:    firstNonNil(a[12], fg),
		AnsiBrightMagenta: firstNonNil(a[13], fg),
		AnsiBrightCyan:    firstNonNil(a[14], fg),
		AnsiBrightWhite:   firstNonNil(a[15], fg),
	}
}
