package styles

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTerminalPaletteIsUsable(t *testing.T) {
	t.Parallel()

	var palette TerminalPalette
	require.False(t, palette.IsUsable())

	palette.ANSI[0] = color.NRGBA{R: 0, G: 0, B: 0, A: 0xff}
	require.False(t, palette.IsUsable())

	palette.ANSI[7] = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	require.True(t, palette.IsUsable())
}

func TestSystemThemeStylesUsesTerminalPalette(t *testing.T) {
	t.Parallel()

	var ansiColors [16]color.Color
	for index := range ansiColors {
		ansiColors[index] = color.NRGBA{R: uint8(index * 12), G: uint8(index * 9), B: uint8(index * 7), A: 0xff}
	}
	ansiColors[5] = color.NRGBA{R: 0xcc, G: 0x00, B: 0xcc, A: 0xff}
	ansiColors[6] = color.NRGBA{R: 0x00, G: 0xcc, B: 0xcc, A: 0xff}

	palette := TerminalPalette{
		Foreground: color.NRGBA{R: 0xf0, G: 0xf0, B: 0xf0, A: 0xff},
		Background: color.NRGBA{R: 0x10, G: 0x10, B: 0x10, A: 0xff},
		ANSI:       ansiColors,
	}
	styles := SystemThemeStyles(palette)

	require.Equal(t, "#101010", *hex(styles.Background))
	require.Equal(t, "#00cccc", *hex(styles.WorkingGradFromColor))
	require.Equal(t, "#cc00cc", *hex(styles.WorkingGradToColor))
	require.Equal(t, "#00cccc", *hex(styles.ANSI[6]))
	require.GreaterOrEqual(t, wcagContrast(styles.ANSI[0], styles.Background), 3.0)
	require.Equal(t, "#f0f0f0", *styles.Markdown.Document.Color)
	require.Equal(t, "#cc00cc", *styles.Markdown.CodeBlock.Chroma.Keyword.Color)
	require.GreaterOrEqual(t, contrastRatio(*styles.Markdown.Document.Color, *hex(styles.Background)), 4.5)
}

func TestSystemThemeStylesKeepsLightForegroundReadable(t *testing.T) {
	t.Parallel()

	styles := SystemThemeStyles(TerminalPalette{
		Foreground: color.NRGBA{R: 0x20, G: 0x20, B: 0x20, A: 0xff},
		Background: color.NRGBA{R: 0xf8, G: 0xf8, B: 0xf8, A: 0xff},
	})

	require.Equal(t, "#f8f8f8", *hex(styles.Background))
	require.GreaterOrEqual(t, contrastRatio(*styles.Markdown.Document.Color, *hex(styles.Background)), 4.5)
	require.GreaterOrEqual(t, contrastRatio(*styles.Markdown.Code.Color, *styles.Markdown.Code.BackgroundColor), 4.5)
}

func TestSystemThemeStylesFallsBackWithoutUsablePalette(t *testing.T) {
	t.Parallel()

	got := SystemThemeStyles(TerminalPalette{})
	want := CharmtonePantera()
	require.Equal(t, *hex(want.Background), *hex(got.Background))
	require.Equal(t, *hex(want.WorkingGradFromColor), *hex(got.WorkingGradFromColor))
}

func TestSystemThemeIsRegisteredAsBuiltin(t *testing.T) {
	t.Parallel()

	require.True(t, IsBuiltinTheme(SystemThemeName))
	require.Contains(t, BuiltinThemeNames(), SystemThemeName)
	got, err := LoadTheme(SystemThemeName)
	require.NoError(t, err)
	require.Equal(t, *hex(CharmtonePantera().Background), *hex(got.Background))
}
