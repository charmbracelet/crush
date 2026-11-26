package model

import (
	"slices"

	tea "charm.land/bubbletea/v2"
)

// SetResumeCommand sets the command that brings Crush back after the
// terminal restarts. The open session is added to it.
func (m *UI) SetResumeCommand(args []string) {
	m.resumeCommand = args
}

// programResume tells terminals that support the Program Resume Protocol
// (OSC 7577) how to bring Crush back, with the open session, after they
// restart. It is nil once the user quits, so the terminal forgets it.
// Terminals without support ignore it.
func (m *UI) programResume() *tea.ProgramResume {
	if len(m.resumeCommand) == 0 || m.quitting {
		return nil
	}
	args := m.resumeCommand
	if id := m.currentSessionID(); id != "" {
		args = append(slices.Clip(args), "--session", id)
	}
	return &tea.ProgramResume{Args: args, Dir: m.com.Workspace.WorkingDir()}
}
