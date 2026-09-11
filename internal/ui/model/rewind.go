package model

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"fmt"
	"github.com/charmbracelet/crush/internal/filehistory"
	"github.com/charmbracelet/crush/internal/ui/util"
	"github.com/charmbracelet/crush/internal/workspace"
)

type rewindResultMsg struct {
	sessionID, action string
	result            filehistory.Result
	err               error
}

func (m *UI) rewindCommand(action, target, sessionID string) tea.Cmd {
	if !m.hasSession() {
		return util.ReportInfo("No saved prompts yet. Send a prompt first.")
	}
	if m.isAgentBusy() {
		return util.ReportWarn("Wait for the agent to finish before rewinding.")
	}
	if sessionID == "" {
		sessionID = m.session.ID
	}
	if sessionID != m.session.ID {
		return util.ReportError(fmt.Errorf("session changed; open /rewind again"))
	}
	ws, ok := m.com.Workspace.(workspace.RewindWorkspace)
	if !ok {
		return util.ReportError(fmt.Errorf("this workspace does not support rewind"))
	}
	return func() tea.Msg {
		result, err := ws.Rewind(context.Background(), sessionID, filehistory.Request{Action: action, Target: target})
		return rewindResultMsg{sessionID: sessionID, action: action, result: result, err: err}
	}
}
