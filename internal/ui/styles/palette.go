package styles

import (
	"fmt"
	"image/color"
	"regexp"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/exp/charmtone"
)

// validHexColor matches "#rgb", "#rrggbb", "#rgba", or "#rrggbbaa".
var validHexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// validANSIColor matches ANSI color indices 0-255.
var validANSIColor = regexp.MustCompile(`^([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)

// charmtoneByName maps lowercase charmtone color names to their hex values.
// Built once at init time from the charmtone package.
var charmtoneByName map[string]string

func init() {
	charmtoneByName = make(map[string]string, len(charmtone.Keys()))
	for _, k := range charmtone.Keys() {
		charmtoneByName[strings.ToLower(k.String())] = k.Hex()
	}
	// Include aliases.
	charmtoneByName["ash"] = charmtone.Sash.Hex()
	charmtoneByName["charcoal"] = charmtone.Char.Hex()
}

// PaletteField describes a single customizable color token in a Palette.
// It pairs the JSON field name with getter/setter accessors so that
// validation, serialization checks, and UI editors can all iterate
// over the same ordered list without duplicating field names.
type PaletteField struct {
	Name string
	Get  func(Palette) string
	Set  func(*Palette, string)
}

// PaletteFields returns the ordered list of all customizable color
// tokens in a Palette. This is the single source of truth for field
// names, ordering, and access; Validate, knownThemeFields, and the
// theme editor all derive from it.
func PaletteFields() []PaletteField {
	return []PaletteField{
		{"primary", func(p Palette) string { return p.Primary }, func(p *Palette, v string) { p.Primary = v }},
		{"secondary", func(p Palette) string { return p.Secondary }, func(p *Palette, v string) { p.Secondary = v }},
		{"accent", func(p Palette) string { return p.Accent }, func(p *Palette, v string) { p.Accent = v }},
		{"keyword", func(p Palette) string { return p.Keyword }, func(p *Palette, v string) { p.Keyword = v }},
		{"fg_base", func(p Palette) string { return p.FgBase }, func(p *Palette, v string) { p.FgBase = v }},
		{"fg_subtle", func(p Palette) string { return p.FgSubtle }, func(p *Palette, v string) { p.FgSubtle = v }},
		{"fg_more_subtle", func(p Palette) string { return p.FgMoreSubtle }, func(p *Palette, v string) { p.FgMoreSubtle = v }},
		{"fg_most_subtle", func(p Palette) string { return p.FgMostSubtle }, func(p *Palette, v string) { p.FgMostSubtle = v }},
		{"bg_base", func(p Palette) string { return p.BgBase }, func(p *Palette, v string) { p.BgBase = v }},
		{"bg_most_visible", func(p Palette) string { return p.BgMostVisible }, func(p *Palette, v string) { p.BgMostVisible = v }},
		{"bg_less_visible", func(p Palette) string { return p.BgLessVisible }, func(p *Palette, v string) { p.BgLessVisible = v }},
		{"bg_least_visible", func(p Palette) string { return p.BgLeastVisible }, func(p *Palette, v string) { p.BgLeastVisible = v }},
		{"on_primary", func(p Palette) string { return p.OnPrimary }, func(p *Palette, v string) { p.OnPrimary = v }},
		{"separator", func(p Palette) string { return p.Separator }, func(p *Palette, v string) { p.Separator = v }},
		{"destructive", func(p Palette) string { return p.Destructive }, func(p *Palette, v string) { p.Destructive = v }},
		{"error", func(p Palette) string { return p.Error }, func(p *Palette, v string) { p.Error = v }},
		{"warning", func(p Palette) string { return p.Warning }, func(p *Palette, v string) { p.Warning = v }},
		{"warning_subtle", func(p Palette) string { return p.WarningSubtle }, func(p *Palette, v string) { p.WarningSubtle = v }},
		{"attention", func(p Palette) string { return p.Attention }, func(p *Palette, v string) { p.Attention = v }},
		{"busy", func(p Palette) string { return p.Busy }, func(p *Palette, v string) { p.Busy = v }},
		{"info", func(p Palette) string { return p.Info }, func(p *Palette, v string) { p.Info = v }},
		{"info_more_subtle", func(p Palette) string { return p.InfoMoreSubtle }, func(p *Palette, v string) { p.InfoMoreSubtle = v }},
		{"info_most_subtle", func(p Palette) string { return p.InfoMostSubtle }, func(p *Palette, v string) { p.InfoMostSubtle = v }},
		{"success", func(p Palette) string { return p.Success }, func(p *Palette, v string) { p.Success = v }},
		{"success_more_subtle", func(p Palette) string { return p.SuccessMoreSubtle }, func(p *Palette, v string) { p.SuccessMoreSubtle = v }},
		{"success_most_subtle", func(p Palette) string { return p.SuccessMostSubtle }, func(p *Palette, v string) { p.SuccessMostSubtle = v }},
		{"insert_fg", func(p Palette) string { return p.InsertFg }, func(p *Palette, v string) { p.InsertFg = v }},
		{"insert_bg", func(p Palette) string { return p.InsertBg }, func(p *Palette, v string) { p.InsertBg = v }},
		{"insert_gutter_bg", func(p Palette) string { return p.InsertGutterBg }, func(p *Palette, v string) { p.InsertGutterBg = v }},
		{"delete_fg", func(p Palette) string { return p.DeleteFg }, func(p *Palette, v string) { p.DeleteFg = v }},
		{"delete_bg", func(p Palette) string { return p.DeleteBg }, func(p *Palette, v string) { p.DeleteBg = v }},
		{"delete_gutter_bg", func(p Palette) string { return p.DeleteGutterBg }, func(p *Palette, v string) { p.DeleteGutterBg = v }},
		{"yolo", func(p Palette) string { return p.Yolo }, func(p *Palette, v string) { p.Yolo = v }},
		{"plan", func(p Palette) string { return p.Plan }, func(p *Palette, v string) { p.Plan = v }},
		{"plan_more_subtle", func(p Palette) string { return p.PlanMoreSubtle }, func(p *Palette, v string) { p.PlanMoreSubtle = v }},
		{"button", func(p Palette) string { return p.Button }, func(p *Palette, v string) { p.Button = v }},
		{"button_subtle", func(p Palette) string { return p.ButtonSubtle }, func(p *Palette, v string) { p.ButtonSubtle = v }},
		{"button_inactive", func(p Palette) string { return p.ButtonInactive }, func(p *Palette, v string) { p.ButtonInactive = v }},
		{"button_hovered", func(p Palette) string { return p.ButtonHovered }, func(p *Palette, v string) { p.ButtonHovered = v }},
		{"ansi_black", func(p Palette) string { return p.AnsiBlack }, func(p *Palette, v string) { p.AnsiBlack = v }},
		{"ansi_red", func(p Palette) string { return p.AnsiRed }, func(p *Palette, v string) { p.AnsiRed = v }},
		{"ansi_green", func(p Palette) string { return p.AnsiGreen }, func(p *Palette, v string) { p.AnsiGreen = v }},
		{"ansi_yellow", func(p Palette) string { return p.AnsiYellow }, func(p *Palette, v string) { p.AnsiYellow = v }},
		{"ansi_blue", func(p Palette) string { return p.AnsiBlue }, func(p *Palette, v string) { p.AnsiBlue = v }},
		{"ansi_magenta", func(p Palette) string { return p.AnsiMagenta }, func(p *Palette, v string) { p.AnsiMagenta = v }},
		{"ansi_cyan", func(p Palette) string { return p.AnsiCyan }, func(p *Palette, v string) { p.AnsiCyan = v }},
		{"ansi_white", func(p Palette) string { return p.AnsiWhite }, func(p *Palette, v string) { p.AnsiWhite = v }},
		{"ansi_bright_black", func(p Palette) string { return p.AnsiBrightBlack }, func(p *Palette, v string) { p.AnsiBrightBlack = v }},
		{"ansi_bright_red", func(p Palette) string { return p.AnsiBrightRed }, func(p *Palette, v string) { p.AnsiBrightRed = v }},
		{"ansi_bright_green", func(p Palette) string { return p.AnsiBrightGreen }, func(p *Palette, v string) { p.AnsiBrightGreen = v }},
		{"ansi_bright_yellow", func(p Palette) string { return p.AnsiBrightYellow }, func(p *Palette, v string) { p.AnsiBrightYellow = v }},
		{"ansi_bright_blue", func(p Palette) string { return p.AnsiBrightBlue }, func(p *Palette, v string) { p.AnsiBrightBlue = v }},
		{"ansi_bright_magenta", func(p Palette) string { return p.AnsiBrightMagenta }, func(p *Palette, v string) { p.AnsiBrightMagenta = v }},
		{"ansi_bright_cyan", func(p Palette) string { return p.AnsiBrightCyan }, func(p *Palette, v string) { p.AnsiBrightCyan = v }},
		{"ansi_bright_white", func(p Palette) string { return p.AnsiBrightWhite }, func(p *Palette, v string) { p.AnsiBrightWhite = v }},
	}
}

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
	Attention         string `json:"attention,omitempty"`
	Busy              string `json:"busy,omitempty"`
	Info              string `json:"info,omitempty"`
	InfoMoreSubtle    string `json:"info_more_subtle,omitempty"`
	InfoMostSubtle    string `json:"info_most_subtle,omitempty"`
	Success           string `json:"success,omitempty"`
	SuccessMoreSubtle string `json:"success_more_subtle,omitempty"`
	SuccessMostSubtle string `json:"success_most_subtle,omitempty"`

	InsertFg       string `json:"insert_fg,omitempty"`
	InsertBg       string `json:"insert_bg,omitempty"`
	InsertGutterBg string `json:"insert_gutter_bg,omitempty"`
	DeleteFg       string `json:"delete_fg,omitempty"`
	DeleteBg       string `json:"delete_bg,omitempty"`
	DeleteGutterBg string `json:"delete_gutter_bg,omitempty"`

	Yolo           string `json:"yolo,omitempty"`
	Plan           string `json:"plan,omitempty"`
	PlanMoreSubtle string `json:"plan_more_subtle,omitempty"`

	Button         string `json:"button,omitempty"`
	ButtonSubtle   string `json:"button_subtle,omitempty"`
	ButtonInactive string `json:"button_inactive,omitempty"`
	ButtonHovered  string `json:"button_hovered,omitempty"`

	AnsiBlack         string `json:"ansi_black,omitempty"`
	AnsiRed           string `json:"ansi_red,omitempty"`
	AnsiGreen         string `json:"ansi_green,omitempty"`
	AnsiYellow        string `json:"ansi_yellow,omitempty"`
	AnsiBlue          string `json:"ansi_blue,omitempty"`
	AnsiMagenta       string `json:"ansi_magenta,omitempty"`
	AnsiCyan          string `json:"ansi_cyan,omitempty"`
	AnsiWhite         string `json:"ansi_white,omitempty"`
	AnsiBrightBlack   string `json:"ansi_bright_black,omitempty"`
	AnsiBrightRed     string `json:"ansi_bright_red,omitempty"`
	AnsiBrightGreen   string `json:"ansi_bright_green,omitempty"`
	AnsiBrightYellow  string `json:"ansi_bright_yellow,omitempty"`
	AnsiBrightBlue    string `json:"ansi_bright_blue,omitempty"`
	AnsiBrightMagenta string `json:"ansi_bright_magenta,omitempty"`
	AnsiBrightCyan    string `json:"ansi_bright_cyan,omitempty"`
	AnsiBrightWhite   string `json:"ansi_bright_white,omitempty"`
}

// PaletteFromOpts extracts a Palette from quickStyleOpts, converting
// each color.Color to its "#rrggbb" hex representation. Diff tokens
// left unset stay empty so overrides can flow through the merge chain;
// derivation happens once, after all overrides are applied (see
// deriveDiffColors).
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
		Attention:         colorToHex(o.attention),
		Busy:              colorToHex(o.busy),
		Info:              colorToHex(o.info),
		InfoMoreSubtle:    colorToHex(o.infoMoreSubtle),
		InfoMostSubtle:    colorToHex(o.infoMostSubtle),
		Success:           colorToHex(o.success),
		SuccessMoreSubtle: colorToHex(o.successMoreSubtle),
		SuccessMostSubtle: colorToHex(o.successMostSubtle),

		InsertFg:       colorToHex(o.insertFg),
		InsertBg:       colorToHex(o.insertBg),
		InsertGutterBg: colorToHex(o.insertGutterBg),
		DeleteFg:       colorToHex(o.deleteFg),
		DeleteBg:       colorToHex(o.deleteBg),
		DeleteGutterBg: colorToHex(o.deleteGutterBg),

		Yolo:           colorToHex(o.yolo),
		Plan:           colorToHex(o.plan),
		PlanMoreSubtle: colorToHex(o.planMoreSubtle),

		Button:         colorToHex(o.button),
		ButtonSubtle:   colorToHex(o.buttonSubtle),
		ButtonInactive: colorToHex(o.buttonInactive),
		ButtonHovered:  colorToHex(o.buttonHovered),

		AnsiBlack:         colorToHex(o.ansiBlack),
		AnsiRed:           colorToHex(o.ansiRed),
		AnsiGreen:         colorToHex(o.ansiGreen),
		AnsiYellow:        colorToHex(o.ansiYellow),
		AnsiBlue:          colorToHex(o.ansiBlue),
		AnsiMagenta:       colorToHex(o.ansiMagenta),
		AnsiCyan:          colorToHex(o.ansiCyan),
		AnsiWhite:         colorToHex(o.ansiWhite),
		AnsiBrightBlack:   colorToHex(o.ansiBrightBlack),
		AnsiBrightRed:     colorToHex(o.ansiBrightRed),
		AnsiBrightGreen:   colorToHex(o.ansiBrightGreen),
		AnsiBrightYellow:  colorToHex(o.ansiBrightYellow),
		AnsiBrightBlue:    colorToHex(o.ansiBrightBlue),
		AnsiBrightMagenta: colorToHex(o.ansiBrightMagenta),
		AnsiBrightCyan:    colorToHex(o.ansiBrightCyan),
		AnsiBrightWhite:   colorToHex(o.ansiBrightWhite),
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
		attention:         resolveColor(p.Attention, base.attention),
		busy:              resolveColor(p.Busy, base.busy),
		info:              resolveColor(p.Info, base.info),
		infoMoreSubtle:    resolveColor(p.InfoMoreSubtle, base.infoMoreSubtle),
		infoMostSubtle:    resolveColor(p.InfoMostSubtle, base.infoMostSubtle),
		success:           resolveColor(p.Success, base.success),
		successMoreSubtle: resolveColor(p.SuccessMoreSubtle, base.successMoreSubtle),
		successMostSubtle: resolveColor(p.SuccessMostSubtle, base.successMostSubtle),

		insertFg:       resolveColor(p.InsertFg, base.insertFg),
		insertBg:       resolveColor(p.InsertBg, base.insertBg),
		insertGutterBg: resolveColor(p.InsertGutterBg, base.insertGutterBg),
		deleteFg:       resolveColor(p.DeleteFg, base.deleteFg),
		deleteBg:       resolveColor(p.DeleteBg, base.deleteBg),
		deleteGutterBg: resolveColor(p.DeleteGutterBg, base.deleteGutterBg),

		yolo:           resolveColor(p.Yolo, base.yolo),
		plan:           resolveColor(p.Plan, base.plan),
		planMoreSubtle: resolveColor(p.PlanMoreSubtle, base.planMoreSubtle),

		button:         resolveColor(p.Button, base.button),
		buttonSubtle:   resolveColor(p.ButtonSubtle, base.buttonSubtle),
		buttonInactive: resolveColor(p.ButtonInactive, base.buttonInactive),
		buttonHovered:  resolveColor(p.ButtonHovered, base.buttonHovered),

		ansiBlack:   resolveColor(p.AnsiBlack, base.ansiBlack),
		ansiRed:     resolveColor(p.AnsiRed, base.ansiRed),
		ansiGreen:   resolveColor(p.AnsiGreen, base.ansiGreen),
		ansiYellow:  resolveColor(p.AnsiYellow, base.ansiYellow),
		ansiBlue:    resolveColor(p.AnsiBlue, base.ansiBlue),
		ansiMagenta: resolveColor(p.AnsiMagenta, base.ansiMagenta),
		ansiCyan:    resolveColor(p.AnsiCyan, base.ansiCyan),
		ansiWhite:   resolveColor(p.AnsiWhite, base.ansiWhite),

		ansiBrightBlack:   resolveColor(p.AnsiBrightBlack, base.ansiBrightBlack),
		ansiBrightRed:     resolveColor(p.AnsiBrightRed, base.ansiBrightRed),
		ansiBrightGreen:   resolveColor(p.AnsiBrightGreen, base.ansiBrightGreen),
		ansiBrightYellow:  resolveColor(p.AnsiBrightYellow, base.ansiBrightYellow),
		ansiBrightBlue:    resolveColor(p.AnsiBrightBlue, base.ansiBrightBlue),
		ansiBrightMagenta: resolveColor(p.AnsiBrightMagenta, base.ansiBrightMagenta),
		ansiBrightCyan:    resolveColor(p.AnsiBrightCyan, base.ansiBrightCyan),
		ansiBrightWhite:   resolveColor(p.AnsiBrightWhite, base.ansiBrightWhite),
	}
}

// IsValidColor reports whether s is a valid color value: a hex code
// (#rgb, #rrggbb, #rgba, #rrggbbaa), an ANSI index (0-255), or a named
// charmtone color (case-insensitive).
func IsValidColor(s string) bool {
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "#") {
		return validHexColor.MatchString(s)
	}
	if validANSIColor.MatchString(s) {
		return true
	}
	_, ok := charmtoneByName[strings.ToLower(s)]
	return ok
}

// ParseColor normalizes a supported color for storage and display. It accepts
// hex codes, ANSI indices (0-255), and named Charmtone colors. Colors matching
// a Charmtone entry use its canonical name; other valid values are returned
// unchanged. Unrecognized input returns an empty string.
func ParseColor(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "#") {
		if validHexColor.MatchString(s) {
			if name := charmtone.NameFromHex(s); name != "" {
				return name
			}
			return s
		}
		return ""
	}
	if validANSIColor.MatchString(s) {
		return s
	}
	if hex, ok := charmtoneByName[strings.ToLower(s)]; ok {
		if name := charmtone.NameFromHex(hex); name != "" {
			return name
		}
		return hex
	}
	return ""
}

// Validate checks that all non-empty color strings in the palette are
// valid color values (hex, ANSI 0-255, or charmtone name). Returns an
// error listing all invalid fields.
func (p Palette) Validate() error {
	var errs []string
	for _, f := range PaletteFields() {
		if v := f.Get(p); v != "" && !IsValidColor(v) {
			errs = append(errs, fmt.Sprintf("%s: invalid color %q", f.Name, v))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid palette colors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// IsEmpty reports whether no palette colors are set, meaning the theme
// inherits everything from its base.
func (p Palette) IsEmpty() bool {
	for _, f := range PaletteFields() {
		if f.Get(p) != "" {
			return false
		}
	}
	return true
}

// ThemePalette returns the Palette for a built-in theme by name.
// Returns an error if the theme is not recognized.
func ThemePalette(name string) (Palette, error) {
	optsFn, ok := builtinThemes[normalizeThemeName(name)]
	if !ok {
		return Palette{}, fmt.Errorf("unknown theme %q; available themes: %s", name, strings.Join(BuiltinThemeNames(), ", "))
	}
	opts := optsFn()
	opts.deriveDiffColors()
	return PaletteFromOpts(opts), nil
}

// MergePalette applies palette overrides on top of a built-in base theme
// and returns the fully resolved palette. Empty baseName uses Charmtone.
// MergePalette applies palette overrides on top of a built-in or user theme
// and returns the fully resolved palette. Empty baseName uses Charmtone.
func MergePalette(baseName string, palette Palette) (Palette, error) {
	base, root, err := resolveThemePalette(baseName, map[string]bool{})
	if err != nil {
		return Palette{}, err
	}
	if err := palette.Validate(); err != nil {
		return Palette{}, err
	}
	rootOpts, err := builtinThemeOpts(root)
	if err != nil {
		return Palette{}, err
	}
	opts := palette.ToQuickStyleOpts(base.ToQuickStyleOpts(rootOpts))
	opts.deriveDiffColors()
	return PaletteFromOpts(opts), nil
}

// baseThemeOpts returns the quickStyleOpts of the named built-in theme.
// An empty name yields the default Charmtone palette.
// resolveThemePalette returns the fully resolved palette and root built-in for
// a theme. User themes may inherit from other user themes; cycles are rejected.
func resolveThemePalette(name string, visiting map[string]bool) (Palette, string, error) {
	if name == "" {
		name = "charmtone-panther"
	}
	key := normalizeThemeName(name)
	if visiting[key] {
		return Palette{}, "", fmt.Errorf("theme inheritance cycle at %q", key)
	}
	visiting[key] = true
	defer delete(visiting, key)

	if path, err := FindThemeFile(key); err == nil {
		tf, err := LoadThemeFile(path)
		if err != nil {
			return Palette{}, "", err
		}
		baseName := tf.Base
		if baseName == "" {
			baseName = "charmtone-panther"
		}
		var base Palette
		var root string
		if strings.EqualFold(baseName, key) && IsBuiltinTheme(key) {
			opts, err := builtinThemeOpts(key)
			if err != nil {
				return Palette{}, "", err
			}
			base = PaletteFromOpts(opts)
			root = key
		} else {
			base, root, err = resolveThemePalette(baseName, visiting)
			if err != nil {
				return Palette{}, "", err
			}
		}
		if err := tf.Validate(); err != nil {
			return Palette{}, "", err
		}
		rootOpts, err := builtinThemeOpts(root)
		if err != nil {
			return Palette{}, "", err
		}
		return PaletteFromOpts(tf.ToQuickStyleOpts(base.ToQuickStyleOpts(rootOpts))), root, nil
	}

	optsFn, ok := builtinThemes[key]
	if !ok {
		return Palette{}, "", fmt.Errorf("unknown theme %q; available themes: %s", name, strings.Join(BuiltinThemeNames(), ", "))
	}
	return PaletteFromOpts(optsFn()), key, nil
}

// builtinThemeOpts returns the quickStyleOpts of a built-in theme.
func builtinThemeOpts(name string) (quickStyleOpts, error) {
	if name == "" {
		name = "charmtone-panther"
	}
	optsFn, ok := builtinThemes[normalizeThemeName(name)]
	if !ok {
		return quickStyleOpts{}, fmt.Errorf("unknown theme %q; available themes: %s", name, strings.Join(BuiltinThemeNames(), ", "))
	}
	return optsFn(), nil
}

// LoadPaletteTheme builds Styles by applying palette overrides on top of
// a built-in base theme. Empty baseName uses the default Charmtone theme.
// LoadPaletteTheme builds Styles by applying palette overrides on top of a
// built-in or user theme. Empty baseName uses the default Charmtone theme.
func LoadPaletteTheme(baseName string, palette Palette) (Styles, error) {
	base, root, err := resolveThemePalette(baseName, map[string]bool{})
	if err != nil {
		return Styles{}, err
	}
	if err := palette.Validate(); err != nil {
		return Styles{}, err
	}
	rootOpts, err := builtinThemeOpts(root)
	if err != nil {
		return Styles{}, err
	}
	s := quickStyle(palette.ToQuickStyleOpts(base.ToQuickStyleOpts(rootOpts)))
	// Root theme overrides hardcode built-in colors (e.g. the Charmtone
	// syntax palette) that only make sense for the untouched built-in.
	// Only apply them when the theme is unmodified, so user themes (e.g.
	// light themes) get fully token-driven styles.
	pure := palette.IsEmpty() && base == PaletteFromOpts(rootOpts)
	if pure {
		if override, ok := builtinThemeOverrides[root]; ok {
			s = override(s)
		}
	}
	return s, nil
}

// colorToHex returns a canonical display string for a color. If the color
// matches a Charmtone palette entry, its name is returned; otherwise the
// "#rrggbb" hex value is returned. Nil colors return an empty string.
func colorToHex(c color.Color) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	hex := fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	if name := charmtone.NameFromHex(hex); name != "" {
		return name
	}
	return hex
}

// ColorString returns a lipgloss-compatible color string for the given
// input. Charmtone names are converted to hex; hex and ANSI values are
// returned as-is. Returns empty string for unrecognized input.
func ColorString(s string) string {
	resolved := ParseColor(s)
	if resolved == "" {
		return ""
	}
	if hex, ok := charmtoneByName[strings.ToLower(resolved)]; ok {
		return hex
	}
	return resolved
}

// resolveColor parses a color string into a color.Color, falling back
// to the provided default when the string is empty. Supports hex codes,
// ANSI indices, and charmtone names.
func resolveColor(s string, fallback color.Color) color.Color {
	if s == "" {
		return fallback
	}
	cs := ColorString(s)
	if cs == "" {
		return fallback
	}
	return lipgloss.Color(cs)
}
