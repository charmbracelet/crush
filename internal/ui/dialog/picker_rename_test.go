package dialog

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

var ctrlR = tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}

// TestRenameIgnoredWithoutSelection verifies ctrl+r does nothing when the
// filter matches nothing, instead of entering rename mode and making the
// dialog vanish.
func TestRenameIgnoredWithoutSelection(t *testing.T) {
	t.Parallel()

	t.Run("sessions", func(t *testing.T) {
		t.Parallel()
		d := newSessionMouseDialog(t, []session.Session{{ID: "a", Title: "alpha"}}, "a")
		for _, r := range "zzzz" {
			d.HandleMsg(keyMsg(r))
		}
		d.HandleMsg(ctrlR)
		require.Equal(t, sessionsModeNormal, d.sessionsMode)
		drawFits(t, d, 100, 30)
	})

	t.Run("themes", func(t *testing.T) {
		t.Parallel()
		d := NewTheme(newPickerTestCommon())
		for _, r := range "zzzzqqq" {
			d.HandleMsg(keyMsg(r))
		}
		d.HandleMsg(ctrlR)
		require.Equal(t, themesModeNormal, d.mode)
		drawFits(t, d, 100, 30)
	})
}
