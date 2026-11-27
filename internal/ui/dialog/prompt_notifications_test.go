package dialog

import (
	"image"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/styles"
	"github.com/charmbracelet/crush/internal/workspace"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

type notificationsTestWorkspace struct{ workspace.Workspace }

func (notificationsTestWorkspace) Config() *config.Config { return nil }

func drawNotifications(n *Notifications, w, h int) string {
	scr := uv.NewScreenBuffer(w, h)
	n.Draw(scr, image.Rect(0, 0, w, h))
	return strings.Join(strings.Fields(scr.String()), " ")
}

// TestNotificationsShowsHighlightedDescription verifies the picker
// explains the highlighted option, following the selection.
func TestNotificationsShowsHighlightedDescription(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	n := NewNotifications(&common.Common{Styles: &s, Workspace: notificationsTestWorkspace{}})

	for _, w := range []int{100, 40} {
		require.Contains(t, drawNotifications(n, w, 30), "Automatically detect the best")
	}
	n.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
	out := drawNotifications(n, 100, 30)
	require.NotContains(t, out, "Automatically detect")
	require.Contains(t, out, AllNotificationStyles[1].Description[:20])
}

// TestNotificationsNoDoubleBlankBeforeHelp verifies there is never more
// than one blank row between dialog content rows.
func TestNotificationsNoDoubleBlankBeforeHelp(t *testing.T) {
	t.Parallel()
	s := styles.CharmtonePantera()
	n := NewNotifications(&common.Common{Styles: &s, Workspace: notificationsTestWorkspace{}})
	scr := uv.NewScreenBuffer(100, 30)
	n.Draw(scr, image.Rect(0, 0, 100, 30))
	var rows []string
	for _, l := range strings.Split(scr.String(), "\n") {
		if strings.Contains(l, "│") {
			rows = append(rows, strings.Trim(strings.TrimSpace(l), "│ "))
		}
	}
	for i := 1; i < len(rows); i++ {
		require.Falsef(t, rows[i] == "" && rows[i-1] == "", "two blank rows in a row:\n%s", scr.String())
	}
	require.Contains(t, strings.Join(rows, "\n"), "Disabled")
}
