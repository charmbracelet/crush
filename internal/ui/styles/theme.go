package styles

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

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

	White     string `json:"white"`
	BlueLight string `json:"blue_light"`
	Blue      string `json:"blue"`
	BlueDark  string `json:"blue_dark"`
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

// resolvedPalette holds parsed color.Color values ready for style construction.
type resolvedPalette struct {
	primary   color.Color
	secondary color.Color
	tertiary  color.Color

	bgBase        color.Color
	bgBaseLighter color.Color
	bgSubtle      color.Color
	bgOverlay     color.Color

	fgBase      color.Color
	fgMuted     color.Color
	fgHalfMuted color.Color
	fgSubtle    color.Color

	border      color.Color
	borderFocus color.Color

	errorColor   color.Color
	warningColor color.Color
	infoColor    color.Color

	white     color.Color
	blueLight color.Color
	blue      color.Color
	blueDark  color.Color
	greenLight color.Color
	green      color.Color
	greenDark  color.Color
	red        color.Color
	redDark    color.Color
	yellow     color.Color

	diffInsertFg      color.Color
	diffInsertBg      color.Color
	diffInsertBgLight color.Color
	diffDeleteFg      color.Color
	diffDeleteBg      color.Color
	diffDeleteBgLight color.Color
}

func (tc *ThemeColors) resolve() resolvedPalette {
	p := resolvedPalette{
		primary:       hexColor(tc.Primary),
		secondary:     hexColor(tc.Secondary),
		tertiary:      hexColor(tc.Tertiary),
		bgBase:        hexColor(tc.BgBase),
		bgBaseLighter: hexColor(tc.BgBaseLighter),
		bgSubtle:      hexColor(tc.BgSubtle),
		bgOverlay:     hexColor(tc.BgOverlay),
		fgBase:        hexColor(tc.FgBase),
		fgMuted:       hexColor(tc.FgMuted),
		fgHalfMuted:   hexColor(tc.FgHalfMuted),
		fgSubtle:      hexColor(tc.FgSubtle),
		border:        hexColor(tc.Border),
		borderFocus:   hexColor(tc.BorderFocus),
		errorColor:    hexColor(tc.Error),
		warningColor:  hexColor(tc.Warning),
		infoColor:     hexColor(tc.Info),
		white:         hexColor(tc.White),
		blueLight:     hexColor(tc.BlueLight),
		blue:          hexColor(tc.Blue),
		blueDark:      hexColor(tc.BlueDark),
		greenLight:    hexColor(tc.GreenLight),
		green:         hexColor(tc.Green),
		greenDark:     hexColor(tc.GreenDark),
		red:           hexColor(tc.Red),
		redDark:       hexColor(tc.RedDark),
		yellow:        hexColor(tc.Yellow),
	}

	p.diffInsertFg = hexColorOr(tc.DiffInsertFg, "#629657")
	p.diffInsertBg = hexColorOr(tc.DiffInsertBg, "#2b322a")
	p.diffInsertBgLight = hexColorOr(tc.DiffInsertBgLight, "#323931")
	p.diffDeleteFg = hexColorOr(tc.DiffDeleteFg, "#a45c59")
	p.diffDeleteBg = hexColorOr(tc.DiffDeleteBg, "#312929")
	p.diffDeleteBgLight = hexColorOr(tc.DiffDeleteBgLight, "#383030")

	return p
}

func hexColor(hex string) color.Color {
	return lipgloss.Color(hex)
}

func hexColorOr(hex, fallback string) color.Color {
	if hex == "" {
		return lipgloss.Color(fallback)
	}
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

// DefaultPalette returns the default Crush theme palette matching the
// current charmtone-based appearance.
func DefaultPalette() ThemePalette {
	return ThemePalette{
		Name:   "Charm",
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
var builtinThemes map[string]ThemePalette

func init() {
	builtinThemes = map[string]ThemePalette{
		"charm":             DefaultPalette(),
		"catppuccin-mocha":  catppuccinMocha(),
		"catppuccin-latte":  catppuccinLatte(),
		"gruvbox-dark":      gruvboxDark(),
		"gruvbox-light":     gruvboxLight(),
	}
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

// LoadTheme loads a theme by built-in name or file path.
func LoadTheme(nameOrPath string) (ThemePalette, error) {
	if nameOrPath == "" {
		return DefaultPalette(), nil
	}

	if theme, ok := builtinThemes[strings.ToLower(nameOrPath)]; ok {
		return theme, nil
	}

	expanded := nameOrPath
	if strings.HasPrefix(expanded, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ThemePalette{}, fmt.Errorf("expanding home dir: %w", err)
		}
		expanded = filepath.Join(home, expanded[2:])
	}

	data, err := os.ReadFile(expanded)
	if err != nil {
		return ThemePalette{}, fmt.Errorf("reading theme file %q: %w", expanded, err)
	}

	return ParseTheme(data)
}

// ParseTheme parses a theme palette from JSON.
func ParseTheme(data []byte) (ThemePalette, error) {
	var theme ThemePalette
	if err := json.Unmarshal(data, &theme); err != nil {
		return ThemePalette{}, fmt.Errorf("parsing theme: %w", err)
	}
	if err := theme.Colors.validate(); err != nil {
		return ThemePalette{}, err
	}
	return theme, nil
}

func (tc *ThemeColors) validate() error {
	required := map[string]string{
		"primary":        tc.Primary,
		"secondary":      tc.Secondary,
		"tertiary":       tc.Tertiary,
		"bg_base":        tc.BgBase,
		"bg_base_lighter": tc.BgBaseLighter,
		"bg_subtle":      tc.BgSubtle,
		"bg_overlay":     tc.BgOverlay,
		"fg_base":        tc.FgBase,
		"fg_muted":       tc.FgMuted,
		"fg_half_muted":  tc.FgHalfMuted,
		"fg_subtle":      tc.FgSubtle,
		"border":         tc.Border,
		"border_focus":   tc.BorderFocus,
		"error":          tc.Error,
		"warning":        tc.Warning,
		"info":           tc.Info,
		"white":          tc.White,
		"blue_light":     tc.BlueLight,
		"blue":           tc.Blue,
		"blue_dark":      tc.BlueDark,
		"green_light":    tc.GreenLight,
		"green":          tc.Green,
		"green_dark":     tc.GreenDark,
		"red":            tc.Red,
		"red_dark":       tc.RedDark,
		"yellow":         tc.Yellow,
	}

	var missing []string
	for name, val := range required {
		if val == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("theme missing required colors: %s", strings.Join(missing, ", "))
	}
	return nil
}

func catppuccinMocha() ThemePalette {
	return ThemePalette{
		Name:   "Catppuccin Mocha",
		Author: "Catppuccin contributors",
		Colors: ThemeColors{
			Primary:   "#89b4fa",
			Secondary: "#cba6f7",
			Tertiary:  "#a6e3a1",

			BgBase:        "#1e1e2e",
			BgBaseLighter: "#313244",
			BgSubtle:      "#45475a",
			BgOverlay:     "#585b70",

			FgBase:      "#cdd6f4",
			FgMuted:     "#7f849c",
			FgHalfMuted: "#9399b2",
			FgSubtle:    "#6c7086",

			Border:      "#45475a",
			BorderFocus: "#89b4fa",

			Error:   "#f38ba8",
			Warning: "#f9e2af",
			Info:    "#89dceb",

			White:      "#cdd6f4",
			BlueLight:  "#74c7ec",
			Blue:       "#89b4fa",
			BlueDark:   "#585b70",
			GreenLight: "#a6e3a1",
			Green:      "#a6e3a1",
			GreenDark:  "#94e2d5",
			Red:        "#f38ba8",
			RedDark:    "#eba0ac",
			Yellow:     "#f9e2af",

			DiffInsertFg:      "#a6e3a1",
			DiffInsertBg:      "#1e3a2c",
			DiffInsertBgLight: "#2b4a37",
			DiffDeleteFg:      "#f38ba8",
			DiffDeleteBg:      "#3e1e2e",
			DiffDeleteBgLight: "#4a2b37",
		},
	}
}

func catppuccinLatte() ThemePalette {
	return ThemePalette{
		Name:   "Catppuccin Latte",
		Author: "Catppuccin contributors",
		Colors: ThemeColors{
			Primary:   "#1e66f5",
			Secondary: "#8839ef",
			Tertiary:  "#40a02b",

			BgBase:        "#eff1f5",
			BgBaseLighter: "#e6e9ef",
			BgSubtle:      "#ccd0da",
			BgOverlay:     "#bcc0cc",

			FgBase:      "#4c4f69",
			FgMuted:     "#8c8fa1",
			FgHalfMuted: "#7c7f93",
			FgSubtle:    "#9ca0b0",

			Border:      "#ccd0da",
			BorderFocus: "#1e66f5",

			Error:   "#d20f39",
			Warning: "#df8e1d",
			Info:    "#04a5e5",

			White:      "#4c4f69",
			BlueLight:  "#209fb5",
			Blue:       "#1e66f5",
			BlueDark:   "#bcc0cc",
			GreenLight: "#40a02b",
			Green:      "#40a02b",
			GreenDark:  "#179299",
			Red:        "#d20f39",
			RedDark:    "#e64553",
			Yellow:     "#df8e1d",

			DiffInsertFg:      "#40a02b",
			DiffInsertBg:      "#d5f0d0",
			DiffInsertBgLight: "#e3f5df",
			DiffDeleteFg:      "#d20f39",
			DiffDeleteBg:      "#f5d0d5",
			DiffDeleteBgLight: "#fae3e6",
		},
	}
}

func gruvboxDark() ThemePalette {
	return ThemePalette{
		Name:   "Gruvbox Dark",
		Author: "morhetz",
		Colors: ThemeColors{
			Primary:   "#fabd2f",
			Secondary: "#d3869b",
			Tertiary:  "#b8bb26",

			BgBase:        "#282828",
			BgBaseLighter: "#3c3836",
			BgSubtle:      "#504945",
			BgOverlay:     "#665c54",

			FgBase:      "#ebdbb2",
			FgMuted:     "#a89984",
			FgHalfMuted: "#bdae93",
			FgSubtle:    "#928374",

			Border:      "#504945",
			BorderFocus: "#fabd2f",

			Error:   "#fb4934",
			Warning: "#fabd2f",
			Info:    "#83a598",

			White:      "#fbf1c7",
			BlueLight:  "#83a598",
			Blue:       "#83a598",
			BlueDark:   "#665c54",
			GreenLight: "#b8bb26",
			Green:      "#b8bb26",
			GreenDark:  "#8ec07c",
			Red:        "#fb4934",
			RedDark:    "#cc241d",
			Yellow:     "#fabd2f",

			DiffInsertFg:      "#b8bb26",
			DiffInsertBg:      "#32361a",
			DiffInsertBgLight: "#3d4220",
			DiffDeleteFg:      "#fb4934",
			DiffDeleteBg:      "#3c1f1e",
			DiffDeleteBgLight: "#462726",
		},
	}
}

func gruvboxLight() ThemePalette {
	return ThemePalette{
		Name:   "Gruvbox Light",
		Author: "morhetz",
		Colors: ThemeColors{
			Primary:   "#b57614",
			Secondary: "#8f3f71",
			Tertiary:  "#79740e",

			BgBase:        "#fbf1c7",
			BgBaseLighter: "#ebdbb2",
			BgSubtle:      "#d5c4a1",
			BgOverlay:     "#bdae93",

			FgBase:      "#3c3836",
			FgMuted:     "#7c6f64",
			FgHalfMuted: "#665c54",
			FgSubtle:    "#928374",

			Border:      "#d5c4a1",
			BorderFocus: "#b57614",

			Error:   "#9d0006",
			Warning: "#b57614",
			Info:    "#076678",

			White:      "#3c3836",
			BlueLight:  "#076678",
			Blue:       "#076678",
			BlueDark:   "#bdae93",
			GreenLight: "#79740e",
			Green:      "#79740e",
			GreenDark:  "#427b58",
			Red:        "#9d0006",
			RedDark:    "#cc241d",
			Yellow:     "#b57614",

			DiffInsertFg:      "#79740e",
			DiffInsertBg:      "#e8ecc7",
			DiffInsertBgLight: "#eff2d6",
			DiffDeleteFg:      "#9d0006",
			DiffDeleteBg:      "#f2d5d0",
			DiffDeleteBgLight: "#f7e3df",
		},
	}
}
