package dialog

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/stretchr/testify/require"
)

var ctrlR = tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}

// TestSessionItemCachesRender verifies a session row keeps its rendered
// line so repeated renders at the same width are reused.
func TestSessionItemCachesRender(t *testing.T) {
	t.Parallel()
	com := newPickerTestCommon()
	item := &SessionItem{Versioned: &list.Versioned{}, Session: session.Session{ID: "a", Title: "alpha", UpdatedAt: time.Now().Unix()}, t: com.Styles}
	first := item.Render(40)
	require.Len(t, item.cache, 1)
	require.Equal(t, first, item.Render(40))
	require.Len(t, item.cache, 1, "a repeat render at the same width reuses the line")

	item.SetFocused(true)
	require.NotEqual(t, first, item.Render(40), "focus change must invalidate the cache")
}

// TestSessionItemRefreshesRelativeTime verifies the cached row follows its
// "ago" text: a session list left open must not freeze at "now".
func TestSessionItemRefreshesRelativeTime(t *testing.T) {
	t.Parallel()
	com := newPickerTestCommon()
	item := &SessionItem{Versioned: &list.Versioned{}, Session: session.Session{ID: "a", Title: "alpha", UpdatedAt: time.Now().Unix()}, t: com.Styles}
	require.Contains(t, item.Render(60), "now")

	// The same session, a few minutes later.
	item.UpdatedAt = time.Now().Add(-3 * time.Minute).Unix()
	require.Contains(t, item.Render(60), "3 minutes ago")
}

// TestSessionsHelpKeepsEsc verifies the sessions help line uses distinct
// labels for moving and opening, and never truncates esc away.
func TestSessionsHelpKeepsEsc(t *testing.T) {
	t.Parallel()
	for _, w := range []int{100, 40} {
		d := newSessionMouseDialog(t, []session.Session{{ID: "a", Title: "alpha"}}, "a")
		text := strings.Join(drawFits(t, d, w, 30), "\n")
		if w == 100 {
			require.Contains(t, text, "↑/↓ choose")
		}
		require.Contains(t, text, "enter open")
		require.Contains(t, text, "esc exit")
	}
}

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
