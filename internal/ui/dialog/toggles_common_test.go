package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

// TestTogglesEmptyState verifies the toggles dialogs' empty message is
// spaced from the title and help, and the help lists only esc.
func TestTogglesEmptyState(t *testing.T) {
	t.Parallel()
	com := newPickerTestCommon()
	for name, d := range map[string]interface {
		Draw(uv.Screen, uv.Rectangle) *tea.Cursor
	}{
		"mcp":    NewMCPToggles(com, nil),
		"skills": NewSkillsToggles(com, nil),
	} {
		lines := drawFits(t, d, 100, 30)
		var rows []string
		for _, l := range lines {
			if l = strings.TrimSpace(l); l != "" {
				rows = append(rows, strings.Trim(l, "│ "))
			}
		}
		// Border, title, blank, message, blank, help, border.
		require.Lenf(t, rows, 7, "%s:\n%s", name, strings.Join(lines, "\n"))
		require.Empty(t, rows[2], name)
		require.Contains(t, rows[3], "configured", name)
		require.Empty(t, rows[4], name)
		require.Equal(t, "esc exit", rows[5], name)
	}
}
