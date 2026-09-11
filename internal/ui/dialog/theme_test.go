package dialog

import (
	"testing"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// themeTestWorkspace is a minimal [workspace.Workspace] stub.
type themeTestWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *themeTestWorkspace) Config() *config.Config { return w.cfg }

// TestThemeDialogListsAllThemes — the theme menu shows every selectable
// theme and marks the current one.
func TestThemeDialogListsAllThemes(t *testing.T) {
	s := styles.CharmtonePantera()
	com := &common.Common{
		Styles:    &s,
		Workspace: &themeTestWorkspace{cfg: &config.Config{}},
	}
	d, err := NewTheme(com, "dyad-mapper")
	require.NoError(t, err)

	items := d.list.FilteredItems()
	require.Len(t, items, len(styles.ThemeNames()), "every theme is listed")

	var names []string
	for _, it := range items {
		ti, ok := it.(*ThemeItem)
		require.True(t, ok)
		names = append(names, ti.name)
		if ti.name == "dyad-mapper" {
			require.True(t, ti.isCurrent, "the active theme is marked current")
		}
	}
	require.Contains(t, names, "dyad-mapper")
	require.Contains(t, names, "charmtone")
}

// TestDyadMapperIsARealTheme — the sensitive-eyes theme builds a complete
// Styles object; sanity-check it carries the soft navy base.
func TestDyadMapperIsARealTheme(t *testing.T) {
	s := styles.DyadMapper()
	require.NotNil(t, s)
	require.NotEmpty(t, styles.ThemeNames())
	// selecting it by name returns it
	got := styles.ThemeForName("dyad-mapper")
	require.Equal(t, s, got)
}
