package model

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/crush/internal/agent/notify"
	"github.com/charmbracelet/crush/internal/pubsub"
	"github.com/charmbracelet/crush/internal/ui/dialog"
)

// programStatusApp is the stable program name reported to the terminal.
const programStatusApp = "crush"

// programStatus reports what Crush is doing to terminals that support the
// Program Status Protocol (OSC 7501), so they can show whether Crush is
// working, waiting on the user, or finished. Terminals without support
// ignore it.
func (m *UI) programStatus() *tea.ProgramStatus {
	ps := &tea.ProgramStatus{State: tea.ProgramStateIdle, App: programStatusApp}
	switch {
	case m.dialog != nil && m.dialog.ContainsDialog(dialog.PermissionsID):
		ps.State = tea.ProgramStateBlocked
		ps.Kind = tea.ProgramStatusKindPermission
		ps.Message = "Permission required"
	case m.hasPendingQuestion():
		ps.State = tea.ProgramStateBlocked
		ps.Kind = tea.ProgramStatusKindQuestion
		ps.Message = "Questions need your input"
	case m.dialog != nil && m.dialog.ContainsDialog(dialog.AWSSSOID):
		ps.State = tea.ProgramStateBlocked
		ps.Kind = tea.ProgramStatusKindAuth
		ps.Message = "AWS sign-in required"
	case m.isAgentBusy():
		ps.State = tea.ProgramStateWorking
	case m.turnOutcome != "":
		ps.State = m.turnOutcome
	}
	return ps
}

func (m *UI) hasPendingQuestion() bool {
	_, ok := m.activeInline.(*dialog.QuestionForm)
	return ok
}

// trackTurnOutcome records how the last agent turn ended until the user
// returns to the window or submits something new. Errors come from
// RunComplete because local mode never publishes TypeAgentError.
func (m *UI) trackTurnOutcome(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.FocusMsg:
		m.turnOutcome = ""
	case pubsub.Event[notify.Notification]:
		if msg.Payload.Type == notify.TypeAgentFinished {
			m.turnOutcome = tea.ProgramStateDone
		}
	case pubsub.Event[notify.RunComplete]:
		switch {
		case msg.Payload.Cancelled:
			m.turnOutcome = ""
		case msg.Payload.Error != "":
			m.turnOutcome = tea.ProgramStateError
		}
	}
}
