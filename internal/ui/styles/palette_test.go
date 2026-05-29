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
