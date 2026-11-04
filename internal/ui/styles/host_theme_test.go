package styles

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"charm.land/glamour/v2/ansi"
	"github.com/alecthomas/chroma/v2"
	"github.com/charmbracelet/x/exp/charmtone"
	"github.com/stretchr/testify/require"
)

// defaultPaletteMap returns charmtone-panther's resolved palette the way a
// host would send it: theme-file keys mapped to "#rrggbb".
func defaultPaletteMap(t *testing.T) map[string]string {
	t.Helper()
	p, err := ThemePalette("charmtone-panther")
	require.NoError(t, err)
	m := map[string]string{}
	for _, f := range PaletteFields() {
		if v := f.Get(p); v != "" {
			m[f.Name] = ColorString(v)
		}
	}
	return m
}

func TestHostThemeStyles_EmptyThemeIsCharmtone(t *testing.T) {
	t.Parallel()

	s, err := HostThemeStyles("", nil, nil)
	require.NoError(t, err)
	builtin := CharmtonePantera()
	require.Equal(t, "#201f26", *hex(s.Background))
	require.Equal(t, *hex(builtin.WorkingGradFromColor), *hex(s.WorkingGradFromColor))
	// The pristine built-in keeps its hardcoded overrides.
	require.Equal(t, *hex(charmtone.Bengal), *s.Markdown.CodeBlock.Chroma.CommentPreproc.Color)
	require.Equal(t, builtin.ChromaTheme(), s.ChromaTheme())
}

func TestHostThemeStyles_DefaultPaletteKeepsBuiltinLook(t *testing.T) {
	t.Parallel()

	// A host that sends Crush's own default colors gets exactly the
	// default look, overrides included.
	s, err := HostThemeStyles("", defaultPaletteMap(t), nil)
	require.NoError(t, err)
	builtin := CharmtonePantera()
	require.Equal(t, builtin.ChromaTheme(), s.ChromaTheme())
	require.Equal(t, builtin.Markdown.Link.Color, s.Markdown.Link.Color)
	require.Equal(t, *hex(builtin.Background), *hex(s.Background))
}

func TestHostThemeStyles_Palette(t *testing.T) {
	t.Parallel()

	palette := map[string]string{
		"primary":    "#112233",
		"on_primary": "Butter",
		"bg_base":    "#fff",
		"fg_base":    "#000000",
		"ansi_red":   "1",
	}
	s, err := HostThemeStyles("", palette, nil)
	require.NoError(t, err)

	require.Equal(t, "#112233", *hex(s.WorkingGradFromColor))
	require.Equal(t, "#112233", *hex(s.TextSelection.GetBackground()))
	require.Equal(t, *hex(charmtone.Butter), *hex(s.TextSelection.GetForeground()))
	require.Equal(t, "#ffffff", *hex(s.Background))
	// Colors left out come from the base.
	require.Equal(t, *hex(charmtone.Dolly), *hex(s.WorkingGradToColor))

	// Diff backgrounds are re-derived from the host's bg_base.
	builtin := CharmtonePantera()
	require.NotEqual(t, *hex(builtin.Diff.InsertLine.Code.GetBackground()), *hex(s.Diff.InsertLine.Code.GetBackground()))

	// A modified palette is token driven: no hardcoded built-in overrides.
	require.NotEqual(t, *hex(charmtone.Bengal), *s.Markdown.CodeBlock.Chroma.CommentPreproc.Color)
}

func TestHostThemeStyles_Syntax(t *testing.T) {
	t.Parallel()

	syntax := map[string]string{
		"keyword":        "#123456",
		"comment":        "Charple",
		"name_function":  "#abc",
		"literal_number": "214",
		"name_class":     "#654321",
	}
	s, err := HostThemeStyles("", nil, syntax)
	require.NoError(t, err)

	c := s.Markdown.CodeBlock.Chroma
	require.Equal(t, "#123456", *c.Keyword.Color)
	require.Equal(t, *hex(charmtone.Charple), *c.Comment.Color)
	require.Equal(t, "#aabbcc", *c.NameFunction.Color)
	require.Equal(t, "#ffaf00", *c.LiteralNumber.Color) // xterm 214.
	// Only the color changes; the token's attributes stay.
	require.Equal(t, "#654321", *c.NameClass.Color)
	require.True(t, *c.NameClass.Bold)

	// Every chroma user derives from this config: the plan card's code
	// blocks and the style that file views and diffs highlight with.
	require.Equal(t, "#123456", *s.PlanMarkdown.CodeBlock.Chroma.Keyword.Color)
	require.Equal(t, "#123456", s.ChromaTheme()[chroma.Keyword])
	style := chroma.MustNewStyle("test", s.ChromaTheme())
	require.Equal(t, "#123456", style.Get(chroma.Keyword).Colour.String())

	// Tokens the host leaves out keep the base's colors, built-in
	// overrides included, since the palette is untouched.
	require.Equal(t, *hex(charmtone.Bengal), *c.CommentPreproc.Color)
}

func TestHostThemeStyles_RejectsInvalidColors(t *testing.T) {
	t.Parallel()

	for _, v := range []string{"purple-ish", "#12", "#12345", "#12345g", "256", "-1", " #ffffff", "#6b50ffff", "#fffa"} {
		_, err := HostThemeStyles("", map[string]string{"primary": v}, nil)
		require.ErrorContains(t, err, "palette.primary", "value %q", v)
		_, err = HostThemeStyles("", nil, map[string]string{"keyword": v})
		require.ErrorContains(t, err, "syntax.keyword", "value %q", v)
		_, err = ValidateHostTheme("", map[string]string{"primary": v}, nil)
		require.Error(t, err, "value %q", v)
	}

	// Every offending color is named at once.
	_, err := ValidateHostTheme("", map[string]string{"primary": "x", "bg_base": "y"}, map[string]string{"comment": "z"})
	require.ErrorContains(t, err, "palette.bg_base")
	require.ErrorContains(t, err, "palette.primary")
	require.ErrorContains(t, err, "syntax.comment")

	// Theme files keep accepting alpha hex; only host themes refuse it.
	require.True(t, IsValidColor("#6b50ffff"))
	require.NoError(t, Palette{Primary: "#fffa"}.Validate())
}

func TestHostThemeStyles_IgnoresUnknownKeysAndEmptyValues(t *testing.T) {
	t.Parallel()

	palette := map[string]string{"primary": "#112233", "hologram": "not a color", "secondary": ""}
	syntax := map[string]string{"keyword": "#123456", "error": "#ff0000", "background": "#000000", "quantum": "x"}

	ignored, err := ValidateHostTheme("", palette, syntax)
	require.NoError(t, err)
	require.Equal(t, []string{"palette.hologram", "syntax.background", "syntax.error", "syntax.quantum"}, ignored)

	s, err := HostThemeStyles("", palette, syntax)
	require.NoError(t, err)
	known, err := HostThemeStyles("", map[string]string{"primary": "#112233"}, map[string]string{"keyword": "#123456"})
	require.NoError(t, err)
	require.Equal(t, known.ChromaTheme(), s.ChromaTheme())
	require.Equal(t, *hex(known.WorkingGradToColor), *hex(s.WorkingGradToColor))
	require.Equal(t, *hex(known.Background), *hex(s.Background))
}

func TestHostThemeStyles_Base(t *testing.T) {
	dir := t.TempDir()
	setTestThemeDirs(t, []string{dir})

	s, err := HostThemeStyles("gruvbox-dark", nil, nil)
	require.NoError(t, err)
	require.Equal(t, "#282828", *hex(s.Background))

	// A user theme works as a base, and the host's colors win over it.
	require.NoError(t, SaveThemeFile(filepath.Join(dir, "mine.json"), &ThemeFile{
		Base:    "keanu",
		Palette: Palette{Primary: "#ff0000", BgBase: "#0000ff"},
	}))
	s, err = HostThemeStyles("mine", map[string]string{"bg_base": "#010101"}, nil)
	require.NoError(t, err)
	require.Equal(t, "#ff0000", *hex(s.WorkingGradFromColor))
	require.Equal(t, "#010101", *hex(s.Background))
	_, err = ValidateHostTheme("mine", nil, nil)
	require.NoError(t, err)

	_, err = HostThemeStyles("no-such-theme", nil, nil)
	require.ErrorContains(t, err, "base theme")
	_, err = ValidateHostTheme("no-such-theme", nil, nil)
	require.ErrorContains(t, err, "base theme")
}

// TestSyntaxFields_MatchChroma checks that the syntax keys are exactly the
// json names of glamour's foreground-only chroma tokens, in order, and that
// each one reaches its own field.
func TestSyntaxFields_MatchChroma(t *testing.T) {
	t.Parallel()

	var want []string
	typ := reflect.TypeFor[ansi.Chroma]()
	for i := range typ.NumField() {
		name, _, _ := strings.Cut(typ.Field(i).Tag.Get("json"), ",")
		if name == "error" || name == "background" {
			continue
		}
		want = append(want, name)
	}

	var got []string
	var c ansi.Chroma
	seen := map[*ansi.StylePrimitive]bool{}
	for _, f := range SyntaxFields() {
		got = append(got, f.Name)
		p := f.Style(&c)
		require.False(t, seen[p], "%s shares a field", f.Name)
		seen[p] = true
	}
	require.Equal(t, want, got)
}
