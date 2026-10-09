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

func newProgramStatusUI(t *testing.T) *UI {
	t.Helper()
	pinTTLs(t)
	u := newBusyUI(&countingWorkspace{ready: true})
	warmCaches(u, false)
	return u
}

func agentFinished() tea.Msg {
	return pubsub.Event[notify.Notification]{
		Type:    pubsub.CreatedEvent,
		Payload: notify.Notification{Type: notify.TypeAgentFinished, SessionID: "s1"},
	}
}

func runComplete(rc notify.RunComplete) tea.Msg {
	rc.SessionID = "s1"
	return pubsub.Event[notify.RunComplete]{Type: pubsub.UpdatedEvent, Payload: rc}
}

func TestProgramStatus(t *testing.T) {
	cases := []struct {
		name  string
		setup func(u *UI)
		want  tea.ProgramStatus
	}{
		{
			name: "idle",
			want: tea.ProgramStatus{State: tea.ProgramStateIdle, App: "crush"},
		},
		{
			name:  "working",
			setup: func(u *UI) { u.agentBusyCache.set(true) },
			want:  tea.ProgramStatus{State: tea.ProgramStateWorking, App: "crush"},
		},
		{
			name: "question",
			setup: func(u *UI) {
				u.agentBusyCache.set(true)
				u.activeInline = dialog.NewQuestionForm(u.com.Styles, question.Request{})
			},
			want: tea.ProgramStatus{
				State: tea.ProgramStateBlocked, App: "crush",
				Kind: tea.ProgramStatusKindQuestion, Message: "Questions need your input",
			},
		},
		{
			name: "permission wins over a question",
			setup: func(u *UI) {
				u.activeInline = dialog.NewQuestionForm(u.com.Styles, question.Request{})
				u.dialog.OpenDialogWithGrace(dialog.NewPermissions(u.com, permission.PermissionRequest{ID: "p", ToolName: "bash"}))
			},
			want: tea.ProgramStatus{
				State: tea.ProgramStateBlocked, App: "crush",
				Kind: tea.ProgramStatusKindPermission, Message: "Permission required",
			},
		},
		{
			name: "aws sign-in",
			setup: func(u *UI) {
				dlg, _ := dialog.NewAWSSSO(u.com, "aws sso login")
				u.dialog.OpenDialog(dlg)
			},
			want: tea.ProgramStatus{
				State: tea.ProgramStateBlocked, App: "crush",
				Kind: tea.ProgramStatusKindAuth, Message: "AWS sign-in required",
			},
		},
		{
			name: "working wins over the last outcome",
			setup: func(u *UI) {
				u.turnOutcome = tea.ProgramStateDone
				u.agentBusyCache.set(true)
			},
			want: tea.ProgramStatus{State: tea.ProgramStateWorking, App: "crush"},
		},
		{
			name:  "outcome once idle",
			setup: func(u *UI) { u.turnOutcome = tea.ProgramStateError },
			want:  tea.ProgramStatus{State: tea.ProgramStateError, App: "crush"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := newProgramStatusUI(t)
			if c.setup != nil {
				c.setup(u)
			}
			require.Equal(t, &c.want, u.programStatus())
		})
	}
}

func TestTurnOutcome(t *testing.T) {
	cases := []struct {
		name string
		msgs []tea.Msg
		want tea.ProgramState
	}{
		{"finished", []tea.Msg{agentFinished()}, tea.ProgramStateDone},
		{"failed", []tea.Msg{runComplete(notify.RunComplete{Error: "boom"})}, tea.ProgramStateError},
		{"succeeded run complete leaves done", []tea.Msg{agentFinished(), runComplete(notify.RunComplete{})}, tea.ProgramStateDone},
		{"cancelled is idle", []tea.Msg{agentFinished(), runComplete(notify.RunComplete{Error: "context canceled", Cancelled: true})}, ""},
		{"queued prompt failing after a finished one", []tea.Msg{agentFinished(), runComplete(notify.RunComplete{Error: "boom"}), runComplete(notify.RunComplete{})}, tea.ProgramStateError},
		// In local mode AgentRun returns only after the turn, so this arrives
		// after the finish notification and must not clear it.
		{"run submitted after finishing", []tea.Msg{agentFinished(), agentRunSubmittedMsg{}}, tea.ProgramStateDone},
		{"focus clears", []tea.Msg{agentFinished(), tea.FocusMsg{}}, ""},
		{"other notifications ignored", []tea.Msg{pubsub.Event[notify.Notification]{Payload: notify.Notification{Type: notify.TypeAgentError}}}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			u := newProgramStatusUI(t)
			for _, msg := range c.msgs {
				_, cmd := u.Update(msg)
				runCmds(u, cmd)
			}
			require.Equal(t, c.want, u.turnOutcome)
		})
	}
}

func TestSendMessageClearsTurnOutcome(t *testing.T) {
	u := newProgramStatusUI(t)
	u.turnOutcome = tea.ProgramStateDone

	require.NotNil(t, u.sendMessage("next"))
	require.Empty(t, u.turnOutcome)
}
