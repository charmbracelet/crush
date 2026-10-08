package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/ui/dialog"
	"github.com/stretchr/testify/require"
)

func TestProgramStatus(t *testing.T) {
	pinTTLs(t)

	ws := &countingWorkspace{ready: true}
	u := newBusyUI(ws)
	require.Equal(t, &tea.ProgramStatus{State: tea.ProgramStateIdle, App: "crush"}, u.programStatus())

	warmCaches(u, true)
	require.Equal(t, tea.ProgramStateWorking, u.programStatus().State)

	u.activeInline = dialog.NewQuestionForm(u.com.Styles, question.Request{})
	ps := u.programStatus()
	require.Equal(t, tea.ProgramStateBlocked, ps.State)
	require.Equal(t, tea.ProgramStatusKindQuestion, ps.Kind)

	u.dialog.OpenDialogWithGrace(dialog.NewPermissions(u.com, permission.PermissionRequest{ID: "p", ToolName: "bash"}))
	ps = u.programStatus()
	require.Equal(t, tea.ProgramStateBlocked, ps.State)
	require.Equal(t, tea.ProgramStatusKindPermission, ps.Kind)

	u.dialog.CloseDialog(dialog.PermissionsID)
	u.activeInline = nil

	finish := func(typ notify.Type) {
		t.Helper()
		_, cmd := u.Update(pubsub.Event[notify.Notification]{
			Type:    pubsub.CreatedEvent,
			Payload: notify.Notification{Type: typ, SessionID: "s1"},
		})
		runCmds(u, cmd)
	}

	finish(notify.TypeAgentError)
	require.Equal(t, tea.ProgramStateError, u.programStatus().State)

	finish(notify.TypeAgentFinished)
	require.Equal(t, tea.ProgramStateDone, u.programStatus().State)

	u.Update(tea.FocusMsg{})
	require.Equal(t, tea.ProgramStateIdle, u.programStatus().State)
}
