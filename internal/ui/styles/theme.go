package styles

import (
	"fmt"
	"image/color"
	"regexp"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// hexColorPattern validates hex color format: #RRGGBB or #RGB.
var hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// ThemePalette defines the complete color palette for a Crush theme.
// All fields are hex color strings (e.g. "#6B50FF").
type ThemePalette struct {
	Name   string `json:"name,omitempty"`
	Author string `json:"author,omitempty"`

	Colors ThemeColors `json:"colors"`
}

// ThemeColors defines the semantic color slots for a theme.
type ThemeColors struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
	Tertiary  string `json:"tertiary"`

	BgBase        string `json:"bg_base"`
	BgBaseLighter string `json:"bg_base_lighter"`
	BgSubtle      string `json:"bg_subtle"`
	BgOverlay     string `json:"bg_overlay"`

	FgBase      string `json:"fg_base"`
	FgMuted     string `json:"fg_muted"`
	FgHalfMuted string `json:"fg_half_muted"`
	FgSubtle    string `json:"fg_subtle"`

	Border      string `json:"border"`
	BorderFocus string `json:"border_focus"`

	Error   string `json:"error"`
	Warning string `json:"warning"`
	Info    string `json:"info"`

	White      string `json:"white"`
	BlueLight  string `json:"blue_light"`
	Blue       string `json:"blue"`
	BlueDark   string `json:"blue_dark"`
	GreenLight string `json:"green_light"`
	Green      string `json:"green"`
	GreenDark  string `json:"green_dark"`
	Red        string `json:"red"`
	RedDark    string `json:"red_dark"`
	Yellow     string `json:"yellow"`

	DiffInsertFg      string `json:"diff_insert_fg,omitempty"`
	DiffInsertBg      string `json:"diff_insert_bg,omitempty"`
	DiffInsertBgLight string `json:"diff_insert_bg_light,omitempty"`
	DiffDeleteFg      string `json:"diff_delete_fg,omitempty"`
	DiffDeleteBg      string `json:"diff_delete_bg,omitempty"`
	DiffDeleteBgLight string `json:"diff_delete_bg_light,omitempty"`
}

// hexColor resolves a hex string to a color.Color.
func hexColor(hex string) color.Color {
	return lipgloss.Color(hex)
}

// hexStr returns the hex string representation of a color.Color.
func hexStr(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// newHex returns a pointer to the hex string of a color, for use with
// glamour/ansi style fields.
func newHex(c color.Color) *string {
	s := hexStr(c)
	return &s
}

// requiredColorFields returns the list of required color field names and their
// values in deterministic order.
func requiredColorFields(tc *ThemeColors) []struct {
	Name  string
	Value string
} {
	return []struct {
		Name  string
		Value string
	}{
		{"bg_base", tc.BgBase},
		{"bg_base_lighter", tc.BgBaseLighter},
		{"bg_overlay", tc.BgOverlay},
		{"bg_subtle", tc.BgSubtle},
		{"blue", tc.Blue},
		{"blue_dark", tc.BlueDark},
		{"blue_light", tc.BlueLight},
		{"border", tc.Border},
		{"border_focus", tc.BorderFocus},
		{"error", tc.Error},
		{"fg_base", tc.FgBase},
		{"fg_half_muted", tc.FgHalfMuted},
		{"fg_muted", tc.FgMuted},
		{"fg_subtle", tc.FgSubtle},
		{"green", tc.Green},
		{"green_dark", tc.GreenDark},
		{"green_light", tc.GreenLight},
		{"info", tc.Info},
		{"primary", tc.Primary},
		{"red", tc.Red},
		{"red_dark", tc.RedDark},
		{"secondary", tc.Secondary},
		{"tertiary", tc.Tertiary},
		{"warning", tc.Warning},
		{"white", tc.White},
		{"yellow", tc.Yellow},
	}
}

func (tc *ThemeColors) validate() error {
	fields := requiredColorFields(tc)

	var missing []string
	var invalid []string
	for _, f := range fields {
		if f.Value == "" {
			missing = append(missing, f.Name)
		} else if !hexColorPattern.MatchString(f.Value) {
			invalid = append(invalid, fmt.Sprintf("%s (%q)", f.Name, f.Value))
		}
	}

	var errs []string
	if len(missing) > 0 {
		errs = append(errs, "missing required colors: "+strings.Join(missing, ", "))
	}

	optionalFields := []struct {
		Name  string
		Value string
	}{
		{"diff_insert_fg", tc.DiffInsertFg},
		{"diff_insert_bg", tc.DiffInsertBg},
		{"diff_insert_bg_light", tc.DiffInsertBgLight},
		{"diff_delete_fg", tc.DiffDeleteFg},
		{"diff_delete_bg", tc.DiffDeleteBg},
		{"diff_delete_bg_light", tc.DiffDeleteBgLight},
	}
	for _, f := range optionalFields {
		if f.Value != "" && !hexColorPattern.MatchString(f.Value) {
			invalid = append(invalid, fmt.Sprintf("%s (%q)", f.Name, f.Value))
		}
	}

	if len(invalid) > 0 {
		errs = append(errs, "invalid hex colors: "+strings.Join(invalid, ", "))
	}

	if len(errs) > 0 {
		return fmt.Errorf("theme validation: %s", strings.Join(errs, "; "))
	}
	return nil
}

// DiffDefaults computes sensible diff color defaults derived from the theme's
// own palette colors, rather than using hardcoded dark-theme values.
func (tc *ThemeColors) DiffDefaults() (insertFg, insertBg, insertBgLight, deleteFg, deleteBg, deleteBgLight string) {
	insertFg = tc.DiffInsertFg
	insertBg = tc.DiffInsertBg
	insertBgLight = tc.DiffInsertBgLight
	deleteFg = tc.DiffDeleteFg
	deleteBg = tc.DiffDeleteBg
	deleteBgLight = tc.DiffDeleteBgLight

	if insertFg == "" {
		insertFg = tc.Green
	}
	if deleteFg == "" {
		deleteFg = tc.Red
	}
	if insertBg == "" {
		insertBg = blendHex(tc.BgBase, tc.Green, 0.12)
	}
	if insertBgLight == "" {
		insertBgLight = blendHex(tc.BgBase, tc.Green, 0.18)
	}
	if deleteBg == "" {
		deleteBg = blendHex(tc.BgBase, tc.Red, 0.12)
	}
	if deleteBgLight == "" {
		deleteBgLight = blendHex(tc.BgBase, tc.Red, 0.18)
	}
	return
}

// blendHex linearly blends two hex colors by the given ratio (0.0 = base, 1.0 = accent).
func blendHex(baseHex, accentHex string, ratio float64) string {
	br, bg, bb := parseHexRGB(baseHex)
	ar, ag, ab := parseHexRGB(accentHex)
	r := uint8(float64(br)*(1-ratio) + float64(ar)*ratio)
	g := uint8(float64(bg)*(1-ratio) + float64(ag)*ratio)
	b := uint8(float64(bb)*(1-ratio) + float64(ab)*ratio)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func parseHexRGB(hex string) (r, g, b uint8) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return 0, 0, 0
	}
	var val uint32
	fmt.Sscanf(hex, "%06x", &val)
	return uint8(val >> 16), uint8(val >> 8), uint8(val)
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

// builtinThemes maps theme names to their palette definitions.
// Populated once during package-level var initialization.
var builtinThemes = buildBuiltinThemes()

// BuiltinThemeNames returns the names of all built-in themes, sorted.
func BuiltinThemeNames() []string {
	names := make([]string, 0, len(builtinThemes))
	for name := range builtinThemes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// LoadTheme loads a theme by built-in name. Returns the default palette
// for an empty name. Returns an error if the name is not recognized.
func LoadTheme(name string) (ThemePalette, error) {
	if name == "" {
		return DefaultPalette(), nil
	}

	if theme, ok := builtinThemes[strings.ToLower(name)]; ok {
		return theme, nil
	}

	return ThemePalette{}, fmt.Errorf("unknown theme %q; available themes: %s", name, strings.Join(BuiltinThemeNames(), ", "))
}
