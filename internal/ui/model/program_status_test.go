package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/question"
	"github.com/charmbracelet/crush/internal/session"
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

func runCompleteMsg(rc notify.RunComplete) tea.Msg {
	if rc.SessionID == "" {
		rc.SessionID = "s1"
	}
	return pubsub.Event[notify.RunComplete]{Type: pubsub.UpdatedEvent, Payload: rc}
}

func finished() tea.Msg { return runCompleteMsg(notify.RunComplete{}) }

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
		{"finished", []tea.Msg{finished()}, tea.ProgramStateDone},
		{"failed", []tea.Msg{runCompleteMsg(notify.RunComplete{Error: "boom"})}, tea.ProgramStateError},
		{"cancelled is idle", []tea.Msg{finished(), runCompleteMsg(notify.RunComplete{Error: "context canceled", Cancelled: true})}, ""},
		{"queued prompt failing after a finished one", []tea.Msg{finished(), runCompleteMsg(notify.RunComplete{Error: "boom"})}, tea.ProgramStateError},
		// In local mode AgentRun returns only after the turn, so this arrives
		// after the run completes and must not clear it.
		{"run submitted after finishing", []tea.Msg{finished(), agentRunSubmittedMsg{}}, tea.ProgramStateDone},
		{"focus clears", []tea.Msg{finished(), tea.FocusMsg{}}, ""},
		{"other session ignored", []tea.Msg{finished(), runCompleteMsg(notify.RunComplete{SessionID: "other", Error: "boom"})}, tea.ProgramStateDone},
		{"other session cancel ignored", []tea.Msg{finished(), runCompleteMsg(notify.RunComplete{SessionID: "other", Cancelled: true})}, tea.ProgramStateDone},
		{"other session finish ignored", []tea.Msg{runCompleteMsg(notify.RunComplete{SessionID: "child"})}, ""},
		{"switching session clears", []tea.Msg{finished(), loadSessionMsg{session: &session.Session{ID: "s2"}}}, ""},
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

func TestNewSessionClearsTurnOutcome(t *testing.T) {
	u := newProgramStatusUI(t)
	u.turnOutcome = tea.ProgramStateError

	u.newSession()
	require.Empty(t, u.turnOutcome)
}
