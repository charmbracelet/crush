package model

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
	"github.com/charmbracelet/crush/internal/ui/common"
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
			Payload: notify.Notification{Type: typ, SessionID: "s1", FinishState: notify.FinishIdleSuccess},
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

func TestProgramStatusCompletionLifecycle(t *testing.T) {
	pinTTLs(t)
	for _, tc := range []struct {
		name    string
		state   notify.FinishState
		busy    bool
		pending int
		prior   tea.ProgramState
		want    tea.ProgramState
	}{
		{"success idle", notify.FinishIdleSuccess, false, 0, "", tea.ProgramStateDone},
		{"queued work", notify.FinishContinuing, true, 0, "", tea.ProgramStateWorking},
		{"handoff admission gap", notify.FinishContinuing, false, 0, "", tea.ProgramStateIdle},
		{"new work", notify.FinishIdleSuccess, true, 0, "", tea.ProgramStateWorking},
		{"pending submission", notify.FinishIdleSuccess, false, 1, "", tea.ProgramStateWorking},
		{"cancelled", notify.FinishIdleUnsuccessful, false, 0, "", tea.ProgramStateIdle},
		{"cancelled after success", notify.FinishIdleUnsuccessful, false, 0, tea.ProgramStateDone, tea.ProgramStateIdle},
		{"failed", notify.FinishIdleUnsuccessful, false, 0, tea.ProgramStateError, tea.ProgramStateError},
		{"legacy", "", false, 0, "", tea.ProgramStateIdle},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := newBusyUI(&countingWorkspace{ready: true, agentBusy: tc.busy})
			warmCaches(u, true)
			u.turnOutcome = tc.prior
			u.pendingSubmissions = tc.pending
			u.activityFor("s1").pending = tc.pending
			runCmds(u, u.handleAgentNotification(notify.Notification{
				Type: notify.TypeAgentFinished, FinishState: tc.state, SessionID: "s1",
			}))
			require.Equal(t, tc.want, u.programStatus().State)
		})
	}
}

func TestProgramStatusSubmissionReturn(t *testing.T) {
	pinTTLs(t)
	for _, tc := range []struct {
		name string
		err  error
		want tea.ProgramState
	}{
		{"completed before return", nil, tea.ProgramStateDone},
		{"cancelled", context.Canceled, tea.ProgramStateIdle},
		{"failed", context.DeadlineExceeded, tea.ProgramStateError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := newBusyUI(&countingWorkspace{ready: true})
			warmCaches(u, true)
			u.pendingSubmissions = 1
			u.activityFor("s1").pending = 1
			runCmds(u, u.handleAgentNotification(notify.Notification{
				Type: notify.TypeAgentFinished, FinishState: notify.FinishIdleSuccess, SessionID: "s1",
			}))
			require.Equal(t, tea.ProgramStateWorking, u.programStatus().State)
			_, cmd := u.Update(agentRunSubmittedMsg{sessionID: "s1", err: tc.err})
			runCmds(u, cmd)
			require.Equal(t, tc.want, u.programStatus().State)
		})
	}
}

func TestProgramStatusIgnoresOtherSessionOutcomes(t *testing.T) {
	pinTTLs(t)
	u := newBusyUI(&countingWorkspace{ready: true})
	warmCaches(u, false)
	for _, typ := range []notify.Type{notify.TypeAgentFinished, notify.TypeAgentError} {
		runCmds(u, u.handleAgentNotification(notify.Notification{
			Type: typ, FinishState: notify.FinishIdleSuccess, SessionID: "s2",
		}))
		require.Equal(t, tea.ProgramStateIdle, u.programStatus().State)
	}

	u.turnOutcome = tea.ProgramStateDone
	u.sendMessage("new work")
	t.Cleanup(common.StopTurn)
	require.Empty(t, u.turnOutcome)
	_, cmd := u.Update(agentRunSubmittedMsg{sessionID: "s1", gen: u.submissionGen})
	runCmds(u, cmd)
	u.turnOutcome = tea.ProgramStateError
	u.Update(loadSessionMsg{session: &session.Session{ID: "s2"}})
	require.Empty(t, u.turnOutcome)
}
