package model

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/event"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// markProjectInitializedCmd marks the current project as initialized in the config.
func (m *UI) markProjectInitializedCmd() tea.Cmd {
	return func() tea.Msg {
		if err := m.com.Workspace.MarkProjectInitialized(); err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  fmt.Sprintf("Failed to mark project as initialized: %v", err),
				TTL:  15 * time.Second,
			}
		}
		return nil
	}
}

// initializeProject starts project initialization and transitions to the landing view.
func (m *UI) initializeProject() tea.Cmd {
	m.firstRun.needsInit = false
	event.ProjectInitStarted()
	// clear the session
	var cmds []tea.Cmd
	if cmd := m.newSession(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	initialize := func() tea.Msg {
		initPrompt, err := m.com.Workspace.InitializePrompt()
		if err != nil {
			return util.InfoMsg{
				Type: util.InfoTypeError,
				Msg:  fmt.Sprintf("Failed to initialize project: %v", err),
			}
		}
		return sendMessageMsg{Content: initPrompt}
	}
	// Mark the project as initialized
	cmds = append(cmds, initialize, m.markProjectInitializedCmd())

	return tea.Sequence(cmds...)
}
