package dialog

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

// TestPickersShowEmptyState verifies pickers show a padded message instead
// of a blank list area when there is nothing to list or nothing matches.
func TestPickersShowEmptyState(t *testing.T) {
	t.Parallel()

	t.Run("no sessions", func(t *testing.T) {
		t.Parallel()
		d := newSessionMouseDialog(t, nil, "")
		lines := drawFits(t, d, 100, 30)
		require.Contains(t, strings.Join(lines, "\n"), "│ No sessions yet.")
	})

	t.Run("sessions no match", func(t *testing.T) {
		t.Parallel()
		d := newSessionMouseDialog(t, []session.Session{{ID: "a", Title: "alpha"}}, "a")
		for _, r := range "zzzz" {
			d.HandleMsg(keyMsg(r))
		}
		lines := drawFits(t, d, 100, 30)
		require.Contains(t, strings.Join(lines, "\n"), "│ No matches.")
	})

	t.Run("channels none", func(t *testing.T) {
		t.Parallel()
		d := newTestChannels(t, nil)
		lines := drawFits(t, d, 100, 30)
		require.Contains(t, strings.Join(lines, "\n"), "│ No channels.")
	})

	t.Run("themes no match", func(t *testing.T) {
		t.Parallel()
		d := NewTheme(newPickerTestCommon())
		for _, r := range "zzzzqqq" {
			d.HandleMsg(keyMsg(r))
		}
		lines := drawFits(t, d, 100, 30)
		require.Contains(t, strings.Join(lines, "\n"), "│ No matches.")
	})
}
