package dialog

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestHasThemeItem(t *testing.T) {
	t.Parallel()
	require.False(t, hasThemeItem(nil))
	require.False(t, hasThemeItem([]list.Item{
		&ThemeSectionHeader{Versioned: &list.Versioned{}},
		&themeSpacer{Versioned: &list.Versioned{}},
	}))
	require.True(t, hasThemeItem([]list.Item{
		&ThemeItem{pickerRow: newPickerRow(), name: "charmtone"},
	}))
}

// TestThemeRefreshStylesReachesFilteredOutRows verifies a theme preview
// recolors every row, so clearing the filter afterwards does not reveal
// rows still cached in the old theme's colors.
func TestThemeRefreshStylesReachesFilteredOutRows(t *testing.T) {
	t.Parallel()
	th := NewTheme(newPickerTestCommon())
	alpha := &ThemeItem{pickerRow: newPickerRow(), name: "alpha", label: "alpha", t: th.com.Styles}
	beta := &ThemeItem{pickerRow: newPickerRow(), name: "beta", label: "beta", t: th.com.Styles}
	th.list.SetItems(alpha, beta)
	alpha.Render(40)
	beta.Render(40)
	require.True(t, beta.cache.ok)

	th.list.SetFilter("alpha")
	th.RefreshStyles()
	require.False(t, alpha.cache.ok)
	require.False(t, beta.cache.ok, "a row hidden by the filter must drop its cached colors too")
}

// TestSectionHeadersKeepRightPadding verifies picker section headers end
// with the same one-column pad as item rows instead of touching the border.
func TestSectionHeadersKeepRightPadding(t *testing.T) {
	t.Parallel()
	com := newPickerTestCommon()
	theme := (&ThemeSectionHeader{Versioned: &list.Versioned{}, title: "System", t: com.Styles}).Render(40)
	group := (&ModelGroup{Title: "OpenAI", t: com.Styles}).Render(40)
	for _, s := range []string{theme, group} {
		plain := ansi.Strip(s)
		require.Equal(t, 39, ansi.StringWidth(plain), plain)
		require.False(t, strings.HasSuffix(plain, " "), plain)
	}
}

// TestThemeListNewThemeUsesEllipsis verifies the new theme entry uses a
// real ellipsis like other labels.
func TestThemeListNewThemeUsesEllipsis(t *testing.T) {
	t.Parallel()
	text := strings.Join(drawFits(t, NewTheme(newPickerTestCommon()), 100, 30), "\n")
	require.Contains(t, text, "New Theme…")
}
