package styles

import (
	"fmt"
	"image/color"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
)

// validHexColor matches "#rgb", "#rrggbb", "#rgba", or "#rrggbbaa".
var validHexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// Palette is a JSON-serializable theme palette. Each field maps to a
// quickStyleOpts color, stored as a hex string ("#rrggbb"). Empty
// strings mean "inherit from base" when merging.
type Palette struct {
	Primary   string `json:"primary,omitempty"`
	Secondary string `json:"secondary,omitempty"`
	Accent    string `json:"accent,omitempty"`
	Keyword   string `json:"keyword,omitempty"`

	FgBase       string `json:"fg_base,omitempty"`
	FgSubtle     string `json:"fg_subtle,omitempty"`
	FgMoreSubtle string `json:"fg_more_subtle,omitempty"`
	FgMostSubtle string `json:"fg_most_subtle,omitempty"`

	BgBase         string `json:"bg_base,omitempty"`
	BgMostVisible  string `json:"bg_most_visible,omitempty"`
	BgLessVisible  string `json:"bg_less_visible,omitempty"`
	BgLeastVisible string `json:"bg_least_visible,omitempty"`

	OnPrimary string `json:"on_primary,omitempty"`
	Separator string `json:"separator,omitempty"`

	Destructive       string `json:"destructive,omitempty"`
	Error             string `json:"error,omitempty"`
	Warning           string `json:"warning,omitempty"`
	WarningSubtle     string `json:"warning_subtle,omitempty"`
	Denied            string `json:"denied,omitempty"`
	Busy              string `json:"busy,omitempty"`
	Info              string `json:"info,omitempty"`
	InfoMoreSubtle    string `json:"info_more_subtle,omitempty"`
	InfoMostSubtle    string `json:"info_most_subtle,omitempty"`
	Success           string `json:"success,omitempty"`
	SuccessMoreSubtle string `json:"success_more_subtle,omitempty"`
	SuccessMostSubtle string `json:"success_most_subtle,omitempty"`
}

// PaletteFromOpts extracts a Palette from quickStyleOpts, converting
// each color.Color to its "#rrggbb" hex representation.
func PaletteFromOpts(o quickStyleOpts) Palette {
	return Palette{
		Primary:   colorToHex(o.primary),
		Secondary: colorToHex(o.secondary),
		Accent:    colorToHex(o.accent),
		Keyword:   colorToHex(o.keyword),

		FgBase:       colorToHex(o.fgBase),
		FgSubtle:     colorToHex(o.fgSubtle),
		FgMoreSubtle: colorToHex(o.fgMoreSubtle),
		FgMostSubtle: colorToHex(o.fgMostSubtle),

		BgBase:         colorToHex(o.bgBase),
		BgMostVisible:  colorToHex(o.bgMostVisible),
		BgLessVisible:  colorToHex(o.bgLessVisible),
		BgLeastVisible: colorToHex(o.bgLeastVisible),

		OnPrimary: colorToHex(o.onPrimary),
		Separator: colorToHex(o.separator),

		Destructive:       colorToHex(o.destructive),
		Error:             colorToHex(o.error),
		Warning:           colorToHex(o.warning),
		WarningSubtle:     colorToHex(o.warningSubtle),
		Denied:            colorToHex(o.denied),
		Busy:              colorToHex(o.busy),
		Info:              colorToHex(o.info),
		InfoMoreSubtle:    colorToHex(o.infoMoreSubtle),
		InfoMostSubtle:    colorToHex(o.infoMostSubtle),
		Success:           colorToHex(o.success),
		SuccessMoreSubtle: colorToHex(o.successMoreSubtle),
		SuccessMostSubtle: colorToHex(o.successMostSubtle),
	}
}

// ToQuickStyleOpts converts a Palette back to quickStyleOpts. Non-empty
// fields are parsed as hex colors; empty fields fall back to the
// provided base palette.
func (p Palette) ToQuickStyleOpts(base quickStyleOpts) quickStyleOpts {
	return quickStyleOpts{
		primary:   resolveColor(p.Primary, base.primary),
		secondary: resolveColor(p.Secondary, base.secondary),
		accent:    resolveColor(p.Accent, base.accent),
		keyword:   resolveColor(p.Keyword, base.keyword),

		fgBase:       resolveColor(p.FgBase, base.fgBase),
		fgSubtle:     resolveColor(p.FgSubtle, base.fgSubtle),
		fgMoreSubtle: resolveColor(p.FgMoreSubtle, base.fgMoreSubtle),
		fgMostSubtle: resolveColor(p.FgMostSubtle, base.fgMostSubtle),

		bgBase:         resolveColor(p.BgBase, base.bgBase),
		bgMostVisible:  resolveColor(p.BgMostVisible, base.bgMostVisible),
		bgLessVisible:  resolveColor(p.BgLessVisible, base.bgLessVisible),
		bgLeastVisible: resolveColor(p.BgLeastVisible, base.bgLeastVisible),

		onPrimary: resolveColor(p.OnPrimary, base.onPrimary),
		separator: resolveColor(p.Separator, base.separator),

		destructive:       resolveColor(p.Destructive, base.destructive),
		error:             resolveColor(p.Error, base.error),
		warning:           resolveColor(p.Warning, base.warning),
		warningSubtle:     resolveColor(p.WarningSubtle, base.warningSubtle),
		denied:            resolveColor(p.Denied, base.denied),
		busy:              resolveColor(p.Busy, base.busy),
		info:              resolveColor(p.Info, base.info),
		infoMoreSubtle:    resolveColor(p.InfoMoreSubtle, base.infoMoreSubtle),
		infoMostSubtle:    resolveColor(p.InfoMostSubtle, base.infoMostSubtle),
		success:           resolveColor(p.Success, base.success),
		successMoreSubtle: resolveColor(p.SuccessMoreSubtle, base.successMoreSubtle),
		successMostSubtle: resolveColor(p.SuccessMostSubtle, base.successMostSubtle),
	}
}

// Validate checks that all non-empty hex strings in the palette are
// valid color values. Returns an error listing all invalid fields.
func (p Palette) Validate() error {
	var errs []string
	validate := func(field, value string) {
		if value == "" {
			return
		}
		if !validHexColor.MatchString(value) {
			errs = append(errs, fmt.Sprintf("%s: invalid color %q", field, value))
		}
	}

	validate("primary", p.Primary)
	validate("secondary", p.Secondary)
	validate("accent", p.Accent)
	validate("keyword", p.Keyword)
	validate("fg_base", p.FgBase)
	validate("fg_subtle", p.FgSubtle)
	validate("fg_more_subtle", p.FgMoreSubtle)
	validate("fg_most_subtle", p.FgMostSubtle)
	validate("bg_base", p.BgBase)
	validate("bg_most_visible", p.BgMostVisible)
	validate("bg_less_visible", p.BgLessVisible)
	validate("bg_least_visible", p.BgLeastVisible)
	validate("on_primary", p.OnPrimary)
	validate("separator", p.Separator)
	validate("destructive", p.Destructive)
	validate("error", p.Error)
	validate("warning", p.Warning)
	validate("warning_subtle", p.WarningSubtle)
	validate("denied", p.Denied)
	validate("busy", p.Busy)
	validate("info", p.Info)
	validate("info_more_subtle", p.InfoMoreSubtle)
	validate("info_most_subtle", p.InfoMostSubtle)
	validate("success", p.Success)
	validate("success_more_subtle", p.SuccessMoreSubtle)
	validate("success_most_subtle", p.SuccessMostSubtle)

	if len(errs) > 0 {
		return fmt.Errorf("invalid palette colors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ThemePalette returns the Palette for a built-in theme by name.
// Returns an error if the theme is not recognized.
func ThemePalette(name string) (Palette, error) {
	optsFn, ok := builtinThemes[strings.ToLower(name)]
	if !ok {
		return Palette{}, fmt.Errorf("unknown theme %q; available themes: %s", name, strings.Join(BuiltinThemeNames(), ", "))
	}
	return PaletteFromOpts(optsFn()), nil
}

// colorToHex converts a color.Color to its "#rrggbb" hex string.
// Returns empty string for nil colors.
func colorToHex(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// resolveColor parses a hex string into a color.Color, falling back to
// the provided default when the string is empty.
func resolveColor(hexStr string, fallback color.Color) color.Color {
	if hexStr == "" {
		return fallback
	}
	return lipgloss.Color(hexStr)
}
