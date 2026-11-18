package styles

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
)

// SyntaxField describes a code highlighting token whose foreground color a
// host theme can set. Name is the token's key in a host theme's syntax map;
// Style returns the token's style in a chroma config.
type SyntaxField struct {
	Name  string
	Style func(*ansi.Chroma) *ansi.StylePrimitive
}

// SyntaxFields returns the ordered list of code highlighting tokens a host
// theme can color. This is the single source of truth for the syntax keys.
// They are the snake_case names of the [ansi.Chroma] fields that carry a
// foreground color; Error (a foreground on a background) and Background are
// left to the palette.
//
// Every chroma user in Crush (markdown code blocks, file views, diffs)
// derives its colors from Markdown.CodeBlock.Chroma, so a token colored here
// is colored everywhere.
func SyntaxFields() []SyntaxField {
	return []SyntaxField{
		{"text", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.Text }},
		{"comment", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.Comment }},
		{"comment_preproc", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.CommentPreproc }},
		{"keyword", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.Keyword }},
		{"keyword_reserved", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.KeywordReserved }},
		{"keyword_namespace", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.KeywordNamespace }},
		{"keyword_type", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.KeywordType }},
		{"operator", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.Operator }},
		{"punctuation", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.Punctuation }},
		{"name", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.Name }},
		{"name_builtin", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.NameBuiltin }},
		{"name_tag", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.NameTag }},
		{"name_attribute", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.NameAttribute }},
		{"name_class", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.NameClass }},
		{"name_constant", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.NameConstant }},
		{"name_decorator", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.NameDecorator }},
		{"name_exception", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.NameException }},
		{"name_function", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.NameFunction }},
		{"name_other", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.NameOther }},
		{"literal", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.Literal }},
		{"literal_number", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.LiteralNumber }},
		{"literal_date", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.LiteralDate }},
		{"literal_string", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.LiteralString }},
		{"literal_string_escape", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.LiteralStringEscape }},
		{"generic_deleted", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.GenericDeleted }},
		{"generic_emph", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.GenericEmph }},
		{"generic_inserted", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.GenericInserted }},
		{"generic_strong", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.GenericStrong }},
		{"generic_subheading", func(c *ansi.Chroma) *ansi.StylePrimitive { return &c.GenericSubheading }},
	}
}

// validHostHexColor matches "#rgb" and "#rrggbb", the hex forms lipgloss
// can render.
var validHostHexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// isValidHostColor reports whether s is a color a host theme may use. It is
// stricter than [IsValidColor]: "#rgba" and "#rrggbbaa" are rejected,
// because lipgloss renders them as no color at all and a host has no way to
// notice.
func isValidHostColor(s string) bool {
	if strings.HasPrefix(s, "#") {
		return validHostHexColor.MatchString(s)
	}
	return IsValidColor(s)
}

// hostSyntaxColor is one syntax override of a host theme.
type hostSyntaxColor struct {
	field SyntaxField
	color string
}

// parseHostTheme converts a host theme's palette and syntax maps. Unknown
// keys are returned as ignored ("palette.<key>" or "syntax.<key>"), empty
// values are treated as unset, and any invalid color fails the whole theme.
func parseHostTheme(palette, syntax map[string]string) (Palette, []hostSyntaxColor, []string, error) {
	var (
		p       Palette
		colors  []hostSyntaxColor
		ignored []string
		errs    []string
	)

	paletteFields := map[string]PaletteField{}
	for _, f := range PaletteFields() {
		paletteFields[f.Name] = f
	}
	for _, k := range slices.Sorted(maps.Keys(palette)) {
		f, ok := paletteFields[k]
		if !ok {
			ignored = append(ignored, "palette."+k)
			continue
		}
		v := palette[k]
		if v == "" {
			continue
		}
		if !isValidHostColor(v) {
			errs = append(errs, fmt.Sprintf("palette.%s: invalid color %q", k, v))
			continue
		}
		f.Set(&p, v)
	}

	syntaxFields := map[string]SyntaxField{}
	for _, f := range SyntaxFields() {
		syntaxFields[f.Name] = f
	}
	for _, k := range slices.Sorted(maps.Keys(syntax)) {
		f, ok := syntaxFields[k]
		if !ok {
			ignored = append(ignored, "syntax."+k)
			continue
		}
		v := syntax[k]
		if v == "" {
			continue
		}
		if !isValidHostColor(v) {
			errs = append(errs, fmt.Sprintf("syntax.%s: invalid color %q", k, v))
			continue
		}
		colors = append(colors, hostSyntaxColor{field: f, color: v})
	}

	if len(errs) > 0 {
		return Palette{}, nil, ignored, fmt.Errorf("invalid theme colors: %s", strings.Join(errs, "; "))
	}
	return p, colors, ignored, nil
}

// hostThemeBase resolves the theme a host theme builds on and returns its
// colors and its root built-in. An empty name is the built-in
// charmtone-panther, resolved without touching the disk (a user file that
// overrides charmtone-panther is not consulted). Any other name resolves the
// way Crush resolves theme names everywhere: a user theme file wins over a
// built-in of the same name.
func hostThemeBase(name string) (quickStyleOpts, string, error) {
	if name == "" {
		const root = "charmtone-panther"
		opts, err := builtinThemeOpts(root)
		return opts, root, err
	}
	base, root, err := resolveThemePalette(name, map[string]bool{})
	if err != nil {
		return quickStyleOpts{}, "", err
	}
	rootOpts, err := builtinThemeOpts(root)
	if err != nil {
		return quickStyleOpts{}, "", err
	}
	return base.ToQuickStyleOpts(rootOpts), root, nil
}

// ValidateHostTheme checks a theme supplied by a host application (see
// [HostThemeStyles]). It returns the keys it ignored because it does not
// know them, so a host written for a newer Crush keeps working, and an error
// naming every invalid color, or saying the base theme cannot be resolved.
func ValidateHostTheme(base string, palette, syntax map[string]string) ([]string, error) {
	_, _, ignored, err := parseHostTheme(palette, syntax)
	if err != nil {
		return ignored, err
	}
	if base != "" {
		if _, _, err := resolveThemePalette(base, map[string]bool{}); err != nil {
			return ignored, fmt.Errorf("base theme: %w", err)
		}
	}
	return ignored, nil
}

// HostThemeStyles builds Styles from a theme supplied by a host application
// that runs Crush in one of its panes. palette maps theme-file keys (see
// [PaletteFields]) to colors and syntax maps code highlighting keys (see
// [SyntaxFields]) to foreground colors; unknown keys are ignored and colors
// left out come from base (empty means charmtone-panther, see
// [hostThemeBase]). Colors are "#rgb", "#rrggbb", an ANSI index (0-255) or
// a CharmTone name.
//
// The base theme's hardcoded overrides are kept only when the resulting
// palette is exactly the base built-in's, so a host that mirrors a built-in
// gets that built-in's look rather than an approximation of it. Any other
// palette gets legible badge text instead (see [legibleBadges]).
func HostThemeStyles(base string, palette, syntax map[string]string) (Styles, error) {
	p, colors, _, err := parseHostTheme(palette, syntax)
	if err != nil {
		return Styles{}, err
	}
	baseOpts, root, err := hostThemeBase(base)
	if err != nil {
		return Styles{}, fmt.Errorf("base theme: %w", err)
	}

	opts := p.ToQuickStyleOpts(baseOpts)
	s := quickStyle(opts)
	if !isBuiltinPalette(opts, root) {
		s = legibleBadges(s, opts)
	} else if override, ok := builtinThemeOverrides[root]; ok {
		s = override(s)
	}
	if chroma := s.Markdown.CodeBlock.Chroma; chroma != nil {
		// PlanMarkdown shares this config, so its code blocks follow too.
		for _, c := range colors {
			c.field.Style(chroma).Color = hex(lipgloss.Color(ColorString(c.color)))
		}
	}
	return s, nil
}

// isBuiltinPalette reports whether opts resolves to exactly the colors of
// the named built-in theme.
func isBuiltinPalette(opts quickStyleOpts, name string) bool {
	ref, err := builtinThemeOpts(name)
	if err != nil {
		return false
	}
	ref.deriveDiffColors()
	opts.deriveDiffColors()
	return PaletteFromOpts(opts) == PaletteFromOpts(ref)
}
