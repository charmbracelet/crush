package dialog

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/x/ansi"
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

// TestShortHelpLineAlwaysMarksDroppedHints verifies a truncated help line
// ends with an ellipsis even when that means dropping one more hint, so
// hints are never dropped silently.
func TestShortHelpLineAlwaysMarksDroppedHints(t *testing.T) {
	t.Parallel()
	h := help.New()
	bindings := []key.Binding{
		key.NewBinding(key.WithKeys("a"), key.WithHelp("aa", "one")),
		key.NewBinding(key.WithKeys("b"), key.WithHelp("bb", "two")),
		key.NewBinding(key.WithKeys("c"), key.WithHelp("cc", "three")),
	}
	full := ansi.Strip(shortHelpLine(&h, bindings, 100))
	require.Equal(t, "aa one • bb two • cc three", full)

	// "aa one • bb two" fills the width exactly; there is no room for
	// " …", so "bb two" must give way to it.
	got := ansi.Strip(shortHelpLine(&h, bindings, ansi.StringWidth("aa one • bb two")))
	require.Equal(t, "aa one …", got)

	// Too narrow for anything but the ellipsis.
	require.Equal(t, "…", ansi.Strip(shortHelpLine(&h, bindings, 3)))
	require.Empty(t, shortHelpLine(&h, bindings, 0))
}
