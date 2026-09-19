package dialog

import (
	"context"
	"testing"

	"charm.land/bubbles/v2/key"

	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

type keybindTestWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *keybindTestWorkspace) ListSessions(context.Context) ([]session.Session, error) {
	return nil, nil
}

func (w *keybindTestWorkspace) Config() *config.Config {
	return w.cfg
}

// Overrides reach dialog keymaps at construction.
func TestApplyDialogKeybinds_SessionsSelect(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	com := &common.Common{
		Workspace: &keybindTestWorkspace{cfg: &config.Config{
			Keybinds: map[string][]string{"dialog.sessions.delete": {"ctrl+d"}},
		}},
		Styles: &s,
	}
	dlg, err := NewSessions(com, "")
	require.NoError(t, err)
	require.Equal(t, []string{"ctrl+d"}, dlg.keyMap.Delete.Keys())
	require.Equal(t, "ctrl+d", dlg.keyMap.Delete.Help().Key)
}

// Empty disables without touching siblings.
func TestApplyDialogKeybinds_EmptyDisables(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	com := &common.Common{
		Workspace: &keybindTestWorkspace{cfg: &config.Config{
			Keybinds: map[string][]string{"dialog.sessions.delete": {}},
		}},
		Styles: &s,
	}
	dlg, err := NewSessions(com, "")
	require.NoError(t, err)
	require.False(t, dlg.keyMap.Delete.Enabled())
	require.True(t, dlg.keyMap.Rename.Enabled())
}

// Nav help stays one compact row while the keys are the arrows, and
// falls back to the two labeled entries once they are not.
func TestNavHelp_CompactAndRemapped(t *testing.T) {
	t.Parallel()
	next := key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "next item"))
	previous := key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "previous item"))

	compact := navHelp(next, previous)
	require.Len(t, compact, 1)
	require.Equal(t, "↑/↓", compact[0].Help().Key)
	require.Equal(t, "choose", compact[0].Help().Desc)

	next.SetKeys("j")
	previous.SetKeys("k")
	expanded := navHelp(next, previous)
	require.Equal(t, []key.Binding{previous, next}, expanded)
}
