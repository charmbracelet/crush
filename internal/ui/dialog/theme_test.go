package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/stretchr/testify/require"
)

func newThemePicker(t *testing.T) *ThemePicker {
	t.Helper()
	s := styles.CharmtonePantera()
	return NewThemePicker(&common.Common{Styles: &s})
}

func TestThemePickerListsThemes(t *testing.T) {
	t.Parallel()
	d := newThemePicker(t)

	require.NotEmpty(t, d.list.FilteredItems())

	first, ok := d.list.SelectedItem().(*ThemeItem)
	require.True(t, ok)
	require.Equal(t, "default", first.scheme.Name)
}

func TestThemePickerNavigationReturnsPreviewAction(t *testing.T) {
	t.Parallel()
	d := newThemePicker(t)

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	preview, ok := action.(ActionPreviewTheme)
	require.True(t, ok)
	require.Equal(t, "hyper", preview.Name)
}

func TestThemePickerSelectReturnsAction(t *testing.T) {
	t.Parallel()
	d := newThemePicker(t)

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	sel, ok := action.(ActionSelectTheme)
	require.True(t, ok)
	require.Equal(t, "default", sel.Name)
}
