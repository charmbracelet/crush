package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/stretchr/testify/require"
)

// recordingDialog stands in for a real dialog, recording every message routed
// to it. It only has to satisfy the three-method [dialog.Dialog] interface;
// Draw is never called in these tests.
type recordingDialog struct {
	id   string
	msgs []tea.Msg
}

func (d *recordingDialog) ID() string { return d.id }

func (d *recordingDialog) HandleMsg(msg tea.Msg) dialog.Action {
	d.msgs = append(d.msgs, msg)
	return nil
}

func (d *recordingDialog) Draw(uv.Screen, uv.Rectangle) *tea.Cursor { return nil }

var _ dialog.Dialog = (*recordingDialog)(nil)

// TestHandleSubagentsDialogMsg_RoutesByIDNotFront verifies that the subagents
// dialog receives its async results even when another dialog has opened on top
// of it. Its initial fetch, running-list refresh, and mutation rollback all
// arrive after a round trip, and anything can open in the meantime — a
// permission prompt during exactly the agent run the user opened the dialog to
// watch. Routing to the front dialog would drop those messages, and nothing
// re-requests them.
func TestHandleSubagentsDialogMsg_RoutesByIDNotFront(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.dialog = dialog.NewOverlay()

	subagentsDialog := &recordingDialog{id: dialog.SubagentsID}
	u.dialog.OpenDialog(subagentsDialog)
	u.dialog.OpenDialogWithGrace(&recordingDialog{id: dialog.PermissionsID})

	require.Equal(t, dialog.PermissionsID, u.dialog.DialogLast().ID(),
		"the permission prompt must be in front for this test to mean anything")

	msg := dialog.SubagentsInitialDataMsg{}
	require.Nil(t, u.handleSubagentsDialogMsg(msg))

	require.Len(t, subagentsDialog.msgs, 1, "the buried subagents dialog must still receive its data")
	require.IsType(t, dialog.SubagentsInitialDataMsg{}, subagentsDialog.msgs[0])
}

// TestHandleSubagentsDialogMsg_NoDialogOpen verifies that a result arriving
// after the dialog closed is dropped rather than panicking. The error half of
// a failed mutation travels as its own top-level command precisely because
// this half goes nowhere.
func TestHandleSubagentsDialogMsg_NoDialogOpen(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.dialog = dialog.NewOverlay()

	require.Nil(t, u.handleSubagentsDialogMsg(dialog.SubagentMutationFailedMsg{}))
}
