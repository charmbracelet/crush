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
		"charm":                DefaultPalette(),
		"catppuccin-mocha":     catppuccinMocha(),
		"catppuccin-macchiato": catppuccinMacchiato(),
		"catppuccin-latte":     catppuccinLatte(),
		"dracula":              dracula(),
		"everforest-dark":      everforestDark(),
		"everforest-light":     everforestLight(),
		"gruvbox-dark":         gruvboxDark(),
		"gruvbox-light":        gruvboxLight(),
		"kanagawa-wave":        kanagawaWave(),
		"kanagawa-lotus":       kanagawaLotus(),
		"material-darker":      materialDarker(),
		"material-lighter":     materialLighter(),
		"nord":                 nord(),
		"one-dark":             oneDark(),
		"rose-pine":            rosePine(),
		"rose-pine-moon":       rosePineMoon(),
		"rose-pine-dawn":       rosePineDawn(),
		"solarized-dark":       solarizedDark(),
		"solarized-light":      solarizedLight(),
		"tokyo-night":          tokyoNight(),
		"tokyo-night-day":      tokyoNightDay(),
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

// DiscoveredTheme holds metadata about a theme found on disk.
type DiscoveredTheme struct {
	Name string
	Path string
}

// DiscoverThemes scans ~/.config/crush/themes/ for custom theme JSON files.
// It returns themes sorted by name, skipping files that fail to parse or
// that shadow a built-in theme name.
func DiscoverThemes() []DiscoveredTheme {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	dir := filepath.Join(home, ".config", "crush", "themes")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var themes []DiscoveredTheme
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}

		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		palette, err := ParseTheme(data)
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		if palette.Name != "" {
			name = palette.Name
		}

		if _, isBuiltin := builtinThemes[strings.ToLower(name)]; isBuiltin {
			continue
		}

		themes = append(themes, DiscoveredTheme{Name: name, Path: path})
	}

	sort.Slice(themes, func(i, j int) bool {
		return themes[i].Name < themes[j].Name
	})
	return themes
}

// AllThemeNames returns built-in theme names followed by discovered custom
// theme names, each group sorted alphabetically.
func AllThemeNames() (builtins []string, custom []DiscoveredTheme) {
	return BuiltinThemeNames(), DiscoverThemes()
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

func catppuccinMacchiato() ThemePalette {
	return ThemePalette{
		Name:   "Catppuccin Macchiato",
		Author: "Catppuccin contributors",
		Colors: ThemeColors{
			Primary:   "#8aadf4",
			Secondary: "#c6a0f6",
			Tertiary:  "#a6da95",

			BgBase:        "#24273a",
			BgBaseLighter: "#363a4f",
			BgSubtle:      "#494d64",
			BgOverlay:     "#5b6078",

			FgBase:      "#cad3f5",
			FgMuted:     "#8087a2",
			FgHalfMuted: "#939ab7",
			FgSubtle:    "#6e738d",

			Border:      "#494d64",
			BorderFocus: "#8aadf4",

			Error:   "#ed8796",
			Warning: "#eed49f",
			Info:    "#91d7e3",

			White:      "#cad3f5",
			BlueLight:  "#7dc4e4",
			Blue:       "#8aadf4",
			BlueDark:   "#5b6078",
			GreenLight: "#a6da95",
			Green:      "#a6da95",
			GreenDark:  "#8bd5ca",
			Red:        "#ed8796",
			RedDark:    "#ee99a0",
			Yellow:     "#eed49f",

			DiffInsertFg:      "#a6da95",
			DiffInsertBg:      "#20362a",
			DiffInsertBgLight: "#2d4635",
			DiffDeleteFg:      "#ed8796",
			DiffDeleteBg:      "#3a202c",
			DiffDeleteBgLight: "#472d38",
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

func nord() ThemePalette {
	return ThemePalette{
		Name:   "Nord",
		Author: "Arctic Ice Studio",
		Colors: ThemeColors{
			Primary:   "#88c0d0",
			Secondary: "#b48ead",
			Tertiary:  "#a3be8c",

			BgBase:        "#2e3440",
			BgBaseLighter: "#3b4252",
			BgSubtle:      "#434c5e",
			BgOverlay:     "#4c566a",

			FgBase:      "#d8dee9",
			FgMuted:     "#81a1c1",
			FgHalfMuted: "#93a1a1",
			FgSubtle:    "#616e88",

			Border:      "#434c5e",
			BorderFocus: "#88c0d0",

			Error:   "#bf616a",
			Warning: "#ebcb8b",
			Info:    "#88c0d0",

			White:      "#eceff4",
			BlueLight:  "#88c0d0",
			Blue:       "#81a1c1",
			BlueDark:   "#4c566a",
			GreenLight: "#a3be8c",
			Green:      "#a3be8c",
			GreenDark:  "#8fbcbb",
			Red:        "#bf616a",
			RedDark:    "#d08770",
			Yellow:     "#ebcb8b",

			DiffInsertFg:      "#a3be8c",
			DiffInsertBg:      "#2e3b2e",
			DiffInsertBgLight: "#3b4a3b",
			DiffDeleteFg:      "#bf616a",
			DiffDeleteBg:      "#3b2e30",
			DiffDeleteBgLight: "#4a3b3d",
		},
	}
}

func dracula() ThemePalette {
	return ThemePalette{
		Name:   "Dracula",
		Author: "Zeno Rocha",
		Colors: ThemeColors{
			Primary:   "#bd93f9",
			Secondary: "#ff79c6",
			Tertiary:  "#50fa7b",

			BgBase:        "#282a36",
			BgBaseLighter: "#44475a",
			BgSubtle:      "#44475a",
			BgOverlay:     "#6272a4",

			FgBase:      "#f8f8f2",
			FgMuted:     "#6272a4",
			FgHalfMuted: "#9ea4b3",
			FgSubtle:    "#6272a4",

			Border:      "#44475a",
			BorderFocus: "#bd93f9",

			Error:   "#ff5555",
			Warning: "#f1fa8c",
			Info:    "#8be9fd",

			White:      "#f8f8f2",
			BlueLight:  "#8be9fd",
			Blue:       "#8be9fd",
			BlueDark:   "#6272a4",
			GreenLight: "#50fa7b",
			Green:      "#50fa7b",
			GreenDark:  "#50fa7b",
			Red:        "#ff5555",
			RedDark:    "#ff5555",
			Yellow:     "#f1fa8c",

			DiffInsertFg:      "#50fa7b",
			DiffInsertBg:      "#1e3a1e",
			DiffInsertBgLight: "#2b4a2b",
			DiffDeleteFg:      "#ff5555",
			DiffDeleteBg:      "#3a1e1e",
			DiffDeleteBgLight: "#4a2b2b",
		},
	}
}

func tokyoNight() ThemePalette {
	return ThemePalette{
		Name:   "Tokyo Night",
		Author: "Folke Lemaitre",
		Colors: ThemeColors{
			Primary:   "#7aa2f7",
			Secondary: "#bb9af7",
			Tertiary:  "#9ece6a",

			BgBase:        "#1a1b26",
			BgBaseLighter: "#292e42",
			BgSubtle:      "#283457",
			BgOverlay:     "#565f89",

			FgBase:      "#c0caf5",
			FgMuted:     "#565f89",
			FgHalfMuted: "#737aa2",
			FgSubtle:    "#565f89",

			Border:      "#283457",
			BorderFocus: "#7aa2f7",

			Error:   "#f7768e",
			Warning: "#e0af68",
			Info:    "#7dcfff",

			White:      "#c0caf5",
			BlueLight:  "#7dcfff",
			Blue:       "#7aa2f7",
			BlueDark:   "#3d59a1",
			GreenLight: "#9ece6a",
			Green:      "#9ece6a",
			GreenDark:  "#73daca",
			Red:        "#f7768e",
			RedDark:    "#db4b4b",
			Yellow:     "#e0af68",

			DiffInsertFg:      "#9ece6a",
			DiffInsertBg:      "#1a2e1a",
			DiffInsertBgLight: "#253525",
			DiffDeleteFg:      "#f7768e",
			DiffDeleteBg:      "#2e1a1e",
			DiffDeleteBgLight: "#352528",
		},
	}
}

func tokyoNightDay() ThemePalette {
	return ThemePalette{
		Name:   "Tokyo Night Day",
		Author: "Folke Lemaitre",
		Colors: ThemeColors{
			Primary:   "#2e7de9",
			Secondary: "#9854f1",
			Tertiary:  "#587539",

			BgBase:        "#e1e2e7",
			BgBaseLighter: "#c4c8da",
			BgSubtle:      "#b7c1e3",
			BgOverlay:     "#848cb5",

			FgBase:      "#3760bf",
			FgMuted:     "#848cb5",
			FgHalfMuted: "#6172b0",
			FgSubtle:    "#848cb5",

			Border:      "#b7c1e3",
			BorderFocus: "#2e7de9",

			Error:   "#f52a65",
			Warning: "#8c6c3e",
			Info:    "#007197",

			White:      "#3760bf",
			BlueLight:  "#007197",
			Blue:       "#2e7de9",
			BlueDark:   "#b7c1e3",
			GreenLight: "#587539",
			Green:      "#587539",
			GreenDark:  "#387068",
			Red:        "#f52a65",
			RedDark:    "#c64343",
			Yellow:     "#8c6c3e",

			DiffInsertFg:      "#587539",
			DiffInsertBg:      "#d5e5d0",
			DiffInsertBgLight: "#e0eddb",
			DiffDeleteFg:      "#f52a65",
			DiffDeleteBg:      "#f0d0d5",
			DiffDeleteBgLight: "#f5dde1",
		},
	}
}

func rosePine() ThemePalette {
	return ThemePalette{
		Name:   "Rosé Pine",
		Author: "Rosé Pine",
		Colors: ThemeColors{
			Primary:   "#c4a7e7",
			Secondary: "#ebbcba",
			Tertiary:  "#31748f",

			BgBase:        "#191724",
			BgBaseLighter: "#1f1d2e",
			BgSubtle:      "#26233a",
			BgOverlay:     "#524f67",

			FgBase:      "#e0def4",
			FgMuted:     "#6e6a86",
			FgHalfMuted: "#908caa",
			FgSubtle:    "#6e6a86",

			Border:      "#26233a",
			BorderFocus: "#c4a7e7",

			Error:   "#eb6f92",
			Warning: "#f6c177",
			Info:    "#9ccfd8",

			White:      "#e0def4",
			BlueLight:  "#9ccfd8",
			Blue:       "#31748f",
			BlueDark:   "#524f67",
			GreenLight: "#9ccfd8",
			Green:      "#31748f",
			GreenDark:  "#9ccfd8",
			Red:        "#eb6f92",
			RedDark:    "#eb6f92",
			Yellow:     "#f6c177",

			DiffInsertFg:      "#9ccfd8",
			DiffInsertBg:      "#1a2a2e",
			DiffInsertBgLight: "#253538",
			DiffDeleteFg:      "#eb6f92",
			DiffDeleteBg:      "#2e1a24",
			DiffDeleteBgLight: "#38252e",
		},
	}
}

func rosePineMoon() ThemePalette {
	return ThemePalette{
		Name:   "Rosé Pine Moon",
		Author: "Rosé Pine",
		Colors: ThemeColors{
			Primary:   "#c4a7e7",
			Secondary: "#ea9a97",
			Tertiary:  "#3e8fb0",

			BgBase:        "#232136",
			BgBaseLighter: "#2a273f",
			BgSubtle:      "#393552",
			BgOverlay:     "#56526e",

			FgBase:      "#e0def4",
			FgMuted:     "#6e6a86",
			FgHalfMuted: "#908caa",
			FgSubtle:    "#6e6a86",

			Border:      "#393552",
			BorderFocus: "#c4a7e7",

			Error:   "#eb6f92",
			Warning: "#f6c177",
			Info:    "#9ccfd8",

			White:      "#e0def4",
			BlueLight:  "#9ccfd8",
			Blue:       "#3e8fb0",
			BlueDark:   "#56526e",
			GreenLight: "#9ccfd8",
			Green:      "#3e8fb0",
			GreenDark:  "#9ccfd8",
			Red:        "#eb6f92",
			RedDark:    "#eb6f92",
			Yellow:     "#f6c177",

			DiffInsertFg:      "#9ccfd8",
			DiffInsertBg:      "#22303a",
			DiffInsertBgLight: "#2d3b44",
			DiffDeleteFg:      "#eb6f92",
			DiffDeleteBg:      "#36222e",
			DiffDeleteBgLight: "#402d38",
		},
	}
}

func rosePineDawn() ThemePalette {
	return ThemePalette{
		Name:   "Rosé Pine Dawn",
		Author: "Rosé Pine",
		Colors: ThemeColors{
			Primary:   "#907aa9",
			Secondary: "#d7827e",
			Tertiary:  "#286983",

			BgBase:        "#faf4ed",
			BgBaseLighter: "#fffaf3",
			BgSubtle:      "#f2e9e1",
			BgOverlay:     "#cecacd",

			FgBase:      "#575279",
			FgMuted:     "#9893a5",
			FgHalfMuted: "#797593",
			FgSubtle:    "#9893a5",

			Border:      "#f2e9e1",
			BorderFocus: "#907aa9",

			Error:   "#b4637a",
			Warning: "#ea9d34",
			Info:    "#56949f",

			White:      "#575279",
			BlueLight:  "#56949f",
			Blue:       "#286983",
			BlueDark:   "#cecacd",
			GreenLight: "#56949f",
			Green:      "#286983",
			GreenDark:  "#56949f",
			Red:        "#b4637a",
			RedDark:    "#b4637a",
			Yellow:     "#ea9d34",

			DiffInsertFg:      "#286983",
			DiffInsertBg:      "#e0ede8",
			DiffInsertBgLight: "#eaf4ef",
			DiffDeleteFg:      "#b4637a",
			DiffDeleteBg:      "#f0e0e3",
			DiffDeleteBgLight: "#f5eaec",
		},
	}
}

func solarizedDark() ThemePalette {
	return ThemePalette{
		Name:   "Solarized Dark",
		Author: "Ethan Schoonover",
		Colors: ThemeColors{
			Primary:   "#268bd2",
			Secondary: "#6c71c4",
			Tertiary:  "#859900",

			BgBase:        "#002b36",
			BgBaseLighter: "#073642",
			BgSubtle:      "#073642",
			BgOverlay:     "#586e75",

			FgBase:      "#839496",
			FgMuted:     "#586e75",
			FgHalfMuted: "#657b83",
			FgSubtle:    "#586e75",

			Border:      "#073642",
			BorderFocus: "#268bd2",

			Error:   "#dc322f",
			Warning: "#b58900",
			Info:    "#2aa198",

			White:      "#eee8d5",
			BlueLight:  "#2aa198",
			Blue:       "#268bd2",
			BlueDark:   "#073642",
			GreenLight: "#859900",
			Green:      "#859900",
			GreenDark:  "#2aa198",
			Red:        "#dc322f",
			RedDark:    "#cb4b16",
			Yellow:     "#b58900",

			DiffInsertFg:      "#859900",
			DiffInsertBg:      "#003a1a",
			DiffInsertBgLight: "#004a25",
			DiffDeleteFg:      "#dc322f",
			DiffDeleteBg:      "#3a0a0a",
			DiffDeleteBgLight: "#4a1515",
		},
	}
}

func solarizedLight() ThemePalette {
	return ThemePalette{
		Name:   "Solarized Light",
		Author: "Ethan Schoonover",
		Colors: ThemeColors{
			Primary:   "#268bd2",
			Secondary: "#6c71c4",
			Tertiary:  "#859900",

			BgBase:        "#fdf6e3",
			BgBaseLighter: "#eee8d5",
			BgSubtle:      "#eee8d5",
			BgOverlay:     "#93a1a1",

			FgBase:      "#657b83",
			FgMuted:     "#93a1a1",
			FgHalfMuted: "#839496",
			FgSubtle:    "#93a1a1",

			Border:      "#eee8d5",
			BorderFocus: "#268bd2",

			Error:   "#dc322f",
			Warning: "#b58900",
			Info:    "#2aa198",

			White:      "#073642",
			BlueLight:  "#2aa198",
			Blue:       "#268bd2",
			BlueDark:   "#eee8d5",
			GreenLight: "#859900",
			Green:      "#859900",
			GreenDark:  "#2aa198",
			Red:        "#dc322f",
			RedDark:    "#cb4b16",
			Yellow:     "#b58900",

			DiffInsertFg:      "#859900",
			DiffInsertBg:      "#e8f0d0",
			DiffInsertBgLight: "#f0f5e0",
			DiffDeleteFg:      "#dc322f",
			DiffDeleteBg:      "#f0d5d0",
			DiffDeleteBgLight: "#f5e2de",
		},
	}
}

func oneDark() ThemePalette {
	return ThemePalette{
		Name:   "One Dark",
		Author: "Atom",
		Colors: ThemeColors{
			Primary:   "#61afef",
			Secondary: "#c678dd",
			Tertiary:  "#98c379",

			BgBase:        "#282c34",
			BgBaseLighter: "#31353f",
			BgSubtle:      "#393f4a",
			BgOverlay:     "#5c6370",

			FgBase:      "#abb2bf",
			FgMuted:     "#5c6370",
			FgHalfMuted: "#848b98",
			FgSubtle:    "#5c6370",

			Border:      "#393f4a",
			BorderFocus: "#61afef",

			Error:   "#e86671",
			Warning: "#e5c07b",
			Info:    "#56b6c2",

			White:      "#abb2bf",
			BlueLight:  "#56b6c2",
			Blue:       "#61afef",
			BlueDark:   "#5c6370",
			GreenLight: "#98c379",
			Green:      "#98c379",
			GreenDark:  "#56b6c2",
			Red:        "#e86671",
			RedDark:    "#993939",
			Yellow:     "#e5c07b",

			DiffInsertFg:      "#98c379",
			DiffInsertBg:      "#2a3325",
			DiffInsertBgLight: "#344030",
			DiffDeleteFg:      "#e86671",
			DiffDeleteBg:      "#33252a",
			DiffDeleteBgLight: "#3e3035",
		},
	}
}

func kanagawaWave() ThemePalette {
	return ThemePalette{
		Name:   "Kanagawa Wave",
		Author: "rebelot",
		Colors: ThemeColors{
			Primary:   "#7e9cd8",
			Secondary: "#957fb8",
			Tertiary:  "#98bb6c",

			BgBase:        "#1f1f28",
			BgBaseLighter: "#2a2a37",
			BgSubtle:      "#363646",
			BgOverlay:     "#54546d",

			FgBase:      "#dcd7ba",
			FgMuted:     "#727169",
			FgHalfMuted: "#c8c093",
			FgSubtle:    "#727169",

			Border:      "#363646",
			BorderFocus: "#7e9cd8",

			Error:   "#e82424",
			Warning: "#ff9e3b",
			Info:    "#658594",

			White:      "#dcd7ba",
			BlueLight:  "#7fb4ca",
			Blue:       "#7e9cd8",
			BlueDark:   "#54546d",
			GreenLight: "#98bb6c",
			Green:      "#98bb6c",
			GreenDark:  "#7aa89f",
			Red:        "#e46876",
			RedDark:    "#c34043",
			Yellow:     "#e6c384",

			DiffInsertFg:      "#76946a",
			DiffInsertBg:      "#2b3328",
			DiffInsertBgLight: "#354035",
			DiffDeleteFg:      "#c34043",
			DiffDeleteBg:      "#43242b",
			DiffDeleteBgLight: "#4d2e35",
		},
	}
}

func kanagawaLotus() ThemePalette {
	return ThemePalette{
		Name:   "Kanagawa Lotus",
		Author: "rebelot",
		Colors: ThemeColors{
			Primary:   "#4d699b",
			Secondary: "#624c83",
			Tertiary:  "#6f894e",

			BgBase:        "#f2ecbc",
			BgBaseLighter: "#e7dba0",
			BgSubtle:      "#e5ddb0",
			BgOverlay:     "#d5cea3",

			FgBase:      "#545464",
			FgMuted:     "#8a8980",
			FgHalfMuted: "#716e61",
			FgSubtle:    "#8a8980",

			Border:      "#e5ddb0",
			BorderFocus: "#4d699b",

			Error:   "#e82424",
			Warning: "#e98a00",
			Info:    "#5a7785",

			White:      "#545464",
			BlueLight:  "#4e8ca2",
			Blue:       "#4d699b",
			BlueDark:   "#d5cea3",
			GreenLight: "#6f894e",
			Green:      "#6f894e",
			GreenDark:  "#597b75",
			Red:        "#c84053",
			RedDark:    "#d7474b",
			Yellow:     "#77713f",

			DiffInsertFg:      "#6e915f",
			DiffInsertBg:      "#b7d0ae",
			DiffInsertBgLight: "#cde0c5",
			DiffDeleteFg:      "#d7474b",
			DiffDeleteBg:      "#d9a594",
			DiffDeleteBgLight: "#e4bfb3",
		},
	}
}

func everforestDark() ThemePalette {
	return ThemePalette{
		Name:   "Everforest Dark",
		Author: "Sainnhe Park",
		Colors: ThemeColors{
			Primary:   "#a7c080",
			Secondary: "#d699b6",
			Tertiary:  "#83c092",

			BgBase:        "#2d353b",
			BgBaseLighter: "#343f44",
			BgSubtle:      "#3d484d",
			BgOverlay:     "#4f585e",

			FgBase:      "#d3c6aa",
			FgMuted:     "#7a8478",
			FgHalfMuted: "#859289",
			FgSubtle:    "#7a8478",

			Border:      "#3d484d",
			BorderFocus: "#a7c080",

			Error:   "#e67e80",
			Warning: "#dbbc7f",
			Info:    "#7fbbb3",

			White:      "#d3c6aa",
			BlueLight:  "#7fbbb3",
			Blue:       "#7fbbb3",
			BlueDark:   "#4f585e",
			GreenLight: "#a7c080",
			Green:      "#a7c080",
			GreenDark:  "#83c092",
			Red:        "#e67e80",
			RedDark:    "#e69875",
			Yellow:     "#dbbc7f",

			DiffInsertFg:      "#a7c080",
			DiffInsertBg:      "#2e3b2e",
			DiffInsertBgLight: "#3a4a3a",
			DiffDeleteFg:      "#e67e80",
			DiffDeleteBg:      "#3b2e30",
			DiffDeleteBgLight: "#4a3a3d",
		},
	}
}

func everforestLight() ThemePalette {
	return ThemePalette{
		Name:   "Everforest Light",
		Author: "Sainnhe Park",
		Colors: ThemeColors{
			Primary:   "#8da101",
			Secondary: "#df69ba",
			Tertiary:  "#35a77c",

			BgBase:        "#fdf6e3",
			BgBaseLighter: "#f4f0d9",
			BgSubtle:      "#efebd4",
			BgOverlay:     "#e0dcc7",

			FgBase:      "#5c6a72",
			FgMuted:     "#a6b0a0",
			FgHalfMuted: "#939f91",
			FgSubtle:    "#a6b0a0",

			Border:      "#efebd4",
			BorderFocus: "#8da101",

			Error:   "#f85552",
			Warning: "#dfa000",
			Info:    "#3a94c5",

			White:      "#5c6a72",
			BlueLight:  "#3a94c5",
			Blue:       "#3a94c5",
			BlueDark:   "#e0dcc7",
			GreenLight: "#8da101",
			Green:      "#8da101",
			GreenDark:  "#35a77c",
			Red:        "#f85552",
			RedDark:    "#f57d26",
			Yellow:     "#dfa000",

			DiffInsertFg:      "#8da101",
			DiffInsertBg:      "#e5ecc7",
			DiffInsertBgLight: "#eef3d8",
			DiffDeleteFg:      "#f85552",
			DiffDeleteBg:      "#f2d5d0",
			DiffDeleteBgLight: "#f7e3df",
		},
	}
}

func materialDarker() ThemePalette {
	return ThemePalette{
		Name:   "Material Darker",
		Author: "Mattia Astorino",
		Colors: ThemeColors{
			Primary:   "#82aaff",
			Secondary: "#c792ea",
			Tertiary:  "#c3e88d",

			BgBase:        "#212121",
			BgBaseLighter: "#292929",
			BgSubtle:      "#2a2a2a",
			BgOverlay:     "#474747",

			FgBase:      "#b0bec5",
			FgMuted:     "#616161",
			FgHalfMuted: "#8a9199",
			FgSubtle:    "#616161",

			Border:      "#292929",
			BorderFocus: "#82aaff",

			Error:   "#ff5370",
			Warning: "#ffcb6b",
			Info:    "#89ddff",

			White:      "#b0bec5",
			BlueLight:  "#89ddff",
			Blue:       "#82aaff",
			BlueDark:   "#474747",
			GreenLight: "#c3e88d",
			Green:      "#c3e88d",
			GreenDark:  "#89ddff",
			Red:        "#f07178",
			RedDark:    "#ff5370",
			Yellow:     "#ffcb6b",

			DiffInsertFg:      "#c3e88d",
			DiffInsertBg:      "#1e2e1a",
			DiffInsertBgLight: "#2b3b25",
			DiffDeleteFg:      "#f07178",
			DiffDeleteBg:      "#2e1a1a",
			DiffDeleteBgLight: "#3b2525",
		},
	}
}

func materialLighter() ThemePalette {
	return ThemePalette{
		Name:   "Material Lighter",
		Author: "Mattia Astorino",
		Colors: ThemeColors{
			Primary:   "#6182b8",
			Secondary: "#7c4dff",
			Tertiary:  "#91b859",

			BgBase:        "#fafafa",
			BgBaseLighter: "#ffffff",
			BgSubtle:      "#eeeeee",
			BgOverlay:     "#d2d4d5",

			FgBase:      "#546e7a",
			FgMuted:     "#aabfc9",
			FgHalfMuted: "#8796a0",
			FgSubtle:    "#aabfc9",

			Border:      "#d3e1e8",
			BorderFocus: "#6182b8",

			Error:   "#e53935",
			Warning: "#f6a434",
			Info:    "#39adb5",

			White:      "#546e7a",
			BlueLight:  "#39adb5",
			Blue:       "#6182b8",
			BlueDark:   "#d2d4d5",
			GreenLight: "#91b859",
			Green:      "#91b859",
			GreenDark:  "#39adb5",
			Red:        "#e53935",
			RedDark:    "#f76d47",
			Yellow:     "#f6a434",

			DiffInsertFg:      "#91b859",
			DiffInsertBg:      "#e5f0d8",
			DiffInsertBgLight: "#eff5e8",
			DiffDeleteFg:      "#e53935",
			DiffDeleteBg:      "#f5d5d0",
			DiffDeleteBgLight: "#f8e3df",
		},
	}
}
