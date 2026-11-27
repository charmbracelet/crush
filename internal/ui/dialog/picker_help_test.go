package dialog

import (
	"strings"
	"testing"

	"github.com/charmbracelet/crush/internal/session"
	"github.com/stretchr/testify/require"
)

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
