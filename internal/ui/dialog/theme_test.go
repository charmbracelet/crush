package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/ui/list"
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
		&ThemeItem{Versioned: &list.Versioned{}, name: "charmtone"},
	}))
}

// TestThemeRefreshStylesReachesFilteredOutRows verifies a theme preview
// recolors every row, so clearing the filter afterwards does not reveal
// rows still cached in the old theme's colors.
func TestThemeRefreshStylesReachesFilteredOutRows(t *testing.T) {
	t.Parallel()
	th := NewTheme(newPickerTestCommon())
	alpha := &ThemeItem{Versioned: &list.Versioned{}, name: "alpha", label: "alpha", t: th.com.Styles}
	beta := &ThemeItem{Versioned: &list.Versioned{}, name: "beta", label: "beta", t: th.com.Styles}
	th.list.SetItems(alpha, beta)
	alpha.Render(40)
	beta.Render(40)
	require.NotNil(t, beta.cache)

	th.list.SetFilter("alpha")
	th.RefreshStyles()
	require.Nil(t, alpha.cache)
	require.Nil(t, beta.cache, "a row hidden by the filter must drop its cached colors too")
}
