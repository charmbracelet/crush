package themes

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/stretchr/testify/require"
)

func TestAdaptFillsAllFields(t *testing.T) {
	t.Parallel()
	schemes, err := catalogSchemes()
	require.NoError(t, err)

	for _, name := range []string{"Dracula", "Nord", "Gruvbox Dark"} {
		s := findScheme(schemes, name)
		require.NotNil(t, s, name)
		p := Adapt(*s)

		require.Equal(t, s.Background, p.BgBase, name)
		require.Equal(t, s.Foreground, p.FgBase, name)
		require.Equal(t, s.ANSI[0], p.AnsiBlack, name)
		require.Equal(t, s.ANSI[15], p.AnsiBrightWhite, name)

		// Every semantic field must be populated (non-nil).
		for _, c := range []color.Color{
			p.Primary, p.Secondary, p.Accent, p.Keyword,
			p.Separator, p.FgSubtle, p.FgMoreSubtle, p.FgMostSubtle, p.OnPrimary,
			p.BgMostVisible, p.BgLessVisible, p.BgLeastVisible,
			p.Destructive, p.Error, p.Warning, p.WarningSubtle, p.Attention,
			p.Busy, p.Info, p.InfoMoreSubtle, p.InfoMostSubtle,
			p.Success, p.SuccessMoreSubtle, p.SuccessMostSubtle,
			p.AnsiBlack, p.AnsiRed, p.AnsiGreen, p.AnsiYellow, p.AnsiBlue,
			p.AnsiMagenta, p.AnsiCyan, p.AnsiWhite, p.AnsiBrightBlack,
			p.AnsiBrightRed, p.AnsiBrightGreen, p.AnsiBrightYellow,
			p.AnsiBrightBlue, p.AnsiBrightMagenta, p.AnsiBrightCyan, p.AnsiBrightWhite,
		} {
			require.NotNil(t, c, name)
		}
	}
}

func TestAdaptFallbackOnMissingSlot(t *testing.T) {
	t.Parallel()
	s := Scheme{
		Name:       "Missing",
		Background: parseHex("#000000"),
		Foreground: parseHex("#ffffff"),
	}
	s.ANSI[4] = parseHex("#0000ff")
	s.ANSI[12] = nil // bright blue missing; primary should fall back to ansi[4].

	p := Adapt(s)
	require.Equal(t, s.ANSI[4], p.Primary)
	require.NotNil(t, p.AnsiBrightBlue) // falls back to fg.
}

func TestAdaptGolden(t *testing.T) {
	schemes, err := catalogSchemes()
	require.NoError(t, err)

	var b strings.Builder
	for _, name := range []string{"Dracula", "Nord", "Gruvbox Dark"} {
		s := findScheme(schemes, name)
		require.NotNil(t, s, name)
		fmt.Fprintf(&b, "== %s ==\n%s", s.Name, paletteString(Adapt(*s)))
	}
	golden.RequireEqual(t, []byte(b.String()))
}

func paletteString(p styles.Palette) string {
	var b strings.Builder
	for _, e := range []struct {
		k string
		c color.Color
	}{
		{"primary", p.Primary}, {"secondary", p.Secondary}, {"accent", p.Accent},
		{"keyword", p.Keyword}, {"fg_base", p.FgBase}, {"bg_base", p.BgBase},
		{"separator", p.Separator}, {"fg_subtle", p.FgSubtle},
		{"fg_more_subtle", p.FgMoreSubtle}, {"fg_most_subtle", p.FgMostSubtle},
		{"on_primary", p.OnPrimary}, {"bg_most_visible", p.BgMostVisible},
		{"bg_less_visible", p.BgLessVisible}, {"bg_least_visible", p.BgLeastVisible},
		{"destructive", p.Destructive}, {"error", p.Error}, {"warning", p.Warning},
		{"warning_subtle", p.WarningSubtle}, {"attention", p.Attention},
		{"busy", p.Busy}, {"info", p.Info}, {"info_more_subtle", p.InfoMoreSubtle},
		{"info_most_subtle", p.InfoMostSubtle}, {"success", p.Success},
		{"success_more_subtle", p.SuccessMoreSubtle}, {"success_most_subtle", p.SuccessMostSubtle},
		{"ansi_black", p.AnsiBlack}, {"ansi_red", p.AnsiRed}, {"ansi_green", p.AnsiGreen},
		{"ansi_yellow", p.AnsiYellow}, {"ansi_blue", p.AnsiBlue}, {"ansi_magenta", p.AnsiMagenta},
		{"ansi_cyan", p.AnsiCyan}, {"ansi_white", p.AnsiWhite}, {"ansi_bright_black", p.AnsiBrightBlack},
		{"ansi_bright_red", p.AnsiBrightRed}, {"ansi_bright_green", p.AnsiBrightGreen},
		{"ansi_bright_yellow", p.AnsiBrightYellow}, {"ansi_bright_blue", p.AnsiBrightBlue},
		{"ansi_bright_magenta", p.AnsiBrightMagenta}, {"ansi_bright_cyan", p.AnsiBrightCyan},
		{"ansi_bright_white", p.AnsiBrightWhite},
	} {
		fmt.Fprintf(&b, "%s=%s\n", e.k, hexOf(e.c))
	}
	return b.String()
}

func hexOf(c color.Color) string {
	if c == nil {
		return "<nil>"
	}
	r, g, b, a := c.RGBA()
	if a == 0 {
		return "#00000000"
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
}
