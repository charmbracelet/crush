package dialog

import (
	"context"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	"github.com/stretchr/testify/require"
)

// sessionsWorkspace is a minimal [workspace.Workspace] stub that only serves
// a fixed session list.
type sessionsWorkspace struct {
	workspace.Workspace
	sessions []session.Session
}

func (w *sessionsWorkspace) ListSessions(context.Context) ([]session.Session, error) {
	return w.sessions, nil
}

func newTestSessions(t *testing.T, count int) *Session {
	t.Helper()

	sessions := make([]session.Session, count)
	for i := range sessions {
		sessions[i] = session.Session{
			ID:    fmt.Sprintf("session-%d", i),
			Title: fmt.Sprintf("Session %d", i),
		}
	}

	s := styles.CharmtonePantera()
	com := &common.Common{
		Styles:    &s,
		Workspace: &sessionsWorkspace{sessions: sessions},
	}
	dialog, err := NewSessions(com, "")
	require.NoError(t, err)
	return dialog
}

// TestSessions_ModifierKeysKeepSelection verifies that a bare modifier key
// press does not move the selection. Terminals that report modifier presses
// on their own used to reset the cursor to the first session, so ctrl+r
// renamed the wrong one.
func TestSessions_ModifierKeysKeepSelection(t *testing.T) {
	t.Parallel()

	modifiers := []tea.KeyPressMsg{
		{Code: tea.KeyLeftCtrl},
		{Code: tea.KeyRightCtrl},
		{Code: tea.KeyLeftShift},
		{Code: tea.KeyRightShift},
		{Code: tea.KeyLeftAlt},
		{Code: tea.KeyRightAlt},
	}

	for _, msg := range modifiers {
		t.Run(msg.String(), func(t *testing.T) {
			t.Parallel()

			s := newTestSessions(t, 5)
			s.list.SetSelected(2)

			s.HandleMsg(msg)
			require.Equal(t, 2, s.list.Selected())
		})
	}
}

// TestSessions_TypingFiltersAndResetsSelection verifies that keys which do
// change the filter still reset the selection to the first match.
func TestSessions_TypingFiltersAndResetsSelection(t *testing.T) {
	t.Parallel()

	s := newTestSessions(t, 5)
	s.list.SetSelected(2)

	s.HandleMsg(keyMsg('3'))
	require.Equal(t, "3", s.input.Value())
	require.Equal(t, 0, s.list.Selected())
}
