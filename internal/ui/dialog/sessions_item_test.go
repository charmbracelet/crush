package dialog

import (
	"testing"
	"time"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/list"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

// TestSessionItemMarksBusySession pins the switcher's busy indicator.
// The switcher is where a user goes to find out which session is
// working; before this the busy flag was dropped on the way to the
// domain type and every row looked idle.
func TestSessionItemMarksBusySession(t *testing.T) {
	t.Parallel()
	sty := styles.CharmtonePantera()
	updated := time.Now().Add(-2 * time.Minute).Unix()

	idle := &SessionItem{
		Versioned: list.NewVersioned(),
		Session:   session.Session{ID: "s1", Title: "idle work", UpdatedAt: updated},
		t:         &sty,
	}
	busy := &SessionItem{
		Versioned: list.NewVersioned(),
		Session:   session.Session{ID: "s2", Title: "live work", UpdatedAt: updated, Busy: true},
		t:         &sty,
	}

	require.NotContains(t, idle.InfoText(), "busy")
	require.Contains(t, busy.InfoText(), "busy")
	require.Contains(t, busy.InfoText(), idle.InfoText(),
		"the busy marker must not replace the timestamp")
	require.Contains(t, ansi.Strip(busy.Render(100)), "busy")
}
