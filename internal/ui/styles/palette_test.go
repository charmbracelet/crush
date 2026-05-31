package styles

import (
	"encoding/json"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/require"
)

func TestPaletteFromOpts_RoundTrip(t *testing.T) {
	t.Parallel()
	opts := charmtoneOpts()
	p := PaletteFromOpts(opts)

	require.NotEmpty(t, p.Primary)
	require.NotEmpty(t, p.BgBase)
	require.NotEmpty(t, p.FgBase)
	require.NotEmpty(t, p.Success)

	recovered := p.ToQuickStyleOpts(quickStyleOpts{})
	require.Equal(t, colorToHex(opts.primary), colorToHex(recovered.primary))
	require.Equal(t, colorToHex(opts.secondary), colorToHex(recovered.secondary))
	require.Equal(t, colorToHex(opts.bgBase), colorToHex(recovered.bgBase))
	require.Equal(t, colorToHex(opts.fgBase), colorToHex(recovered.fgBase))
}

func TestPaletteFromOpts_GruvboxDark(t *testing.T) {
	t.Parallel()
	opts := gruvboxDarkOpts()
	p := PaletteFromOpts(opts)

	require.Equal(t, "#fabd2f", p.Primary)
	require.Equal(t, "#d3869b", p.Secondary)
	require.Equal(t, "#282828", p.BgBase)
	require.Equal(t, "#ebdbb2", p.FgBase)
}

func TestPalette_ToQuickStyleOpts_PartialOverride(t *testing.T) {
	t.Parallel()
	base := charmtoneOpts()
	override := Palette{
		Primary: "#FF0000",
		BgBase:  "#000000",
	}

	result := override.ToQuickStyleOpts(base)

	require.Equal(t, lipgloss.Color("#FF0000"), result.primary)
	require.Equal(t, lipgloss.Color("#000000"), result.bgBase)
	require.Equal(t, base.secondary, result.secondary)
	require.Equal(t, base.fgBase, result.fgBase)
	require.Equal(t, base.success, result.success)
}

func TestPalette_ToQuickStyleOpts_EmptyFallsBackToBase(t *testing.T) {
	t.Parallel()
	base := charmtoneOpts()
	empty := Palette{}

	result := empty.ToQuickStyleOpts(base)

	require.Equal(t, base.primary, result.primary)
	require.Equal(t, base.bgBase, result.bgBase)
	require.Equal(t, base.fgBase, result.fgBase)
}

func TestPalette_Validate_Valid(t *testing.T) {
	t.Parallel()
	p := Palette{
		Primary: "#FF0000",
		BgBase:  "#00FF00",
	}
	require.NoError(t, p.Validate())
}

func TestPalette_Validate_EmptyIsValid(t *testing.T) {
	t.Parallel()
	p := Palette{}
	require.NoError(t, p.Validate())
}

func TestPalette_Validate_InvalidColor(t *testing.T) {
	t.Parallel()
	p := Palette{
		Primary: "not-a-color",
	}
	err := p.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "primary")
}

func TestPalette_JSON_Serialization(t *testing.T) {
	t.Parallel()
	p := Palette{
		Primary:   "#FF0000",
		Secondary: "#00FF00",
		BgBase:    "#1A1A2E",
	}

	data, err := json.Marshal(p)
	require.NoError(t, err)

	var decoded Palette
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.Equal(t, p.Primary, decoded.Primary)
	require.Equal(t, p.Secondary, decoded.Secondary)
	require.Equal(t, p.BgBase, decoded.BgBase)
	require.Empty(t, decoded.Accent)
}

func TestPalette_JSON_OmitsEmpty(t *testing.T) {
	t.Parallel()
	p := Palette{Primary: "#FF0000"}

	data, err := json.Marshal(p)
	require.NoError(t, err)

	var m map[string]any
	err = json.Unmarshal(data, &m)
	require.NoError(t, err)

	require.Contains(t, m, "primary")
	require.NotContains(t, m, "secondary")
	require.NotContains(t, m, "bg_base")
}

func TestThemePalette_Builtin(t *testing.T) {
	t.Parallel()
	for _, name := range BuiltinThemeNames() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p, err := ThemePalette(name)
			require.NoError(t, err)
			require.NotEmpty(t, p.Primary)
			require.NotEmpty(t, p.BgBase)
			require.NoError(t, p.Validate())
		})
	}
}

func TestThemePalette_Unknown(t *testing.T) {
	t.Parallel()
	_, err := ThemePalette("nonexistent")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown theme")
}

func TestThemePalette_CaseInsensitive(t *testing.T) {
	t.Parallel()
	p, err := ThemePalette("Gruvbox-Dark")
	require.NoError(t, err)
	require.Equal(t, "#fabd2f", p.Primary)
}

func TestLoadPaletteTheme_PartialOverride(t *testing.T) {
	t.Parallel()
	s, err := LoadPaletteTheme("gruvbox-dark", Palette{Primary: "#ff0000"})
	require.NoError(t, err)
	require.Equal(t, lipgloss.Color("#ff0000"), s.WorkingGradFromColor)
}

func TestLoadPaletteTheme_UnknownBase(t *testing.T) {
	t.Parallel()
	_, err := LoadPaletteTheme("nonexistent", Palette{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown theme")
}

func TestLoadPaletteTheme_InvalidPalette(t *testing.T) {
	t.Parallel()
	_, err := LoadPaletteTheme("charmtone", Palette{Primary: "not-a-color"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "primary")
}

func TestColorToHex_Nil(t *testing.T) {
	t.Parallel()
	require.Empty(t, colorToHex(nil))
}

func TestResolveColor_EmptyReturnsFallback(t *testing.T) {
	t.Parallel()
	fallback := lipgloss.Color("#AABBCC")
	result := resolveColor("", fallback)
	require.Equal(t, fallback, result)
}

func TestResolveColor_NonEmptyParsesHex(t *testing.T) {
	t.Parallel()
	fallback := lipgloss.Color("#AABBCC")
	result := resolveColor("#FF0000", fallback)
	require.Equal(t, lipgloss.Color("#FF0000"), result)
	require.NotEqual(t, fallback, result)
}

func TestIsValidColor_ShortHex(t *testing.T) {
	t.Parallel()
	require.True(t, IsValidColor("#f00"))
	require.True(t, IsValidColor("#FFF"))
	require.True(t, IsValidColor("#abc"))
	require.False(t, IsValidColor("#gg0"))
	require.False(t, IsValidColor("#12"))
}

func TestIsValidColor_FullHex(t *testing.T) {
	t.Parallel()
	require.True(t, IsValidColor("#ff0000"))
	require.True(t, IsValidColor("#FF0000"))
	require.True(t, IsValidColor("#aabbcc"))
	require.False(t, IsValidColor("#gggggg"))
	require.False(t, IsValidColor("#12345"))
}

func TestIsValidColor_ANSI16(t *testing.T) {
	t.Parallel()
	require.True(t, IsValidColor("0"))
	require.True(t, IsValidColor("7"))
	require.True(t, IsValidColor("15"))
}

func TestIsValidColor_ANSI256(t *testing.T) {
	t.Parallel()
	require.True(t, IsValidColor("16"))
	require.True(t, IsValidColor("100"))
	require.True(t, IsValidColor("255"))
	require.False(t, IsValidColor("256"))
	require.False(t, IsValidColor("-1"))
}

func TestIsValidColor_CharmtoneNames(t *testing.T) {
	t.Parallel()
	require.True(t, IsValidColor("Charple"))
	require.True(t, IsValidColor("charple"))
	require.True(t, IsValidColor("CHARPLE"))
	require.True(t, IsValidColor("Dolly"))
	require.True(t, IsValidColor("Pepper"))
	require.True(t, IsValidColor("BBQ"))
	require.True(t, IsValidColor("Ash"))      // alias for Sash
	require.True(t, IsValidColor("Charcoal")) // alias for Char
	require.False(t, IsValidColor("Notacolor"))
}

func TestIsValidColor_Invalid(t *testing.T) {
	t.Parallel()
	require.False(t, IsValidColor(""))
	require.False(t, IsValidColor("not-a-color"))
	require.False(t, IsValidColor("red"))
	require.False(t, IsValidColor("#"))
	require.False(t, IsValidColor("256"))
}

func TestParseColor_Hex(t *testing.T) {
	t.Parallel()
	require.Equal(t, "#f00", ParseColor("#f00"))
	require.Equal(t, "#ff0000", ParseColor("#ff0000"))
	require.Equal(t, "#FF0000", ParseColor("#FF0000"))
	require.Empty(t, ParseColor("#xyz"))
}

func TestParseColor_HexNormalizesToCharmtone(t *testing.T) {
	t.Parallel()
	require.Equal(t, "Charple", ParseColor("#6b50ff"))
	require.Equal(t, "Charple", ParseColor("#6B50FF"))
	require.Equal(t, "Pepper", ParseColor("#201f26"))
	require.Equal(t, "Dolly", ParseColor("#ff60ff"))
}

func TestParseColor_ANSI(t *testing.T) {
	t.Parallel()
	require.Equal(t, "0", ParseColor("0"))
	require.Equal(t, "15", ParseColor("15"))
	require.Equal(t, "200", ParseColor("200"))
	require.Equal(t, "255", ParseColor("255"))
	require.Empty(t, ParseColor("256"))
}

func TestParseColor_CharmtoneNames(t *testing.T) {
	t.Parallel()
	require.Equal(t, "Charple", ParseColor("Charple"))

	// Case-insensitive.
	require.Equal(t, ParseColor("charple"), ParseColor("Charple"))
	require.Equal(t, ParseColor("CHARPLE"), ParseColor("Charple"))

	// Aliases resolve to canonical name.
	require.Equal(t, "Sash", ParseColor("Ash"))
	require.Equal(t, "Char", ParseColor("Charcoal"))
}

func TestParseColor_Invalid(t *testing.T) {
	t.Parallel()
	require.Empty(t, ParseColor(""))
	require.Empty(t, ParseColor("not-a-color"))
	require.Empty(t, ParseColor("#xyz"))
	require.Empty(t, ParseColor("999"))
}

func TestPalette_Validate_ANSIColors(t *testing.T) {
	t.Parallel()
	p := Palette{
		Primary: "200",
		BgBase:  "0",
	}
	require.NoError(t, p.Validate())
}

func TestPalette_Validate_CharmtoneNames(t *testing.T) {
	t.Parallel()
	p := Palette{
		Primary: "Charple",
		BgBase:  "Pepper",
		Accent:  "Dolly",
	}
	require.NoError(t, p.Validate())
}

func TestPalette_Validate_MixedFormats(t *testing.T) {
	t.Parallel()
	p := Palette{
		Primary:   "#ff0000",
		Secondary: "Charple",
		Accent:    "100",
		Keyword:   "#f00",
	}
	require.NoError(t, p.Validate())
}

func TestResolveColor_CharmtoneName(t *testing.T) {
	t.Parallel()
	fallback := lipgloss.Color("#000000")
	result := resolveColor("Charple", fallback)
	require.NotEqual(t, fallback, result)
}

func TestResolveColor_ANSI(t *testing.T) {
	t.Parallel()
	fallback := lipgloss.Color("#000000")
	result := resolveColor("200", fallback)
	require.NotEqual(t, fallback, result)
}
