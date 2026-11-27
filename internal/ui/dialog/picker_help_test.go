package dialog

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"

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

// TestArrowHelpNotation verifies pickers write arrow keys as "↑/↓" (and
// "←/→") rather than mixing "↑↓", "↑/k up" and similar forms.
func TestArrowHelpNotation(t *testing.T) {
	t.Parallel()
	com := newPickerTestCommon()
	fp, _ := NewFilePicker(com)
	keys := func(km interface{ ShortHelp() []key.Binding }) []string {
		var out []string
		for _, b := range km.ShortHelp() {
			out = append(out, b.Help().Key)
		}
		return out
	}
	items := []MCPToggleItem{{Name: "docker", Status: "connected"}}
	require.Contains(t, keys(NewMCPToggles(com, items)), "↑/↓")
	require.Contains(t, keys(NewSkillsToggles(com, []SkillToggleItem{{Name: "s"}})), "↑/↓")
	require.Contains(t, keys(fp), "↑/↓/←/→")
}
